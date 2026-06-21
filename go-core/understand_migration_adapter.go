// Package core — Understand-Anything 迁移层 (v0.7.0)
//
// T-04: UnderstandMigrationAdapter — 迁移前/后分析
// T-05: DiffKnowledgeGraphs — 纯 Atom 函数，图谱差异对比
// T-06: NewMigrationStep — Step 包装
//
// 约束遵循:
//   C3: 原语纯度 — DiffKnowledgeGraphs 是纯函数
//   C4: Port-Adapter — 文件 I/O 在 Adapter 中
//   C5: Step 统一 — 迁移流程包装为 Step[MigrationRequest, MigrationReport]

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ============================================================================
// T-04: UnderstandMigrationAdapter
// ============================================================================

// UnderstandMigrationAdapter 迁移层适配器
type UnderstandMigrationAdapter struct {
	uaAdapter *UnderstandAdapter
	baselines map[string]*MigrationBaseline // 内存缓存
}

// NewUnderstandMigrationAdapter 创建迁移适配器
func NewUnderstandMigrationAdapter(uaAdapter *UnderstandAdapter) *UnderstandMigrationAdapter {
	return &UnderstandMigrationAdapter{
		uaAdapter: uaAdapter,
		baselines: make(map[string]*MigrationBaseline),
	}
}

// CreateBaseline 从当前图谱创建迁移基线
func (m *UnderstandMigrationAdapter) CreateBaseline(ctx context.Context, version string) (*MigrationBaseline, error) {
	kg, err := m.uaAdapter.LoadGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration: create baseline: %w", err)
	}

	baseline := &MigrationBaseline{
		Version:   version,
		Timestamp: time.Now(),
		FileCount: len(kg.Nodes),
		NodeCount: len(kg.Nodes),
		EdgeCount: len(kg.Edges),
		LayerStats: make(map[string]int),
		NodeTypes:  CountByType(kg),
		EdgeTypes:  CountByEdgeType(kg),
		FileHashes: make(map[string]string),
	}

	// 统计层级
	for _, n := range kg.Nodes {
		layer := extractLayer(n)
		baseline.LayerStats[layer]++
	}

	m.baselines[version] = baseline
	return baseline, nil
}

// CompareWithBaseline 对比当前图谱与基线
func (m *UnderstandMigrationAdapter) CompareWithBaseline(ctx context.Context, sourceVersion string) (*MigrationReport, error) {
	baseline, ok := m.baselines[sourceVersion]
	if !ok {
		// 尝试从文件加载
		loaded, err := m.loadBaselineFromFile(sourceVersion)
		if err != nil {
			return nil, fmt.Errorf("migration: baseline %s not found: %w", sourceVersion, err)
		}
		baseline = loaded
	}

	kg, err := m.uaAdapter.LoadGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration: load current graph: %w", err)
	}

	// 由于我们只有基线统计信息，无法做精确差异对比
	// 此处返回基于统计的迁移报告
	report := &MigrationReport{
		SourceVersion: sourceVersion,
		TargetVersion: "current",
		Timestamp:     time.Now(),
		Success:       true,
		Warnings:      ValidateKnowledgeGraph(kg),
	}

	// 层级漂移检测
	currentLayerStats := make(map[string]int)
	for _, n := range kg.Nodes {
		layer := extractLayer(n)
		currentLayerStats[layer]++
	}
	for layer, before := range baseline.LayerStats {
		after := currentLayerStats[layer]
		delta := after - before
		severity := "info"
		if delta > 5 || delta < -5 {
			severity = "warn"
		}
		if delta > 20 || delta < -20 {
			severity = "critical"
		}
		report.LayerDrifts = append(report.LayerDrifts, LayerDrift{
			Layer:       layer,
			NodesBefore: before,
			NodesAfter:  after,
			Delta:       delta,
			Severity:    severity,
		})
	}

	return report, nil
}

// loadBaselineFromFile 从文件加载基线
func (m *UnderstandMigrationAdapter) loadBaselineFromFile(version string) (*MigrationBaseline, error) {
	path := filepath.Join(m.uaAdapter.config.OutputDir, "baselines", version+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var baseline MigrationBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}
	m.baselines[version] = &baseline
	return &baseline, nil
}

// SaveBaseline 保存基线到文件
func (m *UnderstandMigrationAdapter) SaveBaseline(version string) error {
	baseline, ok := m.baselines[version]
	if !ok {
		return fmt.Errorf("migration: baseline %s not found in memory", version)
	}

	dir := filepath.Join(m.uaAdapter.config.OutputDir, "baselines")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, version+".json")
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ============================================================================
// T-06: Step 包装
// ============================================================================

// NewMigrationStep 创建迁移检测 Step
func NewMigrationStep(adapter *UnderstandMigrationAdapter) Step[MigrationRequest, MigrationReport] {
	return NewStepFunc("Adapter", func(ctx context.Context, req MigrationRequest) (MigrationReport, error) {
		// 创建基线
		if _, err := adapter.CreateBaseline(ctx, req.SourceVersion); err != nil {
			return MigrationReport{
				SourceVersion: req.SourceVersion,
				Success:       false,
				Errors:        []string{err.Error()},
			}, err
		}

		// 对比分析
		report, err := adapter.CompareWithBaseline(ctx, req.SourceVersion)
		if err != nil {
			return MigrationReport{
				SourceVersion: req.SourceVersion,
				Success:       false,
				Errors:        []string{err.Error()},
			}, err
		}

		return *report, nil
	})
}
