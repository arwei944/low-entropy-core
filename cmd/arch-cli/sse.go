package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DevEvent 开发事件
type DevEvent struct {
	Type      string `json:"type"`      // file_changed, build_start, build_done, test_run, violation_found
	Timestamp string `json:"timestamp"`
	File      string `json:"file,omitempty"`
	Action    string `json:"action,omitempty"` // created, modified, deleted
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// devEventBus 开发事件广播总线
type devEventBus struct {
	mu          sync.RWMutex
	subscribers map[chan DevEvent]bool
}

var eventBus = &devEventBus{
	subscribers: make(map[chan DevEvent]bool),
}

func (b *devEventBus) subscribe() chan DevEvent {
	ch := make(chan DevEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *devEventBus) unsubscribe(ch chan DevEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *devEventBus) publish(evt DevEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
			// 丢弃慢消费者
		}
	}
}

// handleSSE 通过 SSE 推送架构数据变更。
func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			pingData, _ := json.Marshal(map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			fmt.Fprintf(w, "data: %s\n\n", pingData)
			flusher.Flush()
		}
	}
}

// handleDevSSE 推送开发事件（文件变更、构建结果等）
func handleDevSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := eventBus.subscribe()
	defer eventBus.unsubscribe(ch)

	// 发送初始连接事件
	data, _ := json.Marshal(DevEvent{Type: "connected", Timestamp: time.Now().Format(time.RFC3339), Message: "开发事件流已连接"})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			pingData, _ := json.Marshal(map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			fmt.Fprintf(w, "data: %s\n\n", pingData)
			flusher.Flush()
		}
	}
}

// watchFiles 定期轮询文件变更，检测到变更后自动刷新架构数据并推送事件
func watchFiles(dir string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fileHashes := make(map[string]string) // path → modTime+size hash

	// 首次扫描，建立基线
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fileHashes[path] = fmt.Sprintf("%d-%d", info.ModTime().UnixNano(), info.Size())
		return nil
	})

	for range ticker.C {
		changedFiles := []string{}
		newFiles := []string{}
		deletedFiles := []string{}

		// 检测变更
		currentFiles := make(map[string]bool)
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
				return nil
			}
			currentFiles[path] = true
			info, err := d.Info()
			if err != nil {
				return nil
			}
			newHash := fmt.Sprintf("%d-%d", info.ModTime().UnixNano(), info.Size())
			oldHash, existed := fileHashes[path]
			if !existed {
				newFiles = append(newFiles, filepath.Base(path))
			} else if oldHash != newHash {
				changedFiles = append(changedFiles, filepath.Base(path))
			}
			fileHashes[path] = newHash
			return nil
		})

		// 检测删除
		for path := range fileHashes {
			if !currentFiles[path] {
				deletedFiles = append(deletedFiles, filepath.Base(path))
				delete(fileHashes, path)
			}
		}

		totalChanges := len(changedFiles) + len(newFiles) + len(deletedFiles)
		if totalChanges == 0 {
			continue
		}

		log.Printf("[watch] 检测到 %d 个文件变更", totalChanges)

		// 推送文件变更事件
		for _, f := range newFiles {
			eventBus.publish(DevEvent{
				Type: "file_changed", Timestamp: time.Now().Format(time.RFC3339),
				File: f, Action: "created",
				Message: fmt.Sprintf("新文件创建: %s", f),
			})
			changelogStore.Append(ArchChangeEntry{
				Category: "file_add", Severity: "info", File: f,
				Detail: fmt.Sprintf("新文件创建: %s", f), Source: "watch",
			})
		}
		for _, f := range changedFiles {
			eventBus.publish(DevEvent{
				Type: "file_changed", Timestamp: time.Now().Format(time.RFC3339),
				File: f, Action: "modified",
				Message: fmt.Sprintf("文件修改: %s", f),
			})
			changelogStore.Append(ArchChangeEntry{
				Category: "file_modify", Severity: "info", File: f,
				Detail: fmt.Sprintf("文件修改: %s", f), Source: "watch",
			})
		}
		for _, f := range deletedFiles {
			eventBus.publish(DevEvent{
				Type: "file_changed", Timestamp: time.Now().Format(time.RFC3339),
				File: f, Action: "deleted",
				Message: fmt.Sprintf("文件删除: %s", f),
			})
			changelogStore.Append(ArchChangeEntry{
				Category: "file_delete", Severity: "warning", File: f,
				Detail: fmt.Sprintf("文件删除: %s", f), Source: "watch",
			})
		}

		// 重新构建架构数据
		eventBus.publish(DevEvent{Type: "build_start", Timestamp: time.Now().Format(time.RFC3339), Message: "正在刷新架构数据..."})
		newData, err := buildArchData(dir)
		if err != nil {
			log.Printf("[watch] 刷新失败: %v", err)
			eventBus.publish(DevEvent{Type: "build_done", Timestamp: time.Now().Format(time.RFC3339), Message: fmt.Sprintf("刷新失败: %v", err)})
			continue
		}

		archMu.Lock()
		archData = newData
		archMu.Unlock()

		// 推送构建完成事件
		eventBus.publish(DevEvent{
			Type: "build_done", Timestamp: time.Now().Format(time.RFC3339),
			Message: fmt.Sprintf("架构数据刷新完成: %d 文件, %d 符号", len(newData.Files), totalSymbols(newData)),
			Data:    map[string]int{"files": len(newData.Files), "symbols": totalSymbols(newData), "changes": totalChanges},
		})

		// 检查违规
		resp := detectViolations(newData)
		if resp.Total > 0 {
			eventBus.publish(DevEvent{
				Type: "violation_found", Timestamp: time.Now().Format(time.RFC3339),
				Message: fmt.Sprintf("检测到 %d 条架构违规", resp.Total),
				Data:    resp,
			})
			for _, v := range resp.Items {
				changelogStore.Append(ArchChangeEntry{
					Category: "violation_add", Severity: string(v.Severity),
					File: v.File, Detail: v.Message, Source: "watch",
				})
			}
		}
		log.Printf("[watch] 刷新完成")
	}
}

func totalSymbols(data *ArchData) int {
	count := 0
	for _, f := range data.Files {
		count += len(f.Symbols)
	}
	return count
}