// Package arch — Guardian 监督原语 (L1 Atom)
//
// 纯函数实现，无副作用。根据架构快照计算：
//   - 架构健康度评分
//   - Guardian 决策（允许/警告/阻止提交）
//   - 架构漂移检测
//   - 熵值趋势分析
package arch

import (
	"math"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Guardian 决策
// ──────────────────────────────────────────────

// GuardianDecision 表示 Guardian 的决策结果。
type GuardianDecision struct {
	Action     string   `json:"action"`      // "allow" | "warn" | "block"
	Reason     string   `json:"reason"`      // 决策原因
	Violations []string `json:"violations"`  // 关键违规项
	Score      float64  `json:"score"`       // 当前健康度 (0.0 ~ 1.0)
	Threshold  float64  `json:"threshold"`   // 决策阈值
	Timestamp  time.Time `json:"timestamp"`
}

// GuardianThresholds Guardian 阈值配置。
type GuardianThresholds struct {
	Allow  float64 // ≥ Allow → 允许（默认 0.90）
	Warn   float64 // ≥ Warn && < Allow → 警告（默认 0.75）
	// < Warn → 阻止
}

// DefaultThresholds 返回默认阈值配置。
func DefaultThresholds() GuardianThresholds {
	return GuardianThresholds{Allow: 0.90, Warn: 0.75}
}

// Decide 根据架构健康度计算 Guardian 决策。
// 纯函数：输入 (healthScore, violations[]) → 输出决策。
func Decide(healthScore float64, violations []Violation, t GuardianThresholds) GuardianDecision {
	blockReasons := make([]string, 0)
	warnReasons := make([]string, 0)

	for _, v := range violations {
		switch v.Severity {
		case SeverityError:
			blockReasons = append(blockReasons,
				string(v.Type)+" @ "+v.File+": "+v.Message)
		case SeverityWarn:
			warnReasons = append(warnReasons,
				string(v.Type)+" @ "+v.File+": "+v.Message)
		}
	}

	action := "allow"
	reason := "架构合规"
	if len(blockReasons) > 0 {
		action = "block"
		n := 3
		if len(blockReasons) < 3 {
			n = len(blockReasons)
		}
		reason = "存在 must-fix 级违规: " + strings.Join(blockReasons[:n], "; ")
	} else if healthScore < t.Warn {
		action = "warn"
		reason = "健康度低于警告阈值"
	} else if healthScore < t.Allow {
		action = "warn"
		reason = "健康度接近阈值"
	} else if len(warnReasons) > 0 {
		action = "warn"
		n := 3
		if len(warnReasons) < 3 {
			n = len(warnReasons)
		}
		reason = "存在警告级违规: " + strings.Join(warnReasons[:n], "; ")
	}

	return GuardianDecision{
		Action:     action,
		Reason:     reason,
		Violations: append(blockReasons, warnReasons...),
		Score:      round2(healthScore),
		Threshold:  t.Allow,
		Timestamp:  time.Now(),
	}
}

// ──────────────────────────────────────────────
// 架构漂移检测
// ──────────────────────────────────────────────

// DriftReport 表示两个架构快照之间的漂移报告。
type DriftReport struct {
	DeltaFiles     int      `json:"delta_files"`      // 文件数变化
	DeltaLines     int      `json:"delta_lines"`      // 行数变化
	DeltaSymbols   int      `json:"delta_symbols"`    // 符号数变化
	FilesAdded     []string `json:"files_added"`
	FilesRemoved   []string `json:"files_removed"`
	FilesModified  []string `json:"files_modified"`
	DriftScore     float64  `json:"drift_score"`      // 综合漂移分数 (0.0 ~ 1.0)
}

// DetectDrift 检测两个架构快照之间的变化。
// 纯函数：输入 (baseline, current) → 输出漂移报告。
func DetectDrift(baseline, current *ArchData) DriftReport {
	if baseline == nil || current == nil {
		return DriftReport{}
	}

	oldFiles := make(map[string]FileInfo, len(baseline.Files))
	for _, f := range baseline.Files {
		oldFiles[f.Name] = f
	}
	newFiles := make(map[string]FileInfo, len(current.Files))
	for _, f := range current.Files {
		newFiles[f.Name] = f
	}

	added := make([]string, 0)
	removed := make([]string, 0)
	modified := make([]string, 0)

	for name := range newFiles {
		if _, ok := oldFiles[name]; !ok {
			added = append(added, name)
		}
	}
	for name, of := range oldFiles {
		nf, ok := newFiles[name]
		if !ok {
			removed = append(removed, name)
		} else if of.Lines != nf.Lines || len(of.Symbols) != len(nf.Symbols) {
			modified = append(modified, name)
		}
	}

	// 漂移分数 = 综合变化量 / 基线规模
	baselineSize := float64(baseline.TotalFiles) + float64(baseline.TotalLines)/100.0
	changeMagnitude := float64(len(added)+len(removed)+len(modified)) +
		math.Abs(float64(current.TotalLines-baseline.TotalLines))/100.0
	drift := 0.0
	if baselineSize > 0 {
		drift = math.Min(1.0, changeMagnitude/baselineSize)
	}

	return DriftReport{
		DeltaFiles:    current.TotalFiles - baseline.TotalFiles,
		DeltaLines:    current.TotalLines - baseline.TotalLines,
		DeltaSymbols:  current.TotalSymbols - baseline.TotalSymbols,
		FilesAdded:    added,
		FilesRemoved:  removed,
		FilesModified: modified,
		DriftScore:    round2(drift),
	}
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
