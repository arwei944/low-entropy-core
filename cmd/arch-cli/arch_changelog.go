package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"
)

// ArchChangeEntry 架构变动日志条目（不可变）
type ArchChangeEntry struct {
    ID        string    `json:"id"`
    SeqNo     int64     `json:"seq_no"`
    Timestamp time.Time `json:"timestamp"`
    Category  string    `json:"category"` // file_add|file_modify|file_delete|symbol_add|symbol_remove|violation_add|violation_resolve|health_change|layer_change
    Severity  string    `json:"severity"` // info|warning|critical
    File      string    `json:"file,omitempty"`
    Detail    string    `json:"detail"`
    Before    string    `json:"before,omitempty"` // JSON 快照（变更前）
    After     string    `json:"after,omitempty"`  // JSON 快照（变更后）
    Source    string    `json:"source"`           // watch|manual_refresh|guardian|migration
}

// ArchChangeFilter 查询过滤条件
type ArchChangeFilter struct {
    Category string    `json:"category"`
    Severity string    `json:"severity"`
    File     string    `json:"file"`
    Source   string    `json:"source"`
    Since    time.Time `json:"since"`
    Limit    int       `json:"limit"`
    Offset   int       `json:"offset"`
}

// ArchChangelogStore 架构变动日志存储
type ArchChangelogStore struct {
    mu      sync.RWMutex
    baseDir string
    seqNo   int64
    entries []ArchChangeEntry
    maxMem  int
}

// changelogEventBus 架构变动事件广播
type changelogEventBus struct {
    mu          sync.RWMutex
    subscribers map[chan ArchChangeEntry]bool
}

var changelogStore *ArchChangelogStore
var chgEventBus = &changelogEventBus{
    subscribers: make(map[chan ArchChangeEntry]bool),
}

func (b *changelogEventBus) subscribe() chan ArchChangeEntry {
    ch := make(chan ArchChangeEntry, 32)
    b.mu.Lock()
    b.subscribers[ch] = true
    b.mu.Unlock()
    return ch
}

func (b *changelogEventBus) unsubscribe(ch chan ArchChangeEntry) {
    b.mu.Lock()
    delete(b.subscribers, ch)
    b.mu.Unlock()
}

func (b *changelogEventBus) publish(evt ArchChangeEntry) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for ch := range b.subscribers {
        select {
        case ch <- evt:
        default:
        }
    }
}

// NewArchChangelogStore 创建架构变动日志存储
func NewArchChangelogStore(baseDir string) *ArchChangelogStore {
    _ = os.MkdirAll(baseDir, 0o755)
    store := &ArchChangelogStore{
        baseDir: baseDir,
        maxMem:  1000,
        entries: make([]ArchChangeEntry, 0),
    }
    store.loadRecent()
    return store
}

// Append 追加一条变动记录（不可变）
func (s *ArchChangelogStore) Append(entry ArchChangeEntry) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.seqNo++
    entry.SeqNo = s.seqNo
    entry.Timestamp = time.Now()
    if entry.ID == "" {
        entry.ID = fmt.Sprintf("chg-%d-%d", time.Now().UnixNano(), s.seqNo)
    }

    s.entries = append(s.entries, entry)
    if len(s.entries) > s.maxMem {
        s.entries = s.entries[len(s.entries)-s.maxMem:]
    }

    // 持久化
    if err := s.persist(entry); err != nil {
        return err
    }

    // 推送 SSE 事件
    chgEventBus.publish(entry)

    return nil
}

// Query 查询变动日志
func (s *ArchChangelogStore) Query(filter ArchChangeFilter) []ArchChangeEntry {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var result []ArchChangeEntry
    for i := len(s.entries) - 1; i >= 0; i-- {
        e := s.entries[i]
        if filter.Category != "" && e.Category != filter.Category {
            continue
        }
        if filter.Severity != "" && e.Severity != filter.Severity {
            continue
        }
        if filter.File != "" && e.File != filter.File {
            continue
        }
        if filter.Source != "" && e.Source != filter.Source {
            continue
        }
        if !filter.Since.IsZero() && e.Timestamp.Before(filter.Since) {
            continue
        }
        result = append(result, e)
        if filter.Limit > 0 && len(result) >= filter.Limit {
            break
        }
    }
    return result
}

// Stats 返回统计摘要
func (s *ArchChangelogStore) Stats() map[string]interface{} {
    s.mu.RLock()
    defer s.mu.RUnlock()

    byCategory := make(map[string]int)
    bySeverity := make(map[string]int)
    bySource := make(map[string]int)

    for _, e := range s.entries {
        byCategory[e.Category]++
        bySeverity[e.Severity]++
        bySource[e.Source]++
    }

    result := map[string]interface{}{
        "total":       len(s.entries),
        "by_category": byCategory,
        "by_severity": bySeverity,
        "by_source":   bySource,
    }

    if len(s.entries) > 0 {
        result["last_entry"] = s.entries[len(s.entries)-1]
    }

    return result
}

// persist 持久化单条到 JSON Lines 文件（按日期分片）
func (s *ArchChangelogStore) persist(entry ArchChangeEntry) error {
    dateStr := entry.Timestamp.Format("2006-01-02")
    filename := filepath.Join(s.baseDir, dateStr+".jsonl")

    f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return err
    }
    defer f.Close()

    return json.NewEncoder(f).Encode(entry)
}

// loadRecent 加载最近的日志到内存
func (s *ArchChangelogStore) loadRecent() {
    entries, _ := os.ReadDir(s.baseDir)
    var files []os.DirEntry
    for _, e := range entries {
        if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
            files = append(files, e)
        }
    }
    sort.Slice(files, func(i, j int) bool {
        return files[i].Name() > files[j].Name()
    })

    var loaded []ArchChangeEntry
    for _, f := range files {
        if len(loaded) >= s.maxMem {
            break
        }
        fullPath := filepath.Join(s.baseDir, f.Name())
        data, err := os.ReadFile(fullPath)
        if err != nil {
            continue
        }
        lines := strings.Split(strings.TrimSpace(string(data)), "\n")
        for _, line := range lines {
            if line == "" {
                continue
            }
            var entry ArchChangeEntry
            if err := json.Unmarshal([]byte(line), &entry); err != nil {
                continue
            }
            loaded = append(loaded, entry)
            if entry.SeqNo > s.seqNo {
                s.seqNo = entry.SeqNo
            }
        }
    }

    // 按时间正序，取最后 maxMem 条
    if len(loaded) > s.maxMem {
        loaded = loaded[len(loaded)-s.maxMem:]
    }
    s.entries = loaded
}
