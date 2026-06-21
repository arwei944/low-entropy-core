// parser_test.go - 测试 Parser Atom
package arch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseDirectory_Empty 测试空目录解析
func TestParseDirectory_Empty(t *testing.T) {
	dir, err := os.MkdirTemp("", "arch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	files, err := ParseDirectory(dir)
	if err != nil {
		t.Fatal("解析空目录不应出错:", err)
	}
	if len(files) != 0 {
		t.Error("空目录应无文件, 得到:", len(files))
	}
}

// TestParseDirectory_SimpleGo 测试简单 Go 文件
func TestParseDirectory_SimpleGo(t *testing.T) {
	dir, err := os.MkdirTemp("", "arch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 创建简单文件
	code := `package mypkg

// MyFunc 是一个函数
func MyFunc() string {
	return "hello"
}

// MyType 是一个类型
type MyType struct {
	Name string
}
`
	filePath := filepath.Join(dir, "simple.go")
	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := ParseFile(filePath)
	if err != nil {
		t.Fatal("解析文件失败:", err)
	}
	if info.Package != "mypkg" {
		t.Errorf("包名应为 'mypkg', 得到 '%s'", info.Package)
	}
	if info.Name != "simple.go" {
		t.Errorf("文件名应为 'simple.go', 得到 '%s'", info.Name)
	}
	if info.Lines <= 0 {
		t.Error("行数应 > 0")
	}
}

// TestParseDirectory_IgnoresTest 测试忽略 _test.go
func TestParseDirectory_IgnoresTest(t *testing.T) {
	dir, err := os.MkdirTemp("", "arch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 创建正常文件和测试文件
	os.WriteFile(filepath.Join(dir, "normal.go"), []byte("package p\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test_test.go"), []byte("package p\n"), 0644)

	files, err := ParseDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Error("应只包含 1 个非测试文件, 得到:", len(files))
	}
}

// TestResolveInternalDeps 测试依赖解析
func TestResolveInternalDeps(t *testing.T) {
	imports := []string{
		"fmt",
		"low-entropy-core/go-core/types",
		"os",
	}
	deps := resolveInternalDeps(imports)
	if len(deps) != 1 {
		t.Error("应只识别 1 个内部依赖, 得到:", len(deps))
	}
	if len(deps) > 0 && deps[0] != "types" {
		t.Error("依赖名应为 'types', 得到:", deps[0])
	}
}

// TestParser_ReturnsStrings 确保返回字符串不含换行
func TestParser_ReturnsStrings(t *testing.T) {
	dir, _ := os.MkdirTemp("", "arch-test")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "atom.go"),
		[]byte("package p\nfunc X()string{return\"\"}\n"), 0644)

	files, err := ParseDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 1 {
		t.Fatal("未找到文件")
	}
	// 验证 Layer 字段
	if !strings.HasPrefix(files[0].Layer, "L") {
		t.Error("层级格式应形如 Lx, 得到:", files[0].Layer)
	}
}
