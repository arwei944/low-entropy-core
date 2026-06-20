package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// runtimeState 运行时统计状态
type runtimeState struct {
	mu           sync.RWMutex
	samplingRate float64 // 0.01 - 1.0
	tpsHistory   []tpsPoint
	errorHistory []errorPoint
	latencyHist  []latencyPoint
}

type tpsPoint struct {
	TPS       int       `json:"tps"`
	Timestamp time.Time `json:"timestamp"`
}
type errorPoint struct {
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}
type latencyPoint struct {
	P50       float64   `json:"p50"`
	P95       float64   `json:"p95"`
	P99       float64   `json:"p99"`
	Timestamp time.Time `json:"timestamp"`
}

var rtState = &runtimeState{
	samplingRate: 0.1, // 默认 10%
}

// handleRuntimeTPS 返回当前 TPS
func handleRuntimeTPS(w http.ResponseWriter, r *http.Request) {
	rtState.mu.RLock()
	defer rtState.mu.RUnlock()

	currentTPS := 0
	if len(rtState.tpsHistory) > 0 {
		currentTPS = rtState.tpsHistory[len(rtState.tpsHistory)-1].TPS
	}

	resp := map[string]any{
		"current": currentTPS,
		"history": rtState.tpsHistory,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRuntimeErrors 返回最近错误列表
func handleRuntimeErrors(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	rtState.mu.RLock()
	defer rtState.mu.RUnlock()

	start := 0
	if len(rtState.errorHistory) > limit {
		start = len(rtState.errorHistory) - limit
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rtState.errorHistory[start:])
}

// handleRuntimeLatency 返回延迟分位数
func handleRuntimeLatency(w http.ResponseWriter, r *http.Request) {
	rtState.mu.RLock()
	defer rtState.mu.RUnlock()

	var p50, p95, p99 float64
	if len(rtState.latencyHist) > 0 {
		latest := rtState.latencyHist[len(rtState.latencyHist)-1]
		p50 = latest.P50
		p95 = latest.P95
		p99 = latest.P99
	}

	resp := map[string]any{
		"p50":       p50,
		"p95":       p95,
		"p99":       p99,
		"history":   rtState.latencyHist,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRuntimeSamplingRate GET 当前采样率
func handleRuntimeSamplingRate(w http.ResponseWriter, r *http.Request) {
	rtState.mu.RLock()
	defer rtState.mu.RUnlock()

	resp := map[string]any{
		"rate":      rtState.samplingRate,
		"mode":      samplingMode(rtState.samplingRate),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRuntimeSamplingRatePut PUT 修改采样率
func handleRuntimeSamplingRatePut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rate float64 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Rate < 0.01 || req.Rate > 1.0 {
		http.Error(w, `{"error":"rate must be between 0.01 and 1.0"}`, http.StatusBadRequest)
		return
	}

	rtState.mu.Lock()
	rtState.samplingRate = req.Rate
	rtState.mu.Unlock()

	resp := map[string]any{
		"rate":   req.Rate,
		"mode":   samplingMode(req.Rate),
		"status": "ok",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// samplingMode 根据采样率返回模式名称
func samplingMode(rate float64) string {
	switch {
	case rate >= 0.9:
		return "dev"
	case rate >= 0.05:
		return "staging"
	default:
		return "production"
	}
}

// recordTPS 记录 TPS（供内部调用）
func recordTPS(tps int) {
	rtState.mu.Lock()
	defer rtState.mu.Unlock()
	rtState.tpsHistory = append(rtState.tpsHistory, tpsPoint{TPS: tps, Timestamp: time.Now()})
	if len(rtState.tpsHistory) > 100 {
		rtState.tpsHistory = rtState.tpsHistory[len(rtState.tpsHistory)-100:]
	}
}

// recordError 记录错误（供内部调用）
func recordError(err string) {
	rtState.mu.Lock()
	defer rtState.mu.Unlock()
	rtState.errorHistory = append(rtState.errorHistory, errorPoint{Error: err, Timestamp: time.Now()})
	if len(rtState.errorHistory) > 100 {
		rtState.errorHistory = rtState.errorHistory[len(rtState.errorHistory)-100:]
	}
}

// recordLatency 记录延迟（供内部调用）
func recordLatency(p50, p95, p99 float64) {
	rtState.mu.Lock()
	defer rtState.mu.Unlock()
	rtState.latencyHist = append(rtState.latencyHist, latencyPoint{P50: p50, P95: p95, P99: p99, Timestamp: time.Now()})
	if len(rtState.latencyHist) > 100 {
		rtState.latencyHist = rtState.latencyHist[len(rtState.latencyHist)-100:]
	}
}

// fmtDuration 格式化持续时间
func fmtDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d)/float64(time.Microsecond))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
}
