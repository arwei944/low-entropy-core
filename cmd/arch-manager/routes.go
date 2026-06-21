package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// handleAPI 返回架构数据 JSON（含复杂度评分）
func handleAPI(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	maxLines := 1
	maxSymbols := 1
	maxDeps := 1
	for _, f := range archData.Files {
		if f.Lines > maxLines {
			maxLines = f.Lines
		}
		if len(f.Symbols) > maxSymbols {
			maxSymbols = len(f.Symbols)
		}
		if len(f.DependsOn) > maxDeps {
			maxDeps = len(f.DependsOn)
		}
	}

	enhanced := EnhancedArchData{
		ArchData:         archData,
		ComplexityScores: make(map[string]float64),
		MaxLines:         maxLines,
		MaxSymbols:       maxSymbols,
		MaxDeps:          maxDeps,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(enhanced)
}

// handleFile 返回单个文件的详细内容
func handleFile(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	for _, f := range archData.Files {
		if f.Name == filename {
			// 读取文件内容
			content, err := os.ReadFile(f.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"info":    f,
				"content": string(content),
			})
			return
		}
	}

	http.Error(w, "file not found", http.StatusNotFound)
}

// handleRefresh 强制刷新架构数据
func handleRefresh(w http.ResponseWriter, r *http.Request) {
	archMu.Lock()
	oldData := archData
	archMu.Unlock()

	newData, err := buildArchData(sourceDir)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	archMu.Lock()
	archData = newData
	archMu.Unlock()

	changes := diffArchData(oldData, newData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "refreshed",
		"changes": changes,
		"data":    newData,
	})
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	archMu.RLock()
	ready := archData != nil
	archMu.RUnlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"ready":  ready,
		"time":   time.Now().Format(time.RFC3339),
	})
}

// registerRoutes 注册所有 HTTP 路由
func registerRoutes(mux *http.ServeMux) {
	// API 路由
	mux.HandleFunc("/api/arch", handleAPI)
	mux.HandleFunc("/api/file", handleFile)
	mux.HandleFunc("/api/refresh", handleRefresh)
	mux.HandleFunc("/api/health", handleHealth)
	mux.HandleFunc("/api/health-score", handleHealthScore)
	mux.HandleFunc("/api/violations", handleViolations)
	mux.HandleFunc("/api/export", handleExport)
	mux.HandleFunc("/api/sse", handleSSE)
	mux.HandleFunc("/api/sse/dev-events", handleDevSSE)

	// 版本管理 API 路由 (v0.6.0)
	mux.HandleFunc("/api/version/snapshot", handleVersionSnapshot)
	mux.HandleFunc("/api/version/diff", handleVersionDiff)
	mux.HandleFunc("/api/version/changelog", handleVersionChangelog)
	mux.HandleFunc("/api/version/commit-analyze", handleVersionCommitAnalyze)
	mux.HandleFunc("/api/version/next-version", handleVersionNextVersion)
	mux.HandleFunc("/api/version/arch-change", handleVersionArchChange)
	mux.HandleFunc("/api/version/adr", handleVersionADR)
	mux.HandleFunc("/api/version/release", handleVersionRelease)
	mux.HandleFunc("/api/version", handleVersion)

	// 引导层 API 路由 (v0.6.0)
	mux.HandleFunc("/api/guide", handleGuide)

	// UA 知识图谱 API 路由 (v0.7.0)
	mux.HandleFunc("/api/ua/graph", handleUAGraph)
	mux.HandleFunc("/api/ua/validate", handleUAValidate)
	mux.HandleFunc("/api/ua/search", handleUASearch)

	// 代码模拟运行 API (v0.9.0)
	mux.HandleFunc("/api/simulate", handleSimulate)
	mux.HandleFunc("/api/entropy", handleEntropyCheck)
	mux.HandleFunc("/api/observe", handleObserveCheck)

	// Agent API 路由 (Phase 2 P6)
	mux.HandleFunc("/api/agents/events", handleAgentEvents)
	mux.HandleFunc("/api/agents/", handleAgentSubmissions)
	mux.HandleFunc("/api/agents", handleAgents)

	// 仪表盘新增 API (Phase B)
	// Observation API (lecore_tier4+)
	registerObservationHandlers(mux)

	// Guardian API
	initGuardian()

	// 迁移引擎 API (v0.11.0)
	initMigrateState(filepath.Join(sourceDir, ".migration-logs"))
	mux.HandleFunc("/api/migrate/analyze", handleMigrateAnalyze)
	mux.HandleFunc("/api/migrate/validate", handleMigrateValidate)
	mux.HandleFunc("/api/migrate/execute", handleMigrateExecute)
	mux.HandleFunc("/api/migrate/rollback", handleMigrateRollback)
	mux.HandleFunc("/api/migrate/sessions", handleMigrateSessions)
	mux.HandleFunc("/api/migrate/sessions/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleMigrateSessionDetail(w, r)
		case http.MethodDelete:
			handleMigrateSessionCancel(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/migrate/logs", handleMigrateLogs)
	mux.HandleFunc("/api/migrate/logs/export", handleMigrateLogsExport)
	mux.HandleFunc("/api/migrate/status", handleMigrateStatus)
	mux.HandleFunc("/api/sse/migrate", handleMigrateSSE)

	// 架构变动日志 API
	changelogStore = NewArchChangelogStore(filepath.Join(sourceDir, ".arch-changelog"))
	mux.HandleFunc("/api/arch-changelog", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleArchChangelog(w, r)
		case http.MethodPost:
			handleArchChangelogCreate(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/arch-changelog/export", handleArchChangelogExport)
	mux.HandleFunc("/api/arch-changelog/stats", handleArchChangelogStats)
	mux.HandleFunc("/api/sse/arch-changelog", handleArchChangelogSSE)

	mux.HandleFunc("/api/guardian/snapshot", handleGuardianSnapshot)
	mux.HandleFunc("/api/guardian/sse", handleGuardianSSE)
	mux.HandleFunc("/api/guardian/thresholds", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			handleGuardianThresholdsPut(w, r)
		} else {
			handleGuardianThresholds(w, r)
		}
	})
	mux.HandleFunc("/api/guardian/drift", handleGuardianDrift)
	mux.HandleFunc("/api/guardian/history", handleGuardianHistory)

	// Runtime API
	mux.HandleFunc("/api/runtime/tps", handleRuntimeTPS)
	mux.HandleFunc("/api/runtime/errors", handleRuntimeErrors)
	mux.HandleFunc("/api/runtime/latency", handleRuntimeLatency)
	mux.HandleFunc("/api/runtime/sampling-rate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			handleRuntimeSamplingRatePut(w, r)
		} else {
			handleRuntimeSamplingRate(w, r)
		}
	})

	// Primitives API
	mux.HandleFunc("/api/primitives", handlePrimitives)

	// Health Score History
	mux.HandleFunc("/api/health-score/history", handleHealthScoreHistory)

	// 静态文件 — 前端
	// 优先使用本地文件，否则使用嵌入式前端
	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/", fileServer)
}
