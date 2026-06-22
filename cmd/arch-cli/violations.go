// Package main — 架构违规检测（L7 入口）
//
// 总入口 + HTTP 处理器，具体规则位于 violations_rules.go。
// 规则对齐 go-core/arch 的 Violation / ViolationResponse 类型。
package main

import (
	"encoding/json"
	"net/http"

	arch "low-entropy-core/go-core/arch"
)

func handleViolations(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	resp := detectViolations(archData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(resp)
}

func detectViolations(data *arch.ArchData) arch.ViolationResponse {
	var items []arch.Violation

	rules := []func(*arch.ArchData) []arch.Violation{
		checkLargeFile,
		checkEmptyFile,
		checkCyclicDependency,
		checkLayerViolation,
		checkNoPrimitive,
		checkLargeFunction,
		checkRawPrint,
		checkHardcodedConfig,
		checkUncheckedError,
		checkTooManySymbols,
		checkGlobalState,
		checkLowTestCoverage,
		checkMissingDoc,
		checkHighCyclomatic,
		checkHighDependencyCentrality,
	}
	for _, fn := range rules {
		items = append(items, fn(data)...)
	}

	return summarizeViolations(items)
}
