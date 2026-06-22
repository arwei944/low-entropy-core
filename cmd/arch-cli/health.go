package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	arch "low-entropy-core/go-core/arch"
)

var healthHistory struct {
	mu      sync.RWMutex
	entries []healthHistoryEntry
	maxSize int
}

type healthHistoryEntry struct {
	Score     arch.HealthScoreResponse `json:"score"`
	Timestamp time.Time                `json:"timestamp"`
}

func init() {
	healthHistory.maxSize = 100
	healthHistory.entries = make([]healthHistoryEntry, 0, 100)
}

func recordHealthScore(score arch.HealthScoreResponse) {
	healthHistory.mu.Lock()
	defer healthHistory.mu.Unlock()
	healthHistory.entries = append(healthHistory.entries, healthHistoryEntry{
		Score:     score,
		Timestamp: time.Now(),
	})
	if len(healthHistory.entries) > healthHistory.maxSize {
		healthHistory.entries = healthHistory.entries[len(healthHistory.entries)-healthHistory.maxSize:]
	}
}

func handleHealthScoreHistory(w http.ResponseWriter, r *http.Request) {
	healthHistory.mu.RLock()
	defer healthHistory.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthHistory.entries)
}

func handleHealthScore(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	score := computeHealthScore(archData)
	recordHealthScore(score)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(score)
}

func computeHealthScore(data *arch.ArchData) arch.HealthScoreResponse {
	hs := arch.HealthScoreResponse{
		Factors:       make(map[string]float64),
		FactorDetails: []arch.HealthFactor{},
		Suggestions:   []string{},
	}

	// 1. 层级平衡度
	var layerFactor arch.HealthFactor
	layerBalance := 100.0
	rawDeviation := "0.00"
	if len(data.Layers) > 0 {
		var lines []int
		for _, l := range data.Layers {
			lines = append(lines, l.Lines)
		}
		avg := float64(data.TotalLines) / float64(len(data.Layers))
		deviation := 0.0
		for _, l := range lines {
			if avg > 0 {
				deviation += math.Abs(float64(l)-avg) / avg
			}
		}
		deviation /= float64(len(lines))
		layerBalance = math.Max(0, 100-deviation*30)
		rawDeviation = fmt.Sprintf("%.3f", deviation)
	}
	layerBalance = math.Round(layerBalance)
	layerSuggest := ""
	layerImpact := "low"
	if layerBalance < 60 {
		layerSuggest = "层级代码量分布不均，建议均衡各层职责"
		layerImpact = "high"
		hs.Suggestions = append(hs.Suggestions, layerSuggest)
	} else if layerBalance < 80 {
		layerSuggest = "层级分布略有偏差，可关注较少代码的层级"
		layerImpact = "medium"
	}
	layerFactor = arch.HealthFactor{
		Key:         "layer_balance",
		Name:        "层级平衡度",
		Score:       layerBalance,
		Explanation: "衡量各层代码量分布是否均衡，避免某一层职责过重或过轻",
		RawValue:    rawDeviation,
		Threshold:   "偏差 < 1.3 良好",
		Suggestion:  layerSuggest,
		Impact:      layerImpact,
	}
	hs.Factors["layer_balance"] = layerBalance
	hs.FactorDetails = append(hs.FactorDetails, layerFactor)

	// 2. 文件粒度
	avgLines := 0.0
	if data.TotalFiles > 0 {
		avgLines = float64(data.TotalLines) / float64(data.TotalFiles)
	}
	fileGranularity := 100.0
	fileSuggest := ""
	fileImpact := "low"
	if avgLines < 100 {
		fileGranularity = avgLines / 100 * 100
		fileSuggest = "文件粒度过细，建议合并相关文件"
		fileImpact = "medium"
		hs.Suggestions = append(hs.Suggestions, fileSuggest)
	} else if avgLines > 800 {
		fileGranularity = math.Max(0, 100-(avgLines-800)/10)
		fileSuggest = "部分文件过大，建议拆分"
		fileImpact = "high"
		hs.Suggestions = append(hs.Suggestions, fileSuggest)
	}
	fileGranularity = math.Round(fileGranularity)
	fileFactor := arch.HealthFactor{
		Key:         "file_granularity",
		Name:        "文件粒度",
		Score:       fileGranularity,
		Explanation: "平均每文件行数，反映文件职责是否适中",
		RawValue:    fmt.Sprintf("%.1f 行/文件", avgLines),
		Threshold:   "100 ~ 800 行/文件",
		Suggestion:  fileSuggest,
		Impact:      fileImpact,
	}
	hs.Factors["file_granularity"] = fileGranularity
	hs.FactorDetails = append(hs.FactorDetails, fileFactor)

	// 3. 符号密度
	avgSymbols := 0.0
	if data.TotalFiles > 0 {
		avgSymbols = float64(data.TotalSymbols) / float64(data.TotalFiles)
	}
	symbolDensity := 100.0
	symbolSuggest := ""
	symbolImpact := "low"
	if avgSymbols < 5 {
		symbolDensity = avgSymbols / 5 * 100
		symbolSuggest = "部分文件符号过少，可能存在未使用的代码"
		symbolImpact = "medium"
		hs.Suggestions = append(hs.Suggestions, symbolSuggest)
	} else if avgSymbols > 50 {
		symbolDensity = math.Max(0, 100-(avgSymbols-50)*2)
		symbolSuggest = "部分文件符号密度过高，建议拆分"
		symbolImpact = "high"
		hs.Suggestions = append(hs.Suggestions, symbolSuggest)
	}
	symbolDensity = math.Round(symbolDensity)
	symbolFactor := arch.HealthFactor{
		Key:         "symbol_density",
		Name:        "符号密度",
		Score:       symbolDensity,
		Explanation: "每文件平均符号（类型/函数/方法等）数",
		RawValue:    fmt.Sprintf("%.1f 符号/文件", avgSymbols),
		Threshold:   "5 ~ 50 符号/文件",
		Suggestion:  symbolSuggest,
		Impact:      symbolImpact,
	}
	hs.Factors["symbol_density"] = symbolDensity
	hs.FactorDetails = append(hs.FactorDetails, symbolFactor)

	// 4. 依赖深度
	totalDeps := 0
	for _, f := range data.Files {
		totalDeps += len(f.DependsOn)
	}
	avgDeps := 0.0
	if data.TotalFiles > 0 {
		avgDeps = float64(totalDeps) / float64(data.TotalFiles)
	}
	depDepth := 100.0
	depSuggest := ""
	depImpact := "low"
	if avgDeps > 10 {
		depDepth = math.Max(0, 100-(avgDeps-10)*5)
		depSuggest = "平均依赖数偏高，建议降低耦合度"
		depImpact = "high"
		hs.Suggestions = append(hs.Suggestions, depSuggest)
	}
	depDepth = math.Round(depDepth)
	depFactor := arch.HealthFactor{
		Key:         "dependency_depth",
		Name:        "依赖深度",
		Score:       depDepth,
		Explanation: "平均每个文件的依赖数，反映模块耦合程度",
		RawValue:    fmt.Sprintf("%.1f 依赖/文件", avgDeps),
		Threshold:   "<= 10 依赖/文件",
		Suggestion:  depSuggest,
		Impact:      depImpact,
	}
	hs.Factors["dependency_depth"] = depDepth
	hs.FactorDetails = append(hs.FactorDetails, depFactor)

	// 5. 接口率
	typeCount := 0
	interfaceCount := 0
	for _, f := range data.Files {
		for _, s := range f.Symbols {
			if s.Kind == "type" || s.Kind == "interface" {
				typeCount++
				if s.Kind == "interface" {
					interfaceCount++
				}
			}
		}
	}
	ratio := 0.0
	if typeCount > 0 {
		ratio = float64(interfaceCount) / float64(typeCount)
	}
	interfaceRatio := 100.0
	ratioSuggest := ""
	ratioImpact := "low"
	if typeCount > 0 {
		if ratio < 0.1 {
			interfaceRatio = ratio / 0.1 * 100
			ratioSuggest = "接口比例偏低，建议增加抽象层"
			ratioImpact = "high"
			hs.Suggestions = append(hs.Suggestions, ratioSuggest)
		} else if ratio > 0.7 {
			interfaceRatio = 100 - (ratio-0.7)*100
			ratioSuggest = "接口比例偏高，可能存在过度抽象"
			ratioImpact = "medium"
			hs.Suggestions = append(hs.Suggestions, ratioSuggest)
		}
	}
	interfaceRatio = math.Round(interfaceRatio)
	ratioFactor := arch.HealthFactor{
		Key:         "interface_ratio",
		Name:        "接口率",
		Score:       interfaceRatio,
		Explanation: "接口类型占总类型的比例，反映抽象程度",
		RawValue:    fmt.Sprintf("%d/%d (%.1f%%)", interfaceCount, typeCount, ratio*100),
		Threshold:   "10% ~ 70%",
		Suggestion:  ratioSuggest,
		Impact:      ratioImpact,
	}
	hs.Factors["interface_ratio"] = interfaceRatio
	hs.FactorDetails = append(hs.FactorDetails, ratioFactor)

	// 加权总分
	hs.Overall = math.Round(layerBalance*0.25 + fileGranularity*0.20 + symbolDensity*0.20 + depDepth*0.20 + interfaceRatio*0.15)

	// 评级
	switch {
	case hs.Overall >= 90:
		hs.Grade = "A+"
	case hs.Overall >= 80:
		hs.Grade = "A"
	case hs.Overall >= 70:
		hs.Grade = "B"
	case hs.Overall >= 60:
		hs.Grade = "C"
	default:
		hs.Grade = "D"
	}
	hs.GradeDesc = arch.GradeDescription(hs.Grade)

	// 项目统计
	primitiveCount := len(data.Primitives)
	hs.ProjectStats = arch.ProjectStats{
		TotalFiles:        data.TotalFiles,
		TotalLines:        data.TotalLines,
		AvgLinesPerFile:   avgLines,
		AvgSymbolsPerFile: avgSymbols,
		PrimitiveCount:    primitiveCount,
	}

	return hs
}
