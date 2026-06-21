package main

import (
	"encoding/json"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// EntropyMetrics 熵度量
type EntropyMetrics struct {
	Module     string    `json:"module"`
	FileCount  int       `json:"file_count"`
	LineCount  int       `json:"line_count"`
	Cyclomatic float64   `json:"cyclomatic"`    // 平均圈复杂度
	Depth      float64   `json:"depth"`          // 最大依赖深度
	DriftScore float64   `json:"drift_score"`    // 架构漂移分数
	Layer      string    `json:"layer"`
	RiskLevel  string    `json:"risk_level"`     // "low", "medium", "high", "critical"
	Timestamp  time.Time `json:"timestamp"`
}

// ObservedMetrics 观测指标
type ObservedMetrics struct {
	CompileTime   string  `json:"compile_time"`
	CompileStatus string  `json:"compile_status"`
	TestTime      string  `json:"test_time"`
	TestPassRate  float64 `json:"test_pass_rate"`
	CoverageRate  float64 `json:"coverage_rate"`
	RaceDetected  bool    `json:"race_detected"`
	StaticIssues  int     `json:"static_issues"`  // go vet 问题数
	ComplexityAvg float64 `json:"complexity_avg"`
	ComplexityMax float64 `json:"complexity_max"`
	Timestamp     string  `json:"timestamp"`
}

// handleEntropyCheck 计算熵度量
// GET /api/entropy
func handleEntropyCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	archMu.RLock()
	ad := archData
	archMu.RUnlock()

	metrics := make([]EntropyMetrics, 0)
	now := time.Now()

	// 计算每个模块的熵
	layerFiles := make(map[string][]FileInfo)
	for _, f := range ad.Files {
		layer := f.Layer
		if layer == "" {
			layer = "L0"
		}
		layerFiles[layer] = append(layerFiles[layer], f)
	}

	layerOrder := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	for _, layer := range layerOrder {
		files := layerFiles[layer]
		if len(files) == 0 {
			continue
		}

		totalLines := 0
		totalSymbols := 0
		maxDepth := 0.0
		totalComplexity := 0.0

		for _, f := range files {
			totalLines += f.Lines
			totalSymbols += len(f.Symbols)
			// 依赖深度
			depDepth := float64(len(f.DependsOn))
			if depDepth > maxDepth {
				maxDepth = depDepth
			}
			// 简单复杂度估算：符号数 / 文件大小
			if f.Lines > 0 {
				totalComplexity += float64(len(f.Symbols)) / float64(f.Lines) * 10
			}
		}

		avgComplexity := 0.0
		if len(files) > 0 {
			avgComplexity = totalComplexity / float64(len(files))
		}

		// 漂移分数：综合考虑文件大小、复杂度和依赖深度
		avgLines := float64(totalLines) / float64(len(files))
		driftScore := (avgLines / 200.0) + (avgComplexity / 5.0) + (maxDepth / 20.0)
		// 限制精度到 1 位小数
		driftScore = math.Round(driftScore*10) / 10

		riskLevel := "low"
		if driftScore >= 5.0 {
			riskLevel = "critical"
		} else if driftScore >= 3.0 {
			riskLevel = "high"
		} else if driftScore >= 1.5 {
			riskLevel = "medium"
		}

		metrics = append(metrics, EntropyMetrics{
			Module:     layer,
			FileCount:  len(files),
			LineCount:  totalLines,
			Cyclomatic: avgComplexity,
			Depth:      maxDepth,
			DriftScore: driftScore,
			Layer:      layer,
			RiskLevel:  riskLevel,
			Timestamp:  now,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics":     metrics,
		"total_files": ad.TotalFiles,
		"total_lines": ad.TotalLines,
		"timestamp":   now,
	})
}

// handleObserveCheck 运行观测指标
// GET /api/observe
func handleObserveCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	start := time.Now()
	pkg := r.URL.Query().Get("pkg")
	if pkg == "" {
		pkg = "."
	}

	metrics := ObservedMetrics{Timestamp: time.Now().Format(time.RFC3339)}

	// 1. 编译检查
	buildStart := time.Now()
	buildCmd := exec.Command("go", "build", pkg)
	buildCmd.Dir = sourceDir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	metrics.CompileTime = time.Since(buildStart).Round(time.Millisecond).String()
	metrics.CompileStatus = "pass"
	if buildErr != nil {
		metrics.CompileStatus = "fail"
	}

	// 2. go vet 静态分析（过滤编译器噪音）
	vetCmd := exec.Command("go", "vet", pkg)
	vetCmd.Dir = sourceDir
	vetRawOutput, _ := vetCmd.CombinedOutput()
	// 过滤掉 runtime warning 等噪音，只统计真正的 vet 问题
	realIssues := 0
	for _, line := range strings.Split(strings.TrimSpace(string(vetRawOutput)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 跳过 Go 编译器内部警告
		if strings.Contains(line, "runtime: warning:") ||
			strings.HasPrefix(line, "# ") ||
			strings.Contains(line, "IsLongPathAwareProcess") ||
			strings.Contains(line, "GOPATH set to GOROOT") ||
			strings.HasPrefix(line, "warning:") {
			continue
		}
		realIssues++
	}
	metrics.StaticIssues = realIssues

	// 3. 测试运行
	testStart := time.Now()
	testCmd := exec.Command("go", "test", pkg, "-count=1", "-cover", "-timeout=60s")
	testCmd.Dir = sourceDir
	testOutput, testErr := testCmd.CombinedOutput()
	metrics.TestTime = time.Since(testStart).Round(time.Millisecond).String()

	// 解析测试输出
	output := string(testOutput) + string(buildOutput)
	lines := strings.Split(output, "\n")
	passCount := 0
	failCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "--- PASS:") {
			passCount++
		} else if strings.HasPrefix(line, "--- FAIL:") {
			failCount++
		} else if strings.Contains(line, "coverage:") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasSuffix(p, "%") {
					metrics.CoverageRate, _ = parsePercent(p)
				}
			}
		}
	}

	total := passCount + failCount
	if total > 0 {
		metrics.TestPassRate = float64(passCount) / float64(total) * 100
	} else {
		metrics.TestPassRate = 100
	}

	_ = testErr // 使用但不强制

	// 4. 复杂度分析
	archMu.RLock()
	ad := archData
	archMu.RUnlock()

	totalComplexity := 0.0
	maxComplexity := 0.0
	symbolCount := 0
	for _, f := range ad.Files {
		if f.Lines > 0 {
			complexity := float64(len(f.Symbols)) / float64(f.Lines) * 10
			totalComplexity += complexity
			if complexity > maxComplexity {
				maxComplexity = complexity
			}
			symbolCount++
		}
	}
	if symbolCount > 0 {
		metrics.ComplexityAvg = totalComplexity / float64(symbolCount)
	}
	metrics.ComplexityMax = maxComplexity

	metrics.Timestamp = time.Now().Format(time.RFC3339)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics":       metrics,
		"total_elapsed": time.Since(start).Round(time.Millisecond).String(),
	})
}
