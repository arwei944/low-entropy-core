// Package core — GraphObserver 图谱观测器 (v0.7.0)
package core

import (
	"context"
	"fmt"
)

// GraphObserver 图谱观测器
type GraphObserver struct {
	kg    *KnowledgeGraph
	index *GraphIndex
}

// NewGraphObserver 创建观测器
func NewGraphObserver(kg *KnowledgeGraph) *GraphObserver {
	o := &GraphObserver{kg: kg}
	if kg != nil {
		o.index = NewGraphIndex(kg)
	}
	return o
}

// SetGraph 更新图谱
func (o *GraphObserver) SetGraph(kg *KnowledgeGraph) {
	o.kg = kg
	o.index = NewGraphIndex(kg)
}

// ObserveStructure 观测结构信息
func (o *GraphObserver) ObserveStructure() *StructureObservation {
	if o.kg == nil {
		return &StructureObservation{}
	}

	obs := &StructureObservation{
		NodeCount:  len(o.kg.Nodes),
		EdgeCount:  len(o.kg.Edges),
		LayerCount: len(o.kg.Layers),
		NodeTypes:  CountByType(o.kg),
		EdgeTypes:  CountByEdgeType(o.kg),
		LayerStats: make(map[string]int),
	}

	// 图密度 = 2 * |E| / (|V| * (|V| - 1))
	if obs.NodeCount > 1 {
		obs.GraphDensity = float64(2*obs.EdgeCount) / float64(obs.NodeCount*(obs.NodeCount-1))
	}

	// 层级统计
	for _, n := range o.kg.Nodes {
		layer := extractLayer(n)
		obs.LayerStats[layer]++
	}

	// 复杂性统计
	for _, n := range o.kg.Nodes {
		switch n.Complexity {
		case "simple":
			obs.Complexity.Simple++
		case "moderate":
			obs.Complexity.Moderate++
		case "complex":
			obs.Complexity.Complex++
		}
	}

	// 孤立节点检测
	referenced := make(map[string]bool)
	for _, e := range o.kg.Edges {
		referenced[e.Source] = true
		referenced[e.Target] = true
	}
	for _, n := range o.kg.Nodes {
		if !referenced[n.ID] && n.Type != NodeTypeFile {
			obs.OrphanNodes++
		}
	}

	return obs
}

// ObserveChanges 观测变更（对比当前图谱与历史快照）
func (o *GraphObserver) ObserveChanges(previous *KnowledgeGraph) *ChangeObservation {
	if o.kg == nil || previous == nil {
		return &ChangeObservation{}
	}

	diff := DiffKnowledgeGraphs(previous, o.kg)
	drifts := DetectLayerDrifts(previous, o.kg)

	return &ChangeObservation{
		SnapshotVersion: previous.Project.GitCommitHash,
		CurrentVersion:  o.kg.Project.GitCommitHash,
		Diff:            diff,
		LayerDrifts:     drifts,
	}
}

// SearchGraph 语义搜索
func (o *GraphObserver) SearchGraph(query SearchQuery) *SearchResult {
	if o.index == nil {
		return &SearchResult{Query: query.Keyword}
	}
	return o.index.Search(query)
}

// StructureObservationInput 结构观测输入
type StructureObservationInput struct{}

// NewObserverStructureStep 创建结构观测 Step
func NewObserverStructureStep(observer *GraphObserver) Step[StructureObservationInput, *StructureObservation] {
	return NewStepFunc("Atom", func(ctx context.Context, _ StructureObservationInput) (*StructureObservation, error) {
		obs := observer.ObserveStructure()
		if obs == nil {
			return nil, fmt.Errorf("observe: no graph data")
		}
		return obs, nil
	})
}

// NewObserverSearchStep 创建搜索 Step
func NewObserverSearchStep(observer *GraphObserver) Step[SearchQuery, *SearchResult] {
	return NewStepFunc("Atom", func(ctx context.Context, query SearchQuery) (*SearchResult, error) {
		result := observer.SearchGraph(query)
		if result == nil {
			return nil, fmt.Errorf("search: no index")
		}
		return result, nil
	})
}
