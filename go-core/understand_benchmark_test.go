package core

import (
	"encoding/json"
	"testing"
)

func BenchmarkParseKnowledgeGraph(b *testing.B) {
	kg := makeTestGraph(100)
	data, _ := json.Marshal(kg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseKnowledgeGraph(data)
	}
}

func BenchmarkValidateKnowledgeGraph(b *testing.B) {
	kg := makeTestGraph(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateKnowledgeGraph(kg)
	}
}

func BenchmarkDiffKnowledgeGraphs(b *testing.B) {
	before := makeTestGraph(500)
	after := makeTestGraph(500)
	after.Nodes[0].Name = "Modified"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiffKnowledgeGraphs(before, after)
	}
}

func BenchmarkSupervisor_ValidateAll(b *testing.B) {
	kg := makeTestGraph(500)
	supervisor := NewGraphSupervisor(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		supervisor.ValidateAll(kg)
	}
}

func BenchmarkObserver_ObserveStructure(b *testing.B) {
	kg := makeTestGraph(500)
	observer := NewGraphObserver(kg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		observer.ObserveStructure()
	}
}

func BenchmarkGraphIndex_Search(b *testing.B) {
	kg := makeTestGraph(500)
	index := NewGraphIndex(kg)
	query := SearchQuery{Keyword: "Node", Limit: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Search(query)
	}
}

func BenchmarkGraphIndex_Build(b *testing.B) {
	kg := makeTestGraph(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGraphIndex(kg)
	}
}

func BenchmarkCountByType(b *testing.B) {
	kg := makeTestGraph(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CountByType(kg)
	}
}
