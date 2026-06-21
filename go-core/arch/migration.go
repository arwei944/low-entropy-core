// Package arch — Migration + Runtime + Version 原语 (L1)
package arch

import (
	"time"
)

// ──────────────────────────────────────────────
// Migration
// ──────────────────────────────────────────────

type MigrationStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

type MigrationPlan struct {
	ID        string          `json:"id"`
	Source    string          `json:"source"`
	Target    string          `json:"target"`
	Steps     []MigrationStep `json:"steps"`
	Status    string          `json:"status"`
	Progress  int             `json:"progress"`
	CreatedAt time.Time       `json:"created_at"`
}

type MigrationReport struct {
	Plan         MigrationPlan `json:"plan"`
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
	Duration     time.Duration `json:"duration_ms"`
	ChangedFiles []string      `json:"changed_files,omitempty"`
}

func AnalyzeMigration(baseline, current *ArchData) MigrationPlan {
	drift := DetectDrift(baseline, current)
	steps := []MigrationStep{
		{Name: "analyze", Description: "分析架构差异", Status: "done"},
		{Name: "validate", Description: "验证目标架构合规性", Status: "pending"},
		{Name: "execute", Description: "执行变更", Status: "pending"},
		{Name: "verify", Description: "验证结果", Status: "pending"},
	}
	if drift.DeltaFiles != 0 || drift.DeltaLines != 0 {
		steps[0].Status = "done"
	}
	return MigrationPlan{
		ID:        "migration-" + time.Now().Format("20060102-150405"),
		Source:    "baseline",
		Target:    "current",
		Steps:     steps,
		Status:    "analyzed",
		Progress:  25,
		CreatedAt: time.Now(),
	}
}

// ──────────────────────────────────────────────
// Runtime
// ──────────────────────────────────────────────

type RuntimeMetrics struct {
	ThroughputPerSec int64         `json:"throughput_per_sec"`
	ErrorCount       int64         `json:"error_count"`
	AvgLatencyMs     float64       `json:"avg_latency_ms"`
	P99LatencyMs     float64       `json:"p99_latency_ms"`
	SamplingRate     float64       `json:"sampling_rate"`
	Uptime           time.Duration `json:"uptime_ms"`
	Timestamp        time.Time     `json:"timestamp"`
}

func ComputeRuntimeScore(m RuntimeMetrics) float64 {
	score := 1.0
	if m.ErrorCount > 100 {
		score -= 0.2
	}
	if m.AvgLatencyMs > 1000 {
		score -= 0.2
	}
	if m.P99LatencyMs > 5000 {
		score -= 0.3
	}
	if score < 0 {
		score = 0
	}
	return round2(score)
}

// ──────────────────────────────────────────────
// Version
// ──────────────────────────────────────────────

type VersionSnap struct {
	Version   string    `json:"version"`
	ArchData  *ArchData `json:"arch_data"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	ChangeLog []string  `json:"changelog,omitempty"`
}

func ComputeNextVersion(currentVersion string, drift DriftReport) string {
	major, minor, patch := 1, 0, 0
	if currentVersion != "" {
		n, _ := fmtSscanf(currentVersion)
		if n[0] > 0 {
			major = n[0]
		}
		if n[1] > 0 {
			minor = n[1]
		}
		if n[2] > 0 {
			patch = n[2]
		}
	}
	switch {
	case len(drift.FilesRemoved) > 3 || drift.DriftScore > 0.5:
		major++
		minor = 0
		patch = 0
	case drift.DeltaFiles != 0 || drift.DriftScore > 0.1:
		minor++
		patch = 0
	default:
		patch++
	}
	return fmtSprintf(major, minor, patch)
}

func fmtSscanf(s string) ([3]int, bool) {
	var result [3]int
	idx, val := 0, 0
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= '0' && c <= '9' {
			val = val*10 + int(c-'0')
		} else if c == '.' {
			if idx < 3 {
				result[idx] = val
			}
			idx++
			val = 0
		}
	}
	if idx < 3 {
		result[idx] = val
	}
	return result, true
}

func fmtSprintf(major, minor, patch int) string {
	return itoa(major) + "." + itoa(minor) + "." + itoa(patch)
}