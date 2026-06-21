package arch

import (
	"testing"
)

// TestSymbolZeroValue 验证 Symbol 零值行为。
func TestSymbolZeroValue(t *testing.T) {
	var s Symbol
	if s.IsExported {
		t.Error("零值 Symbol IsExported 应为 false")
	}
	if s.Name != "" {
		t.Error("零值 Symbol Name 应为空")
	}
}

// TestFileInfoZeroValue 验证 FileInfo 零值行为。
func TestFileInfoZeroValue(t *testing.T) {
	var f FileInfo
	if f.Lines != 0 {
		t.Error("零值 FileInfo Lines 应为 0")
	}
	if f.Layer != "" {
		t.Error("零值 FileInfo Layer 应为空")
	}
}

// TestGetLayerInfo 验证层级分类。
func TestGetLayerInfo(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"errors.go", "L0"},
		{"atom.go", "L1"},
		{"composer.go", "L1"},
		{"patterns_circuit.go", "L2"},
		{"patterns_distributed.go", "L3"},
		{"guardian_decision.go", "L4"},
		{"observation_pipeline.go", "L5"},
		{"eventstore.go", "L6"},
		{"config_hotreload.go", "L7"},
		{"unknown_file.go", "L7"}, // 默认 L7
	}

	for _, c := range cases {
		info := GetLayerInfo(c.filename)
		if info.Layer != c.want {
			t.Errorf("GetLayerInfo(%q).Layer = %q, 期望 %q", c.filename, info.Layer, c.want)
		}
	}
}

// TestComputeGrade 验证健康度等级映射。
func TestComputeGrade(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.00, "A"},
		{0.95, "A"},
		{0.90, "A"},
		{0.89, "B"},
		{0.75, "B"},
		{0.74, "C"},
		{0.60, "C"},
		{0.59, "D"},
		{0.00, "D"},
	}

	for _, c := range cases {
		got := ComputeGrade(c.score)
		if got != c.want {
			t.Errorf("ComputeGrade(%v) = %q, 期望 %q", c.score, got, c.want)
		}
	}
}

// TestViolationTypeConstants 验证违规类型常量不为空。
func TestViolationTypeConstants(t *testing.T) {
	types := []ViolationType{
		ViolationReverseDependency,
		ViolationLayerJump,
		ViolationCircularDependency,
		ViolationMissingPrimitive,
		ViolationFileTooLong,
		ViolationThirdPartyInLowerLayer,
		ViolationRawPrintln,
	}
	for _, vt := range types {
		if vt == "" {
			t.Error("ViolationType 不应为空字符串")
		}
	}
}

// TestArchDataHasZeroViolations 验证 ArchData 零值行为。
func TestArchDataHasZeroViolations(t *testing.T) {
	var d ArchData
	if d.TotalFiles != 0 || d.TotalLines != 0 {
		t.Error("零值 ArchData 的统计字段应为 0")
	}
	if len(d.Files) != 0 || len(d.Layers) != 0 {
		t.Error("零值 ArchData 的切片字段应为空")
	}
}
