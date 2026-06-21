package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"low-entropy-core/go-core/migrate"
)

// migrateState 迁移引擎全局状态
type migrateState struct {
	mu            sync.RWMutex
	sessions      []migrateSessionInfo
	activeSession *migrateSessionInfo
	logStore      *migrate.FileLogStore
}

// migrateSessionInfo 迁移会话摘要
type migrateSessionInfo struct {
	SessionID    string         `json:"session_id"`
	StartedAt    time.Time      `json:"started_at"`
	Status       string         `json:"status"`
	FileCount    int            `json:"file_count"`
	EntryCount   int            `json:"entry_count"`
	Language     string         `json:"language"`
	TargetTier   string         `json:"target_tier"`
	PatternStats map[string]int `json:"pattern_stats"`
}

var migState *migrateState

func initMigrateState(baseDir string) {
	migState = &migrateState{
		logStore: migrate.NewFileLogStore(baseDir),
	}
}

// GET /api/migrate/sessions
func handleMigrateSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	migState.mu.RLock()
	sessions := make([]migrateSessionInfo, len(migState.sessions))
	copy(sessions, migState.sessions)
	migState.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// DELETE /api/migrate/sessions/{id}
func handleMigrateSessionDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/migrate/sessions/")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	migState.mu.Lock()
	defer migState.mu.Unlock()
	for i, s := range migState.sessions {
		if s.SessionID == id {
			migState.sessions = append(migState.sessions[:i], migState.sessions[i+1:]...)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"deleted": id})
			return
		}
	}
	http.Error(w, "session not found", http.StatusNotFound)
}

// GET /api/migrate/sessions/{id}
func handleMigrateSessionDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/migrate/sessions/")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	migState.mu.RLock()
	defer migState.mu.RUnlock()
	for _, s := range migState.sessions {
		if s.SessionID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s)
			return
		}
	}
	http.Error(w, "session not found", http.StatusNotFound)
}

// handleMigrateSessionDetailOrDelete 根据方法分发 GET/DELETE /api/migrate/sessions/{id}
func handleMigrateSessionDetailOrDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		handleMigrateSessionDelete(w, r)
	} else {
		handleMigrateSessionDetail(w, r)
	}
}

// GET /api/migrate/logs
func handleMigrateLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	phase := q.Get("phase")
	action := q.Get("action")
	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	var entries []migrate.MigrationLogEntry
	if phase != "" {
		entries = migState.logStore.QueryByPhase(phase)
	} else if action != "" {
		entries = migState.logStore.QueryByActionType(action)
	} else {
		entries = migState.logStore.QueryByPhase("")
	}

	if len(entries) > limit {
		entries = entries[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// GET /api/migrate/logs/export
func handleMigrateLogsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	content, err := migState.logStore.Export(format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "md" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.Write([]byte(content))
}

// DELETE /api/migrate/sessions/{id} — 取消迁移会话
func handleMigrateSessionCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/migrate/sessions/")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	migState.mu.Lock()
	var found bool
	for i := range migState.sessions {
		if migState.sessions[i].SessionID == id {
			migState.sessions[i].Status = "cancelled"
			found = true
			break
		}
	}
	migState.mu.Unlock()

	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	migEventBus.publish(MigrateEvent{
		Type:      "migration_cancelled",
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: id,
		Message:   "会话已取消",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": id,
		"status":     "cancelled",
	})
}

// GET /api/migrate/status
func handleMigrateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	migState.mu.RLock()
	sessionCount := len(migState.sessions)
	var activeCount int
	for _, s := range migState.sessions {
		if s.Status == "running" {
			activeCount++
		}
	}
	migState.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active_sessions": activeCount,
		"total_sessions":  sessionCount,
		"status":          "ready",
		"version":         "v0.3.0",
	})
}
