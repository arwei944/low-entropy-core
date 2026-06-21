package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"low-entropy-core/go-core/migrate"
)

// POST /api/migrate/execute
func handleMigrateExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Dir        string                 `json:"dir"`
		TargetTier string                 `json:"target_tier"`
		Options    map[string]interface{} `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Dir == "" {
		http.Error(w, "missing dir parameter", http.StatusBadRequest)
		return
	}
	if req.TargetTier == "" {
		req.TargetTier = "L1"
	}

	sessionID := fmt.Sprintf("mig-%d", time.Now().UnixNano())
	session := migrateSessionInfo{
		SessionID:  sessionID,
		StartedAt:  time.Now(),
		Status:     "running",
		TargetTier: req.TargetTier,
	}

	migState.mu.Lock()
	migState.sessions = append(migState.sessions, session)
	migState.activeSession = &migState.sessions[len(migState.sessions)-1]
	migState.mu.Unlock()

	// 异步执行迁移
	go runMigrationAsync(sessionID, req.Dir, req.TargetTier)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id":  sessionID,
		"status":      "running",
		"target_tier": req.TargetTier,
	})
}

func runMigrationAsync(sessionID, dir, targetTier string) {
	publish := func(evtType string, data map[string]interface{}) {
		migEventBus.publish(MigrateEvent{
			Type:      evtType,
			Timestamp: time.Now().Format(time.RFC3339),
			SessionID: sessionID,
			Data:      data,
		})
	}

	publish("migration_start", map[string]interface{}{"dir": dir, "target_tier": targetTier})

	// 语言检测
	detector := migrate.NewLanguageDetector()
	lang := detector.Detect(dir)
	if lang == "unknown" {
		publish("migration_error", map[string]interface{}{"error": "unsupported language"})
		updateSessionStatus(sessionID, "error")
		return
	}

	backend, err := migrate.GetParser(lang)
	if err != nil {
		publish("migration_error", map[string]interface{}{"error": err.Error()})
		updateSessionStatus(sessionID, "error")
		return
	}

	files, err := backend.ParseDir(dir)
	if err != nil {
		publish("migration_error", map[string]interface{}{"error": err.Error()})
		updateSessionStatus(sessionID, "error")
		return
	}

	var allFuncs []migrate.UnifiedFunction
	for _, f := range files {
		allFuncs = append(allFuncs, f.Functions...)
	}

	classifiers := []migrate.PatternClassifier{
		&migrate.AtomClassifier{},
		&migrate.PortClassifier{},
		&migrate.AdapterClassifier{},
		&migrate.ComposerClassifier{},
	}
	patternMap := migrate.ClassifyFunctions(allFuncs, classifiers)

	// 模拟逐个文件迁移
	successCount := 0
	errorCount := 0
	for i, f := range files {
		fileName := filepath.Base(f.Path)
		publish("migration_file_start", map[string]interface{}{"file": fileName, "index": i})

		// 模拟处理延迟
		time.Sleep(50 * time.Millisecond)

		// 模拟部分文件出错
		status := "ok"
		if strings.Contains(fileName, "_test.") {
			status = "ok"
		} else if i%7 == 3 {
			status = "error"
			errorCount++
		} else {
			successCount++
		}

		publish("migration_file_done", map[string]interface{}{"file": fileName, "status": status})
	}

	// 更新会话统计
	statsStr := make(map[string]int)
	for k, v := range patternMap.Stats() {
		statsStr[string(k)] = v
	}
	migState.mu.Lock()
	for i := range migState.sessions {
		if migState.sessions[i].SessionID == sessionID {
			migState.sessions[i].FileCount = len(files)
			migState.sessions[i].EntryCount = len(allFuncs)
			migState.sessions[i].Language = lang
			migState.sessions[i].PatternStats = statsStr
			if errorCount > 0 {
				migState.sessions[i].Status = "completed_with_errors"
			} else {
				migState.sessions[i].Status = "completed"
			}
			break
		}
	}
	migState.mu.Unlock()

	publish("migration_complete", map[string]interface{}{
		"total_files":   len(files),
		"success_count": successCount,
		"error_count":   errorCount,
	})
}

func updateSessionStatus(sessionID, status string) {
	migState.mu.Lock()
	defer migState.mu.Unlock()
	for i := range migState.sessions {
		if migState.sessions[i].SessionID == sessionID {
			migState.sessions[i].Status = status
			break
		}
	}
}
