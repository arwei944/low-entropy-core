package main

import (
	"encoding/json"
	"fmt"
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

// POST /api/migrate/analyze
func handleMigrateAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Dir      string `json:"dir"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Dir == "" {
		http.Error(w, "missing dir parameter", http.StatusBadRequest)
		return
	}

	// 推送开始事件
	migEventBus.publish(MigrateEvent{
		Type: "analyze_start", Timestamp: time.Now().Format(time.RFC3339),
		Message: "开始分析: " + req.Dir,
	})

	// 语言检测
	if req.Language == "" || req.Language == "auto" {
		detector := migrate.NewLanguageDetector()
		req.Language = detector.Detect(req.Dir)
	}

	backend, err := migrate.GetParser(req.Language)
	if err != nil {
		http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
		return
	}

	files, err := backend.ParseDir(req.Dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	// 将 CodePattern map 转为 string map 用于 JSON
	statsStr := make(map[string]int)
	for k, v := range patternMap.Stats() {
		statsStr[string(k)] = v
	}

	sessionID := fmt.Sprintf("mig-%d", time.Now().UnixNano())
	session := migrateSessionInfo{
		SessionID:    sessionID,
		StartedAt:    time.Now(),
		Status:       "completed",
		FileCount:    len(files),
		Language:     req.Language,
		PatternStats: statsStr,
	}

	migState.mu.Lock()
	migState.sessions = append(migState.sessions, session)
	migState.mu.Unlock()

	// 推送完成事件
	migEventBus.publish(MigrateEvent{
		Type: "analyze_done", Timestamp: time.Now().Format(time.RFC3339),
		SessionID: sessionID, Message: fmt.Sprintf("分析完成: %d 文件, %d 函数", len(files), len(allFuncs)),
		Data: map[string]interface{}{"pattern_map": patternMap, "stats": statsStr},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id":  sessionID,
		"language":    req.Language,
		"file_count": len(files),
		"func_count": len(allFuncs),
		"pattern_map": patternMap,
		"stats":       statsStr,
	})
}

// POST /api/migrate/validate
func handleMigrateValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Dir      string `json:"dir"`
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Dir == "" {
		http.Error(w, "missing dir parameter", http.StatusBadRequest)
		return
	}

	migEventBus.publish(MigrateEvent{
		Type: "validate_start", Timestamp: time.Now().Format(time.RFC3339),
		Message: "开始验证: " + req.Dir,
	})

	if req.Language == "" || req.Language == "auto" {
		detector := migrate.NewLanguageDetector()
		req.Language = detector.Detect(req.Dir)
	}

	backend, err := migrate.GetParser(req.Language)
	if err != nil {
		http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
		return
	}

	files, err := backend.ParseDir(req.Dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	// 运行约束门链
	ctx := &migrate.MigrationContext{
		Files:      files,
		PatternMap: patternMap,
		Log:        migrate.NewMigrationLog(fmt.Sprintf("val-%d", time.Now().UnixNano())),
		Phase:      "validate",
		ProjectDir: req.Dir,
		Language:   req.Language,
	}

	chain := migrate.DefaultGateChain()
	decision := chain.Evaluate(ctx)

	// 增强验证
	validator := migrate.EnhancedValidator{Log: ctx.Log}
	report := validator.Validate(ctx)

	result := map[string]interface{}{
		"pass":     decision.Pass,
		"gate_id":  decision.GateID,
		"blocked":  decision.BlockedRules,
		"warnings": decision.Warnings,
		"report":   report,
	}

	migEventBus.publish(MigrateEvent{
		Type: "validate_done", Timestamp: time.Now().Format(time.RFC3339),
		Message: fmt.Sprintf("验证完成: %v", decision.Pass),
		Data:    result,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
		// 返回所有（通过 QueryByPhase 空字符串或直接返回空）
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
