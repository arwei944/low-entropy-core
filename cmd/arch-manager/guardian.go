//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	core "low-entropy-core/go-core"
)

// guardianState 全局 Guardian 状态
type guardianState struct {
	mu         sync.RWMutex
	collector  *core.MultiDimensionEntropyCollector
	history    []core.MultiDimensionEntropySnapshot
	maxHistory int
	thresholds struct {
		Yellow int
		Orange int
		Red    int
	}
	overrides []thresholdOverride
}

// thresholdOverride 阈值覆盖记录
type thresholdOverride struct {
	Level      string    `json:"level"`
	Value      int       `json:"value"`
	Reason     string    `json:"reason"`
	Timestamp  time.Time `json:"timestamp"`
	AutoRevert time.Time `json:"auto_revert,omitempty"`
}

var gState = &guardianState{
	maxHistory: 100,
	thresholds: struct {
		Yellow int
		Orange int
		Red    int
	}{20, 50, 100},
}

// initGuardian 初始化 Guardian 监控
func initGuardian() {
	gState.mu.Lock()
	defer gState.mu.Unlock()
	gState.collector = core.NewMultiDimensionEntropyCollector(
		core.NewModuleEntropyTracker(),
		core.NewPipelineStepGrowthDetector(),
		core.NewAgentBehaviorDriftDetector(),
	)
}

// handleGuardianSnapshot 返回多维度熵值快照
func handleGuardianSnapshot(w http.ResponseWriter, r *http.Request) {
	gState.mu.RLock()
	defer gState.mu.RUnlock()

	if gState.collector == nil {
		http.Error(w, `{"error":"guardian not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	snapshot := gState.collector.Collect()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// handleGuardianSSE SSE 推送熵值快照
func handleGuardianSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	dataTicker := time.NewTicker(5 * time.Second)
	defer dataTicker.Stop()
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-dataTicker.C:
			gState.mu.RLock()
			if gState.collector != nil {
				snapshot := gState.collector.Collect()
				gState.mu.RUnlock()
				data, _ := json.Marshal(snapshot)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			} else {
				gState.mu.RUnlock()
			}
		case <-pingTicker.C:
			pingData, _ := json.Marshal(map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			fmt.Fprintf(w, "data: %s\n\n", pingData)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleGuardianThresholds GET 当前阈值
func handleGuardianThresholds(w http.ResponseWriter, r *http.Request) {
	gState.mu.RLock()
	defer gState.mu.RUnlock()

	resp := map[string]any{
		"yellow":    gState.thresholds.Yellow,
		"orange":    gState.thresholds.Orange,
		"red":       gState.thresholds.Red,
		"overrides": gState.overrides,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGuardianThresholdsPut PUT 覆盖阈值
func handleGuardianThresholdsPut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level      string `json:"level"`
		Value      int    `json:"value"`
		Reason     string `json:"reason"`
		AutoRevert int    `json:"auto_revert_minutes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	gState.mu.Lock()
	defer gState.mu.Unlock()

	ov := thresholdOverride{
		Level:     req.Level,
		Value:     req.Value,
		Reason:    req.Reason,
		Timestamp: time.Now(),
	}
	if req.AutoRevert > 0 {
		ov.AutoRevert = time.Now().Add(time.Duration(req.AutoRevert) * time.Minute)
	}
	gState.overrides = append(gState.overrides, ov)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleGuardianDrift 返回漂移检测状态
func handleGuardianDrift(w http.ResponseWriter, r *http.Request) {
	gState.mu.RLock()
	defer gState.mu.RUnlock()

	if gState.collector == nil {
		http.Error(w, `{"error":"guardian not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	snapshot := gState.collector.Collect()
	drift := map[string]any{
		"agents_with_drift":    snapshot.AgentsWithEarlyWarn,
		"pipelines_with_growth": snapshot.PipelinesWithGrowth,
		"timestamp":            time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drift)
}

// handleGuardianHistory 返回熵值历史
func handleGuardianHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	gState.mu.RLock()
	defer gState.mu.RUnlock()

	start := 0
	if len(gState.history) > limit {
		start = len(gState.history) - limit
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gState.history[start:])
}

// recordSnapshot 记录熵值快照到历史
func recordSnapshot() {
	gState.mu.Lock()
	defer gState.mu.Unlock()
	if gState.collector == nil {
		return
	}
	snapshot := gState.collector.Collect()
	gState.history = append(gState.history, *snapshot)
	if len(gState.history) > gState.maxHistory {
		gState.history = gState.history[len(gState.history)-gState.maxHistory:]
	}
}
