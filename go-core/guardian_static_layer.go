//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// ──────────────────────────────────────────────
// SECTION 4: 检查 2 — 外部依赖
// ──────────────────────────────────────────────

// allowedImportPaths 定义了允许的 import 路径。
var allowedImportPaths = map[string]bool{
	"go-core": true,
	"fmt":     true, "time": true, "context": true, "sync": true,
	"strings": true, "strconv": true, "encoding": true, "encoding/json": true,
	"errors": true, "math": true, "crypto": true, "crypto/sha256": true,
	"crypto/sha512": true, "crypto/md5": true, "encoding/hex": true,
	"encoding/base64": true, "io": true, "os": true, "path": true,
	"path/filepath": true, "sort": true, "bytes": true, "bufio": true,
	"log": true, "flag": true, "net": true, "net/http": true,
	"database/sql": true, "database/sql/driver": true,
	"reflect": true, "regexp": true, "unicode": true, "unicode/utf8": true,
	"container": true, "container/heap": true, "container/list": true,
	"hash": true, "hash/fnv": true, "image": true,
	"runtime": true, "runtime/debug": true, "sync/atomic": true,
	"testing": true, "compress": true,
}

// checkExternalDeps 检查 import 是否引入了非白名单包。
// 只允许 go-core 和标准库（白名单中的路径）。
func (g *StaticGuardPort) checkExternalDeps(fset *token.FileSet, file *ast.File) []Violation {
	var violations []Violation

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// 检查是否为允许的路径
		if isAllowedImport(importPath) {
			continue
		}

		line := fset.Position(imp.Pos()).Line
		violations = append(violations, Violation{
			Rule:       "external_dependency",
			Severity:   "error",
			Location:   fmt.Sprintf("line %d: import \"%s\"", line, importPath),
			Detail:     fmt.Sprintf("引入了外部依赖 %s，只允许 import go-core 和标准库", importPath),
			Suggestion: fmt.Sprintf("移除 %s 的 import，将相关功能封装为 Adapter 原语（通过 go-core 的 Adapter 接口隔离外部依赖）", importPath),
		})
	}

	return violations
}

// isAllowedImport 检查 import 路径是否在白名单中。
func isAllowedImport(path string) bool {
	if allowedImportPaths[path] {
		return true
	}
	// 检查前缀匹配（如 "encoding/json" 匹配 "encoding" 前缀）
	for allowed := range allowedImportPaths {
		if strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}
	// 内部包：go-core 的子包
	if strings.HasPrefix(path, "go-core/") {
		return true
	}
	return false
}

// ──────────────────────────────────────────────
// SECTION 5: 检查 3 — 层级合规
// ──────────────────────────────────────────────

// validLayers 定义了合法的层级。
var validLayers = map[string]bool{
	"L1": true, "L2": true, "L3": true, "L4": true,
	"L5": true, "L6": true, "L7": true,
}

// primitiveLayerRules 定义了每种原语类型的合法层级。
var primitiveLayerRules = map[string][]string{
	"Atom":     {"L1", "L2", "L3", "L4", "L5", "L6", "L7"}, // Atom 可在任意层
	"Port":     {"L1", "L2", "L3"},                            // Port 在边界层
	"Adapter":  {"L5", "L6", "L7"},                            // Adapter 在基础设施层
	"Composer": {"L1", "L2", "L3", "L4", "L5", "L6", "L7"},   // Composer 可在任意层
}

// checkLayerCompliance 检查 Manifest 声明的层级是否合规。
func (g *StaticGuardPort) checkLayerCompliance(manifest []PrimitiveManifest) []Violation {
	var violations []Violation

	for _, m := range manifest {
		// 检查层级是否合法
		if !validLayers[m.Layer] {
			violations = append(violations, Violation{
				Rule:       "invalid_layer",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("原语 %s 声明了非法层级 %s，合法层级为 L1-L7", m.Name, m.Layer),
				Suggestion: fmt.Sprintf("将 %s 的 layer 改为 L1-L7 之间的合法层级", m.Name),
			})
			continue
		}

		// 检查原语类型与层级是否匹配
		allowedLayers, ok := primitiveLayerRules[m.PrimitiveType]
		if !ok {
			continue // 已在 Manifest.Validate() 中检查
		}

		layerAllowed := false
		for _, l := range allowedLayers {
			if l == m.Layer {
				layerAllowed = true
				break
			}
		}

		if !layerAllowed {
			suggestedLayer := allowedLayers[0]
			if m.PrimitiveType == "Adapter" {
				suggestedLayer = "L7"
			} else if m.PrimitiveType == "Port" {
				suggestedLayer = "L1"
			}
			violations = append(violations, Violation{
				Rule:       "layer_mismatch",
				Severity:   "error",
				Location:   fmt.Sprintf("Manifest[%s]", m.Name),
				Detail:     fmt.Sprintf("原语类型 %s 不能在 %s 层，允许的层级: %s", m.PrimitiveType, m.Layer, strings.Join(allowedLayers, ", ")),
				Suggestion: fmt.Sprintf("将 %s 的 layer 从 %s 改为 %s", m.Name, m.Layer, suggestedLayer),
			})
		}
	}

	// 检查依赖关系：是否越层调用
	for _, m := range manifest {
		for _, depName := range m.Dependencies {
			dep := findManifest(manifest, depName)
			if dep == nil {
				continue
			}
			// 检查是否从低层调用高层（越层）
			if layerOrder(m.Layer) > layerOrder(dep.Layer) {
				// 当前层高于依赖层，允许（高层依赖低层）
				continue
			}
			// 如果跨层调用距离过大（> 3 层），warn
			if layerOrder(dep.Layer)-layerOrder(m.Layer) > 3 {
				violations = append(violations, Violation{
					Rule:       "layer_skip",
					Severity:   "warn",
					Location:   fmt.Sprintf("Manifest[%s].dependencies[%s]", m.Name, depName),
					Detail:     fmt.Sprintf("原语 %s(%s) 依赖 %s(%s)，跨层距离过大（%d 层）", m.Name, m.Layer, depName, dep.Layer, layerOrder(dep.Layer)-layerOrder(m.Layer)),
					Suggestion: "考虑引入中间层原语来减少跨层距离",
				})
			}
		}
	}

	return violations
}

// layerOrder 返回层级的数值顺序。
func layerOrder(layer string) int {
	order := map[string]int{
		"L1": 1, "L2": 2, "L3": 3, "L4": 4,
		"L5": 5, "L6": 6, "L7": 7,
	}
	return order[layer]
}

// findManifest 在 Manifest 列表中查找指定名称的原语。
func findManifest(manifest []PrimitiveManifest, name string) *PrimitiveManifest {
	for i := range manifest {
		if manifest[i].Name == name {
			return &manifest[i]
		}
	}
	return nil
}
