//go:build lecore_tier1 || lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTierDriftMonitor(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL1)
	if m == nil {
		t.Fatal("NewTierDriftMonitor returned nil")
	}
	if m.currentTier != TierL1 {
		t.Errorf("expected TierL1, got %s", m.currentTier)
	}
}

func TestTierDriftMonitor_Check(t *testing.T) {
	dir := t.TempDir()
	// 创建一些文件
	for i := 0; i < 5; i++ {
		f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("file_%d.go", i)))
		f.Close()
	}

	m := NewTierDriftMonitor(dir, TierL0)
	report := m.Check()

	if report.DetectedTier < TierL0 {
		t.Errorf("unexpected detected tier: %s", report.DetectedTier)
	}
	if len(report.History) != 1 {
		t.Errorf("expected 1 history point, got %d", len(report.History))
	}
}

func TestTierDriftMonitor_MultipleChecks(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)

	// 执行多次检查
	for i := 0; i < 5; i++ {
		// 每次添加一些文件模拟增长
		for j := 0; j < 10; j++ {
			f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("f_%d_%d.go", i, j)))
			f.Close()
		}
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
		m.Check()
	}

	history := m.History()
	if len(history) != 5 {
		t.Errorf("expected 5 history points, got %d", len(history))
	}

	// 验证历史记录按时间排序
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Errorf("history not sorted by time at index %d", i)
		}
	}
}

func TestTierDriftMonitor_PredictNextTier(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)

	// 添加文件模拟增长趋势
	for i := 0; i < 5; i++ {
		for j := 0; j < 20; j++ {
			f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("f_%d_%d.go", i, j)))
			f.Close()
		}
		time.Sleep(10 * time.Millisecond)
		m.Check()
	}

	pred := m.PredictNextTier()
	if pred.Confidence <= 0 {
		t.Error("expected confidence > 0 with 5 data points")
	}
	if pred.GrowthRate <= 0 {
		t.Error("expected positive growth rate")
	}
}

func TestTierDriftMonitor_PredictInsufficientData(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)

	// 只有 1 个数据点
	_ = m.Check()

	pred := m.PredictNextTier()
	if pred.Confidence != 0 {
		t.Errorf("expected 0 confidence with 1 data point, got %f", pred.Confidence)
	}
}

func TestTierDriftMonitor_SetCurrentTier(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)
	m.SetCurrentTier(TierL2)

	if m.currentTier != TierL2 {
		t.Errorf("expected TierL2, got %s", m.currentTier)
	}
}

func TestTierDriftMonitor_HistoryLimit(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)

	// 超过 maxHistory (100) 次检查
	for i := 0; i < 150; i++ {
		m.Check()
	}

	history := m.History()
	if len(history) > 100 {
		t.Errorf("history should be capped at 100, got %d", len(history))
	}
}

func TestTierDriftMonitor_Concurrent(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				m.Check()
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	history := m.History()
	if len(history) == 0 {
		t.Error("expected history after concurrent checks")
	}
}

func TestTierDriftReport_Fields(t *testing.T) {
	dir := t.TempDir()
	m := NewTierDriftMonitor(dir, TierL0)
	report := m.Check()

	if report.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
	if report.CurrentTier != TierL0 {
		t.Errorf("expected TierL0, got %s", report.CurrentTier)
	}
	if report.Suggestion == "" {
		t.Error("suggestion should not be empty")
	}
}