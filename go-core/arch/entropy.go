// Package arch — Entropy 熵值原语 (L1 Atom)
//
// 纯函数实现。计算架构数据的各种熵值指标，用于量化：
//   - 每个层级的代码复杂度分布
//   - 依赖深度
//   - 架构漂移分数
//   - 风险等级
package arch

import (
	"math"
	"time"
)

// EntropyMetric 表示单条熵值度量。
type EntropyMetric struct {
	Module     string  `json:"module"`
	Layer      string  `json:"layer"`
	FileCount  int     `json:"file_count"`
	LineCount  int     `json:"line_count"`
	Cyclomatic float64 `json:"cyclomatic"` // 平均圈复杂度（用符号密度近似）
	Depth      float64 `json:"depth"`       // 平均依赖深度
	DriftScore float64 `json:"drift_score"` // 架构漂移分数
	RiskLevel  string  `json:"risk_level"`  // "low" | "medium" | "high" | "critical"
	Timestamp  time.Time `json:"timestamp"`
}

// ObservedMetrics 观测指标（包含编译/测试/静态分析状态）。
type ObservedMetrics struct {
	CompileTime   string  `json:"compile_time"`
	CompileStatus string  `json:"compile_status"`
	TestTime      string  `json:"test_time"`
	TestPassRate  float64 `json:"test_pass_rate"`
	CoverageRate  float64 `json:"coverage_rate"`
	RaceDetected  bool    `json:"race_detected"`
	StaticIssues  int     `json:"static_issues"`
	ComplexityAvg float64 `json:"complexity_avg"`
	ComplexityMax float64 `json:"complexity_max"`
	Timestamp     string  `json:"timestamp"`
}

// ComputeEntropy 计算架构数据的熵值指标（按层级分组）。
// 纯函数：输入 ArchData → 输出 EntropyMetric[]。
func ComputeEntropy(data *ArchData) []EntropyMetric {
	if data == nil || len(data.Files) == 0 {
		return nil
	}

	// 按层级分组
	layerFiles := make(map[string][]FileInfo)
	for _, f := range data.Files {
		layer := f.Layer
		if layer == "" {
			layer = "L7"
		}
		layerFiles[layer] = append(layerFiles[layer], f)
	}

	// 固定顺序输出
	layerOrder := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	result := make([]EntropyMetric, 0, len(layerOrder))
	now := time.Now()

	for _, layer := range layerOrder {
		files := layerFiles[layer]
		if len(files) == 0 {
			continue
		}

		totalLines := 0
		totalSymbols := 0
		maxDepth := 0.0
		cycloSum := 0.0

		for _, f := range files {
			totalLines += f.Lines
			totalSymbols += len(f.Symbols)
			dep := float64(len(f.DependsOn))
			if dep > maxDepth {
				maxDepth = dep
			}
			// 圈复杂度近似 = 每百行符号数的自然对数 + 1
			if f.Lines > 0 {
				cycloSum += math.Log(float64(len(f.Symbols))+1) + 1.0
			}
		}

		avgCyclo := 0.0
		if len(files) > 0 {
			avgCyclo = cycloSum / float64(len(files))
		}

		// 架构漂移 = 综合变化倾向 / 规模
		drift := 0.0
		if totalLines > 0 {
			drift = (float64(totalSymbols) / float64(totalLines)) * 100
			drift = math.Min(1.0, drift)
		}

		// 风险等级：根据圈复杂度 + 依赖深度 + 漂移
		riskScore := avgCyclo*0.4 + maxDepth*0.1 + drift*0.5
		risk := "low"
		switch {
		case riskScore >= 3.0:
			risk = "critical"
		case riskScore >= 2.0:
			risk = "high"
		case riskScore >= 1.0:
			risk = "medium"
		}

		result = append(result, EntropyMetric{
			Module:     "arch",
			Layer:      layer,
			FileCount:  len(files),
			LineCount:  totalLines,
			Cyclomatic: round2(avgCyclo),
			Depth:      round2(maxDepth),
			DriftScore: round2(drift),
			RiskLevel:  risk,
			Timestamp:  now,
		})
	}

	return result
}

// ComputeOverallEntropy 计算整个项目的综合熵值。
func ComputeOverallEntropy(data *ArchData) EntropyMetric {
	metrics := ComputeEntropy(data)
	if len(metrics) == 0 {
		return EntropyMetric{RiskLevel: "low", Timestamp: time.Now()}
	}

	totalFiles := 0
	totalLines := 0
	sumCyclo := 0.0
	sumDepth := 0.0
	sumDrift := 0.0

	for _, m := range metrics {
		totalFiles += m.FileCount
		totalLines += m.LineCount
		sumCyclo += m.Cyclomatic
		sumDepth += m.Depth
		sumDrift += m.DriftScore
	}

	n := float64(len(metrics))
	overallRisk := "low"
	avgScore := (sumCyclo/n)*0.4 + (sumDepth/n)*0.1 + (sumDrift/n)*0.5
	switch {
	case avgScore >= 3.0:
		overallRisk = "critical"
	case avgScore >= 2.0:
		overallRisk = "high"
	case avgScore >= 1.0:
		overallRisk = "medium"
	}

	return EntropyMetric{
		Module:     "overall",
		Layer:      "ALL",
		FileCount:  totalFiles,
		LineCount:  totalLines,
		Cyclomatic: round2(sumCyclo / n),
		Depth:      round2(sumDepth / n),
		DriftScore: round2(sumDrift / n),
		RiskLevel:  overallRisk,
		Timestamp:  time.Now(),
	}
}

// ValidateStructure 做结构层面的快速健康检查。
// 返回：健康度分数 (0.0~1.0) + 建议列表
func ValidateStructure(data *ArchData) (float64, []string) {
	if data == nil || len(data.Files) == 0 {
		return 0.0, []string{"项目为空"}
	}

	suggestions := make([]string, 0)
	score := 1.0

	// 1. 文件行数检查
	tooLong := 0
	for _, f := range data.Files {
		if f.Lines > 300 {
			tooLong++
		}
	}
	if tooLong > 0 {
		score -= 0.1 * float64(tooLong) / float64(len(data.Files))
		suggestions = append(suggestions,
			"有 "+itoa(tooLong)+" 个文件超过 300 行，建议拆分")
	}

	// 2. 层级完整性
	coveredLayers := make(map[string]bool)
	for _, f := range data.Files {
		coveredLayers[f.Layer] = true
	}
	if len(coveredLayers) < 3 {
		score -= 0.05
		suggestions = append(suggestions,
			"架构层级分布较稀疏（<3 层）")
	}

	// 3. 依赖深度检查
	maxDepth := 0
	for _, f := range data.Files {
		if len(f.DependsOn) > maxDepth {
			maxDepth = len(f.DependsOn)
		}
	}
	if maxDepth > 10 {
		score -= 0.05
		suggestions = append(suggestions,
			"最大依赖深度较高 ("+itoa(maxDepth)+")")
	}

	if score < 0 {
		score = 0
	}
	return round2(score), suggestions
}

// itoa 由 analyzer.go 提供（同包共享）
