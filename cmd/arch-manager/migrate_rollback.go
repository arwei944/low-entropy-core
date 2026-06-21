package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// POST /api/migrate/rollback
func handleMigrateRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.SessionID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}

	migState.mu.Lock()
	var found bool
	for i := range migState.sessions {
		if migState.sessions[i].SessionID == req.SessionID {
			migState.sessions[i].Status = "rolling_back"
			found = true
			break
		}
	}
	migState.mu.Unlock()

	if !found {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// 推送回滚开始事件
	migEventBus.publish(MigrateEvent{
		Type:      "rollback_start",
		Timestamp: time.Now().Format(time.RFC3339),
		SessionID: req.SessionID,
		Message:   fmt.Sprintf("开始回滚会话: %s", req.SessionID),
	})

	// 模拟回滚操作（异步）
	go func(sessionID string) {
		time.Sleep(500 * time.Millisecond)
		migState.mu.Lock()
		for i := range migState.sessions {
			if migState.sessions[i].SessionID == sessionID {
				migState.sessions[i].Status = "rolled_back"
				break
			}
		}
		migState.mu.Unlock()

		migEventBus.publish(MigrateEvent{
			Type:      "rollback_complete",
			Timestamp: time.Now().Format(time.RFC3339),
			SessionID: sessionID,
			Message:   "回滚完成",
		})
	}(req.SessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": req.SessionID,
		"status":     "rolling_back",
	})
}
