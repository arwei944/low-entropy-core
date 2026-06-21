package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MigrateEvent 迁移引擎 SSE 事件
type MigrateEvent struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	SessionID string      `json:"session_id,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// migrateEventBus 迁移引擎事件广播总线
type migrateEventBus struct {
	mu          sync.RWMutex
	subscribers map[chan MigrateEvent]bool
}

var migEventBus = &migrateEventBus{
	subscribers: make(map[chan MigrateEvent]bool),
}

func (b *migrateEventBus) subscribe() chan MigrateEvent {
	ch := make(chan MigrateEvent, 32)
	b.mu.Lock()
	b.subscribers[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *migrateEventBus) unsubscribe(ch chan MigrateEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
}

func (b *migrateEventBus) publish(evt MigrateEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
			// 慢消费者，跳过
		}
	}
}

// GET /api/sse/migrate — 迁移引擎实时事件流
func handleMigrateSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := migEventBus.subscribe()
	defer migEventBus.unsubscribe(ch)

	// 发送初始连接事件
	data, _ := json.Marshal(MigrateEvent{
		Type:      "connected",
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   "迁移引擎事件流已连接",
	})
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
			// 确保所有迁移相关事件类型都能被正确转发
			switch evt.Type {
			case "connected", "analyze_start", "analyze_done",
				"validate_start", "validate_done",
				"migration_start", "migration_file_start", "migration_file_done",
				"migration_complete", "migration_error",
				"rollback_start", "rollback_complete",
				"ping":
				// 所有已知事件类型均允许通过
			default:
				// 未知类型也允许通过，保持前向兼容
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
