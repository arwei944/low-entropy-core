//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 异常标签 (v4.0)
package core

import "math"

// AnomalyLabelType 表示异常类型。
type AnomalyLabelType string

const (
	AnomalyNormal             AnomalyLabelType = "normal"
	AnomalyLatency            AnomalyLabelType = "latency_anomaly"
	AnomalyError              AnomalyLabelType = "error_anomaly"
	AnomalyDistribution       AnomalyLabelType = "distribution_anomaly"
	AnomalyLatencyAndError    AnomalyLabelType = "latency_error_anomaly"
)

// AnomalyAutoLabeler 自动标注异常聚合结果。
// 基于三个条件：P99 超过基线 3σ、错误率飙升、分布漂移。
type AnomalyAutoLabeler struct {
	baselineP99Mean   float64
	baselineP99Std    float64
	baselineErrorRate float64
	baselineErrorStd  float64
	sampleCount       int
	distributionDrift *DistributionDriftDetector
}

// NewAnomalyAutoLabeler 创建异常自动标注器。
func NewAnomalyAutoLabeler() *AnomalyAutoLabeler {
	return &AnomalyAutoLabeler{
		distributionDrift: NewDistributionDriftDetector(0.1),
	}
}

// UpdateBaseline 更新历史基线。
// p99 和 errorRate 是当前窗口的值。
func (l *AnomalyAutoLabeler) UpdateBaseline(p99, errorRate float64) {
	l.sampleCount++

	// EMA 更新 P99 均值
	if l.sampleCount == 1 {
		l.baselineP99Mean = p99
		l.baselineErrorRate = errorRate
	} else {
		alpha := 0.1
		diffP99 := p99 - l.baselineP99Mean
		l.baselineP99Mean += alpha * diffP99
		l.baselineP99Std = math.Sqrt(alpha*diffP99*diffP99 + (1-alpha)*l.baselineP99Std*l.baselineP99Std)

		diffErr := errorRate - l.baselineErrorRate
		l.baselineErrorRate += alpha * diffErr
		l.baselineErrorStd = math.Sqrt(alpha*diffErr*diffErr + (1-alpha)*l.baselineErrorStd*l.baselineErrorStd)
	}
}

// Label 对聚合结果进行异常标注。
// 返回异常标签和异常分数（0-1）。
func (l *AnomalyAutoLabeler) Label(p99 float64, errorRate float64, drifted bool, driftScore float64) (AnomalyLabelType, float64) {
	if l.sampleCount < 10 {
		return AnomalyNormal, 0
	}

	latencyAnomaly := false
	errorAnomaly := false
	anomalyScore := 0.0

	// 检查 P99 延迟异常（超过基线 3σ）
	if l.baselineP99Std > 0 {
		upperLimit := l.baselineP99Mean + 3*l.baselineP99Std
		if p99 > upperLimit {
			latencyAnomaly = true
			anomalyScore = math.Max(anomalyScore, (p99-l.baselineP99Mean)/l.baselineP99Std/3.0)
		}
	}

	// 检查错误率异常
	if l.baselineErrorStd > 0 {
		upperLimit := l.baselineErrorRate + 3*l.baselineErrorStd
		if errorRate > upperLimit {
			errorAnomaly = true
			anomalyScore = math.Max(anomalyScore, (errorRate-l.baselineErrorRate)/l.baselineErrorStd/3.0)
		}
	}

	// 检查分布漂移
	if drifted {
		anomalyScore = math.Max(anomalyScore, driftScore)
	}

	// 分类
	switch {
	case latencyAnomaly && errorAnomaly:
		return AnomalyLatencyAndError, anomalyScore
	case latencyAnomaly:
		return AnomalyLatency, anomalyScore
	case errorAnomaly:
		return AnomalyError, anomalyScore
	case drifted:
		return AnomalyDistribution, anomalyScore
	default:
		return AnomalyNormal, anomalyScore
	}
}

// GetBaselineStats 返回当前基线统计。
func (l *AnomalyAutoLabeler) GetBaselineStats() (p99Mean, p99Std, errorRateMean, errorRateStd float64) {
	return l.baselineP99Mean, l.baselineP99Std, l.baselineErrorRate, l.baselineErrorStd
}
