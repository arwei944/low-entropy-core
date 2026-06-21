package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GET /api/arch-changelog — 查询架构变动日志
func handleArchChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	filter := ArchChangeFilter{
		Category: q.Get("category"),
		Severity: q.Get("severity"),
		File:     q.Get("file"),
		Source:   q.Get("source"),
		Limit:    100,
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	if sinceStr := q.Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filter.Since = t
		}
	}

	entries := changelogStore.Query(filter)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// POST /api/arch-changelog — 创建架构变动日志条目
func handleArchChangelogCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Category string `json:"category"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
		File     string `json:"file"`
		Source   string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, "missing message", http.StatusBadRequest)
		return
	}
	if req.Category == "" {
		req.Category = "manual"
	}
	if req.Severity == "" {
		req.Severity = "info"
	}
	if req.Source == "" {
		req.Source = "manual"
	}

	entry := ArchChangeEntry{
		Category: req.Category,
		Severity: req.Severity,
		Detail:   req.Message,
		File:     req.File,
		Source:   req.Source,
	}
	if err := changelogStore.Append(entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// GET /api/arch-changelog/export — 导出架构变动日志
func handleArchChangelogExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	entries := changelogStore.Query(ArchChangeFilter{})

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		var sb strings.Builder
		sb.WriteString("| ID | SeqNo | Timestamp | Category | Severity | File | Detail | Source |\n")
		sb.WriteString("|----|-------|-----------|----------|----------|------|--------|--------|\n")
		for _, e := range entries {
			fmt.Fprintf(&sb, "| %s | %d | %s | %s | %s | %s | %s | %s |\n",
				e.ID, e.SeqNo, e.Timestamp.Format(time.RFC3339), e.Category, e.Severity, e.File, e.Detail, e.Source)
		}
		w.Write([]byte(sb.String()))
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=arch-changelog.csv")
		cw := csv.NewWriter(w)
		cw.Write([]string{"id", "seq_no", "timestamp", "category", "severity", "file", "detail", "source"})
		for _, e := range entries {
			cw.Write([]string{
				e.ID,
				strconv.FormatInt(e.SeqNo, 10),
				e.Timestamp.Format(time.RFC3339),
				e.Category,
				e.Severity,
				e.File,
				e.Detail,
				e.Source,
			})
		}
		cw.Flush()
	default:
		http.Error(w, "unsupported format: "+format, http.StatusBadRequest)
	}
}

// GET /api/arch-changelog/stats — 变动统计摘要
func handleArchChangelogStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(changelogStore.Stats())
}

// handleArchChangelogOrPost 根据方法分发 GET/POST /api/arch-changelog
func handleArchChangelogOrPost(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleArchChangelogCreate(w, r)
	} else {
		handleArchChangelog(w, r)
	}
}

// GET /api/sse/arch-changelog — 架构变动实时事件流
func handleArchChangelogSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := chgEventBus.subscribe()
	defer chgEventBus.unsubscribe(ch)

	// 发送初始连接事件
	data, _ := json.Marshal(map[string]interface{}{
		"type":      "connected",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "架构变动事件流已连接",
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
