package main

import (
	"encoding/json"
	"math"
	"net/http"
	"sync"
	"time"
)

// healthHistory 健康评分历史（环形缓冲区）
var healthHistory struct {
	mu       sync.RWMutex
	entries  []healthHistoryEntry
	maxSize  int
}

type healthHistoryEntry struct {
	Score     HealthScore `json:"score"`
	Timestamp time.Time   `json:"timestamp"`
}

func init() {
	healthHistory.maxSize = 100
	healthHistory.entries = make([]healthHistoryEntry, 0, 100)
}

// recordHealthScore 记录健康评分到历史
func recordHealthScore(score HealthScore) {
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

// handleHealthScoreHistory 返回健康评分历史
func handleHealthScoreHistory(w http.ResponseWriter, r *http.Request) {
	healthHistory.mu.RLock()
	defer healthHistory.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthHistory.entries)
}

// handleHealthScore 计算架构健康度评分
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

func computeHealthScore(data *ArchData) HealthScore {
	hs := HealthScore{
		Factors:     make(map[string]float64),
		Suggestions: []string{},
	}

	// 1. 层级平衡度 (25分) — 每层是否有足够的代码量
	layerBalance := 100.0
	if len(data.Layers) > 0 {
		var lines []int
		for _, l := range data.Layers {
			lines = append(lines, l.Lines)
		}
		avg := float64(data.TotalLines) / float64(len(data.Layers))
		deviation := 0.0
		for _, l := range lines {
			deviation += math.Abs(float64(l)-avg) / avg
		}
		deviation /= float64(len(lines))
		layerBalance = math.Max(0, 100-deviation*30)
		if layerBalance < 60 {
			hs.Suggestions = append(hs.Suggestions, "层级代码量分布不均，建议均衡各层职责")
		}
	}
	hs.Factors["layer_balance"] = math.Round(layerBalance)

	// 2. 文件粒度 (20分) — 平均每文件行数应在 100-500 之间
	avgLines := float64(data.TotalLines) / float64(data.TotalFiles)
	fileGranularity := 100.0
	if avgLines < 100 {
		fileGranularity = avgLines / 100 * 100
		hs.Suggestions = append(hs.Suggestions, "文件粒度过细，建议合并相关文件")
	} else if avgLines > 800 {
		fileGranularity = math.Max(0, 100-(avgLines-800)/10)
		hs.Suggestions = append(hs.Suggestions, "部分文件过大，建议拆分")
	}
	hs.Factors["file_granularity"] = math.Round(fileGranularity)

	// 3. 符号密度 (20分) — 每文件平均符号数
	avgSymbols := float64(data.TotalSymbols) / float64(data.TotalFiles)
	symbolDensity := 100.0
	if avgSymbols < 5 {
		symbolDensity = avgSymbols / 5 * 100
		hs.Suggestions = append(hs.Suggestions, "部分文件符号过少，可能存在未使用的代码")
	} else if avgSymbols > 50 {
		symbolDensity = math.Max(0, 100-(avgSymbols-50)*2)
		hs.Suggestions = append(hs.Suggestions, "部分文件符号密度过高，建议拆分")
	}
	hs.Factors["symbol_density"] = math.Round(symbolDensity)

	// 4. 依赖深度 (20分) — 平均依赖深度
	totalDeps := 0
	for _, f := range data.Files {
		totalDeps += len(f.DependsOn)
	}
	avgDeps := float64(totalDeps) / float64(data.TotalFiles)
	depDepth := 100.0
	if avgDeps > 10 {
		depDepth = math.Max(0, 100-(avgDeps-10)*5)
		hs.Suggestions = append(hs.Suggestions, "平均依赖数偏高，建议降低耦合度")
	}
	hs.Factors["dependency_depth"] = math.Round(depDepth)

	// 5. 接口率 (15分) — 接口类型占总类型的比例
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
	interfaceRatio := 100.0
	if typeCount > 0 {
		ratio := float64(interfaceCount) / float64(typeCount)
		if ratio < 0.1 {
			interfaceRatio = ratio / 0.1 * 100
			hs.Suggestions = append(hs.Suggestions, "接口比例偏低，建议增加抽象层")
		} else if ratio > 0.7 {
			interfaceRatio = 100 - (ratio-0.7)*100
			hs.Suggestions = append(hs.Suggestions, "接口比例偏高，可能存在过度抽象")
		}
	}
	hs.Factors["interface_ratio"] = math.Round(interfaceRatio)

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

	return hs
}
