//go:build lecore_tier1 || lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"sync"
	"time"
)

// TierDriftPoint 表示一次漂移检测的历史记录点。
type TierDriftPoint struct {
	Timestamp    time.Time
	FileCount    int
	DetectedTier ComplexityTier
	CurrentTier  ComplexityTier
	DriftLevel   int
}

// TierDriftPrediction 预测何时需要升级 tier。
type TierDriftPrediction struct {
	EstimatedNextTier  ComplexityTier
	EstimatedTimeframe time.Duration
	GrowthRate         float64 // 文件增长率（files/week）
	Confidence         float64 // 预测置信度 (0-1)
}

// TierDriftReport 包含完整的漂移分析报告。
type TierDriftReport struct {
	Timestamp    time.Time
	ProjectStats ProjectStats
	CurrentTier  ComplexityTier
	DetectedTier ComplexityTier
	DriftLevel   int
	History      []TierDriftPoint
	Prediction   TierDriftPrediction
	Suggestion   string
}

// TierDriftMonitor 持续监控 tier 漂移。
type TierDriftMonitor struct {
	mu          sync.Mutex
	root        string
	currentTier ComplexityTier
	history     []TierDriftPoint
	maxHistory  int
}

// NewTierDriftMonitor 创建 tier 漂移监控器。
func NewTierDriftMonitor(root string, currentTier ComplexityTier) *TierDriftMonitor {
	return &TierDriftMonitor{
		root:        root,
		currentTier: currentTier,
		history:     make([]TierDriftPoint, 0),
		maxHistory:  100,
	}
}

// Check 执行一次漂移检测并记录到历史。
func (m *TierDriftMonitor) Check() TierDriftReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := ScanProject(m.root)
	detected := AutoDetectWithBoost(m.root)
	drift := int(detected) - int(m.currentTier)

	point := TierDriftPoint{
		Timestamp:    time.Now(),
		FileCount:    stats.TotalFiles,
		DetectedTier: detected,
		CurrentTier:  m.currentTier,
		DriftLevel:   drift,
	}

	m.history = append(m.history, point)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}

	report := TierDriftReport{
		Timestamp:    point.Timestamp,
		ProjectStats: stats,
		CurrentTier:  m.currentTier,
		DetectedTier: detected,
		DriftLevel:   drift,
		History:      m.copyHistory(),
		Prediction:   m.predictLocked(),
	}

	switch {
	case drift <= 0:
		report.Suggestion = "Tier is appropriate for current project scale."
	case drift == 1:
		report.Suggestion = "Consider planning a tier migration."
	default:
		report.Suggestion = "Urgent: project has significantly outgrown its tier."
	}

	return report
}

// PredictNextTier 基于历史趋势预测下一次 tier 升级的时间。
func (m *TierDriftMonitor) PredictNextTier() TierDriftPrediction {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.predictLocked()
}

func (m *TierDriftMonitor) predictLocked() TierDriftPrediction {
	if len(m.history) < 2 {
		return TierDriftPrediction{Confidence: 0}
	}

	first := m.history[0]
	last := m.history[len(m.history)-1]
	duration := last.Timestamp.Sub(first.Timestamp)
	if duration <= 0 {
		return TierDriftPrediction{Confidence: 0}
	}

	fileDelta := float64(last.FileCount - first.FileCount)
	weeks := duration.Hours() / (24 * 7)
	if weeks < 0.01 {
		weeks = 0.01
	}

	growthRate := fileDelta / weeks

	// 预测下一 tier 边界
	nextTier := m.currentTier + 1
	if nextTier > TierL7 {
		nextTier = TierL7
	}

	// 每个 tier 的文件数阈值
	thresholds := map[ComplexityTier]int{
		TierL0: 10, TierL1: 100, TierL2: 1000,
		TierL3: 10000, TierL4: 100000, TierL5: 1000000,
		TierL6: 10000000, TierL7: 100000000,
	}

	nextThreshold := thresholds[nextTier]
	remaining := float64(nextThreshold - last.FileCount)
	if remaining <= 0 {
		return TierDriftPrediction{
			EstimatedNextTier:  nextTier,
			EstimatedTimeframe: 0,
			GrowthRate:         growthRate,
			Confidence:         1.0,
		}
	}

	if growthRate <= 0 {
		return TierDriftPrediction{
			EstimatedNextTier: nextTier,
			GrowthRate:        growthRate,
			Confidence:        0.1,
		}
	}

	estimatedWeeks := remaining / growthRate
	estimatedDuration := time.Duration(estimatedWeeks * 7 * 24 * float64(time.Hour))

	confidence := 0.5
	if len(m.history) >= 5 {
		confidence = 0.8
	}
	if len(m.history) >= 10 {
		confidence = 0.95
	}

	return TierDriftPrediction{
		EstimatedNextTier:  nextTier,
		EstimatedTimeframe: estimatedDuration,
		GrowthRate:         growthRate,
		Confidence:         confidence,
	}
}

// History 返回历史记录的副本。
func (m *TierDriftMonitor) History() []TierDriftPoint {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.copyHistory()
}

func (m *TierDriftMonitor) copyHistory() []TierDriftPoint {
	dst := make([]TierDriftPoint, len(m.history))
	copy(dst, m.history)
	return dst
}

// SetCurrentTier 更新当前 tier（用于迁移完成后）。
func (m *TierDriftMonitor) SetCurrentTier(tier ComplexityTier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTier = tier
}