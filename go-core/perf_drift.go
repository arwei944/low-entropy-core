//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 分布漂移检测 (v4.0)
package core

import "math"

// DriftDirection 表示分布漂移的方向。
type DriftDirection int

const (
	DriftDirNone   DriftDirection = iota
	DriftDirRight  // 右移（延迟增加）
	DriftDirLeft   // 左移（延迟减少）
)

// DistributionDriftResult 是分布漂移检测的结果。
type DistributionDriftResult struct {
	Distance      float64       // 两个分布之间的 Wasserstein 距离
	Drifted       bool          // 是否检测到漂移
	Direction     DriftDirection
	CurrentMean   float64
	BaselineMean  float64
	Threshold     float64
}

// DistributionDriftDetector 比较当前窗口与历史窗口的延迟分布。
// 使用近似 Wasserstein 距离（基于 t-digest 的 L1 质心距离）。
type DistributionDriftDetector struct {
	baseline    *TDigest
	threshold   float64
}

// NewDistributionDriftDetector 创建分布漂移检测器。
// threshold: 漂移距离阈值（默认 0.1）。
func NewDistributionDriftDetector(threshold float64) *DistributionDriftDetector {
	if threshold <= 0 {
		threshold = 0.1
	}
	return &DistributionDriftDetector{
		baseline:  nil,
		threshold: threshold,
	}
}

// SetBaseline 设置基线分布（通常在系统稳定时设置）。
func (d *DistributionDriftDetector) SetBaseline(baseline *TDigest) {
	d.baseline = baseline
}

// CompareDistributions 比较当前分布与基线分布的差异。
// 返回 Wasserstein 距离和是否漂移。
func (d *DistributionDriftDetector) CompareDistributions(current *TDigest) DistributionDriftResult {
	if d.baseline == nil || d.baseline.Count() == 0 || current.Count() == 0 {
		return DistributionDriftResult{}
	}

	// 近似 Wasserstein 距离：使用 P25/P50/P75/P95/P99 的 L1 距离
	percentiles := []float64{0.25, 0.50, 0.75, 0.95, 0.99}
	distance := d.wassersteinApprox(d.baseline, current, percentiles)

	currentMean := current.Mean()
	baselineMean := d.baseline.Mean()

	result := DistributionDriftResult{
		Distance:     distance,
		CurrentMean:  currentMean,
		BaselineMean: baselineMean,
		Threshold:    d.threshold,
	}

	if distance > d.threshold {
		result.Drifted = true
		if currentMean > baselineMean {
			result.Direction = DriftDirRight
		} else {
			result.Direction = DriftDirLeft
		}
	}

	return result
}

// wassersteinApprox 使用关键分位数的 L1 距离近似 Wasserstein 距离。
func (d *DistributionDriftDetector) wassersteinApprox(a, b *TDigest, percentiles []float64) float64 {
	var total float64
	for _, p := range percentiles {
		qa := a.Quantile(p)
		qb := b.Quantile(p)
		total += math.Abs(qa - qb)
	}
	return total / float64(len(percentiles))
}
