// tier_check.go — 编译时 tier 漂移检测
// This file is part of the kernel (always compiled, no build tags).
// TierCheck compares the current Build Tag tier with AutoDetect results
// and reports whether the project has outgrown its current tier.

package core

import "fmt"

// TierCheckResult 表示一次 tier 漂移检测的结果。
type TierCheckResult struct {
	Status       string          // "ok" | "drift_detected" | "oversized"
	CurrentTier  ComplexityTier
	DetectedTier ComplexityTier
	DriftLevel   int    // 正数 = 需要升级的层级数，0 = 匹配，负数 = 当前 tier 高于检测结果
	Suggestion   string
}

// TierCheck 比较当前 Build Tag 指定的 tier 与 AutoDetect 扫描结果。
// 如果检测到项目规模已超出当前 tier，返回 DriftLevel > 0。
func TierCheck(root string, currentTier ComplexityTier) TierCheckResult {
	detected := AutoDetectWithBoost(root)
	drift := int(detected) - int(currentTier)

	result := TierCheckResult{
		CurrentTier:  currentTier,
		DetectedTier: detected,
		DriftLevel:   drift,
	}

	switch {
	case drift <= 0:
		result.Status = "ok"
		result.Suggestion = fmt.Sprintf("Current tier %s matches project scale.", currentTier)
	case drift == 1:
		result.Status = "drift_detected"
		result.Suggestion = fmt.Sprintf(
			"Project has outgrown tier %s. Consider migrating to %s. Drift: +%d level.",
			currentTier, detected, drift,
		)
	default:
		result.Status = "oversized"
		result.Suggestion = fmt.Sprintf(
			"Project is severely oversized for tier %s. Must migrate to %s. Drift: +%d levels.",
			currentTier, detected, drift,
		)
	}

	return result
}

// IsOk 返回 true 表示当前 tier 匹配项目规模。
func (r TierCheckResult) IsOk() bool {
	return r.Status == "ok"
}

// NeedsMigration 返回 true 表示需要迁移到更高 tier。
func (r TierCheckResult) NeedsMigration() bool {
	return r.DriftLevel > 0
}

// IsCritical 返回 true 表示漂移严重（2+ 级），必须迁移。
func (r TierCheckResult) IsCritical() bool {
	return r.DriftLevel >= 2
}