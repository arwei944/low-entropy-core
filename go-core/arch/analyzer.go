// Package arch — Analyzer Atom (L1)
//
// 从一组 FileInfo 构建 ArchData（架构快照），并检测违规。
// 纯函数：输入 []FileInfo → 输出 *ArchData + []Violation。
//
// 来源:
//   cmd/arch-manager/builder.go  → ArchData 构建
//   cmd/arch-manager/violations.go → 违规检测
//   cmd/arch-manager/primitives.go → 四原语识别
//
// 设计约束: 文件 ≤ 300 行
package arch

import (
	"time"
)

// AnalyzeArchitecture 从解析的文件列表构建完整的 ArchData。
// 聚合层级统计、按类型分组符号、识别四原语。
func AnalyzeArchitecture(files []FileInfo) *ArchData {
	now := time.Now()

	totalLines := 0
	totalSymbols := 0
	symbolKinds := make(map[string]int)

	for _, f := range files {
		totalLines += f.Lines
		totalSymbols += len(f.Symbols)
		for _, s := range f.Symbols {
			symbolKinds[s.Kind]++
		}
	}

	return &ArchData{
		GeneratedAt:  now,
		TotalFiles:   len(files),
		TotalLines:   totalLines,
		TotalSymbols: totalSymbols,
		Files:        files,
		Layers:       buildLayerStats(files),
		SymbolKinds:  symbolKinds,
		Primitives:   detectPrimitives(files),
	}
}

// buildLayerStats 按层级聚合文件统计。
func buildLayerStats(files []FileInfo) []LayerStat {
	// 预置的 8 层顺序
	order := []string{"L0", "L1", "L2", "L3", "L4", "L5", "L6", "L7"}
	statMap := make(map[string]*LayerStat)
	for _, l := range order {
		statMap[l] = &LayerStat{Layer: l}
	}

	for _, f := range files {
		layer := f.Layer
		if statMap[layer] == nil {
			statMap[layer] = &LayerStat{Layer: layer}
		}
		statMap[layer].Name = f.LayerName
		statMap[layer].Files++
		statMap[layer].Lines += f.Lines
		statMap[layer].Symbols += len(f.Symbols)
	}

	result := make([]LayerStat, 0, 8)
	for _, l := range order {
		if statMap[l].Files > 0 {
			statMap[l].Color = layerColor(l)
			result = append(result, *statMap[l])
		}
	}
	return result
}

// detectPrimitives 在所有文件中查找四原语断言。
// 通过文件名前缀 + 关键字简单模式匹配。
func detectPrimitives(files []FileInfo) []PrimitiveInfo {
	result := make([]PrimitiveInfo, 0)

	primPatterns := []struct {
		file   string
		pType  string
	}{
		{"atom.go", "Atom"},
		{"port.go", "Port"},
		{"adapter.go", "Adapter"},
		{"composer.go", "Composer"},
		{"composer_parallel.go", "Composer"},
		{"composer_fanout.go", "Composer"},
		{"composer_stream.go", "Composer"},
	}

	for _, f := range files {
		for _, sym := range f.Symbols {
			for _, p := range primPatterns {
				if f.Name == p.file {
					result = append(result, PrimitiveInfo{
						Name:       sym.Name,
						Type:       p.pType,
						File:       f.Name,
						Package:    f.Package,
						Signature:  sym.Signature,
						IsExported: sym.IsExported,
					})
				}
			}
		}
	}
	return result
}

// ──────────────────────────────────────────────
// 违规检测
// ──────────────────────────────────────────────

// DetectViolations 检查架构数据是否违反 CLAUDE.md 的约束。
// 返回违规列表。
func DetectViolations(data *ArchData) []Violation {
	violations := make([]Violation, 0)

	// 1. 反向依赖：低层文件 import 高层
	violations = append(violations, detectReverseDependencies(data)...)

	// 2. 跨层跳跃：L3 直接依赖 L5+（跳过 L4）
	violations = append(violations, detectLayerJumps(data)...)

	// 3. 文件 > 300 行
	violations = append(violations, detectLongFiles(data)...)

	// 4. L0-L3 中引入第三方库（含点的非标准库路径）
	violations = append(violations, detectThirdParty(data)...)

	// 5. 循环依赖检测
	violations = append(violations, detectCircularDependencies(data)...)

	return violations
}

// detectReverseDependencies 检测反向依赖（低层 import 高层的外部包）。
func detectReverseDependencies(data *ArchData) []Violation {
	var v []Violation
	layerLevel := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "L4": 4, "L5": 5, "L6": 6, "L7": 7}

	for _, f := range data.Files {
		fileLevel := layerLevel[f.Layer]
		for _, dep := range f.DependsOn {
			depLayer := GetLayerInfo(dep + ".go")
			depLevel := layerLevel[depLayer.Layer]
			// 反向依赖：文件所处层 > 其依赖所处层
			// 例如 L2 文件 import L4 的包
			if fileLevel < depLevel {
				v = append(v, Violation{
					Type:       ViolationReverseDependency,
					Severity:   SeverityError,
					File:       f.Name,
					Message:    "反向依赖：" + f.Layer + " 层依赖 " + depLayer.Layer + " 层",
					Detail:     f.Name + " (" + f.Layer + ") → " + dep + " (" + depLayer.Layer + ")",
					Suggestion: "应将依赖逻辑下沉或通过接口抽象",
				})
			}
		}
	}
	return v
}

// detectLayerJumps 检测跨层跳跃（如 L3 直接依赖 L5，跳过 L4）。
func detectLayerJumps(data *ArchData) []Violation {
	var v []Violation
	layerLevel := map[string]int{"L0": 0, "L1": 1, "L2": 2, "L3": 3, "L4": 4, "L5": 5, "L6": 6, "L7": 7}

	for _, f := range data.Files {
		fileLevel := layerLevel[f.Layer]
		for _, dep := range f.DependsOn {
			depLayer := GetLayerInfo(dep + ".go")
			depLevel := layerLevel[depLayer.Layer]
			// 跳跃 ≥ 2 层（如 L3 → L5 跳过了 L4）
			if depLevel-fileLevel >= 2 {
				v = append(v, Violation{
					Type:       ViolationLayerJump,
					Severity:   SeverityWarn,
					File:       f.Name,
					Message:    "跨层跳跃：跳过了中间层",
					Detail:     f.Name + " (" + f.Layer + ") → " + dep + " (" + depLayer.Layer + ")",
					Suggestion: "应通过中间层提供的接口间接访问",
				})
			}
		}
	}
	return v
}

// detectLongFiles 检测超过 300 行的文件。
func detectLongFiles(data *ArchData) []Violation {
	var v []Violation
	for _, f := range data.Files {
		if f.Lines > 300 {
			v = append(v, Violation{
				Type:       ViolationFileTooLong,
				Severity:   SeverityWarn,
				File:       f.Name,
				Message:    "文件超过 300 行",
				Detail:     "实际 " + itoa(f.Lines) + " 行",
				Suggestion: "按功能/层级拆分为多个小文件",
			})
		}
	}
	return v
}

// detectThirdParty 检测 L0-L3 中引入的第三方库。
func detectThirdParty(data *ArchData) []Violation {
	var v []Violation
	lowLevels := map[string]bool{"L0": true, "L1": true, "L2": true, "L3": true}

	for _, f := range data.Files {
		if !lowLevels[f.Layer] {
			continue
		}
		for _, imp := range f.Imports {
			// 含点的路径通常是第三方库（如 github.com/x/y）
			if containsDot(imp) && !containsLowEntropyDomain(imp) {
				v = append(v, Violation{
					Type:       ViolationThirdPartyInLowerLayer,
					Severity:   SeverityWarn,
					File:       f.Name,
					Message:    "L0-L3 引入了第三方库",
					Detail:     "import: " + imp,
					Suggestion: "将第三方依赖提升到 L4 以上，或使用接口隔离",
				})
			}
		}
	}
	return v
}

// detectCircularDependencies 使用简单的传递闭包检测循环。
func detectCircularDependencies(data *ArchData) []Violation {
	var v []Violation
	// 构建简单的依赖图：文件名 → 直接依赖的文件
	graph := make(map[string][]string)
	for _, f := range data.Files {
		graph[f.Name] = f.DependsOn
	}

	// 对每个文件做 DFS，检测是否能回到自己
	for _, f := range data.Files {
		visited := make(map[string]bool)
		if dfsCycle(f.Name, f.Name, graph, visited) {
			v = append(v, Violation{
				Type:       ViolationCircularDependency,
				Severity:   SeverityError,
				File:       f.Name,
				Message:    "检测到循环依赖",
				Detail:     f.Name + " 参与了循环依赖链",
				Suggestion: "重构：提取共享类型或使用依赖倒置",
			})
			break // 每个文件只报告一次
		}
	}
	return v
}

func dfsCycle(start, current string, graph map[string][]string, visited map[string]bool) bool {
	if visited[current] {
		return current == start // 回到起点 → 循环
	}
	visited[current] = true
	for _, dep := range graph[current] {
		if dfsCycle(start, dep, graph, visited) {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────
// 辅助
// ──────────────────────────────────────────────

func layerColor(layer string) string {
	colors := map[string]string{
		"L0": "#7f8ea3", "L1": "#00d4aa", "L2": "#60a5fa", "L3": "#38bdf8",
		"L4": "#ef4444", "L5": "#34d399", "L6": "#f472b6", "L7": "#f59e0b",
	}
	if c, ok := colors[layer]; ok {
		return c
	}
	return "#cccccc"
}

// itoa 不依赖 strconv 的简单整数转字符串（0-9999 足够）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte(n%10) + '0'}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

func containsLowEntropyDomain(s string) bool {
	return false // 占位：如有内部域名，在此添加
}
