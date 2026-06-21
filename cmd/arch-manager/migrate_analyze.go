package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"low-entropy-core/go-core/migrate"
)

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
