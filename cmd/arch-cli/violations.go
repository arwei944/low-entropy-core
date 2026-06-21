package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleViolations 检测架构违规
func handleViolations(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, "no data", http.StatusServiceUnavailable)
		return
	}

	violations := detectViolations(archData)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(violations)
}

func detectViolations(data *ArchData) []Violation {
	violations := make([]Violation, 0)

	// 构建层级索引
	layerOrder := map[string]int{}
	for i, l := range data.Layers {
		layerOrder[l.Layer] = i
	}

	// 检测层级反向依赖（仅当跨层数 > 3 时报告，同包 Go 文件间的自然引用不算违规）
	for _, f := range data.Files {
		fileLayer := layerOrder[f.Layer]
		for _, dep := range f.DependsOn {
			// 查找依赖文件所属层级
			for _, df := range data.Files {
				if df.Name == dep || df.Name == dep+".go" {
					depLayer := layerOrder[df.Layer]
					gap := depLayer - fileLayer
					if gap > 7 {
						violations = append(violations, Violation{
							Severity: "warning",
							Type:     "layer_violation",
							File:     f.Name,
							Message:  fmt.Sprintf("%s (%s) 依赖了上层文件 %s (%s)", f.Name, f.Layer, df.Name, df.Layer),
							Detail:   fmt.Sprintf("架构约束：允许下层依赖上层（同包 Go 文件自然引用），但跨层数过多（%d 层）需关注", gap),
						})
					}
				}
			}
		}
	}

	// 检测循环依赖（仅跨包循环才算违规，同包内文件天然互相引用）
	// 构建文件→包名映射
	filePkg := make(map[string]string)
	for _, f := range data.Files {
		filePkg[f.Name] = f.Package
	}
	depGraph := make(map[string]map[string]bool)
	for _, f := range data.Files {
		depGraph[f.Name] = make(map[string]bool)
		for _, dep := range f.DependsOn {
			depGraph[f.Name][dep] = true
		}
	}
	cycles := detectCycles(depGraph)
	for _, cycle := range cycles {
		// cycle 格式包含闭合节点，所以 4 个元素 = 3 个文件
		if len(cycle) <= 4 {
			continue
		}
		// 检查是否所有文件都在同一个 Go package 中
		// 同包内文件共享命名空间，互相引用不构成循环依赖
		allSamePkg := true
		firstPkg := filePkg[cycle[0]]
		for _, f := range cycle {
			if filePkg[f] != firstPkg {
				allSamePkg = false
				break
			}
		}
		if allSamePkg {
			continue
		}
		violations = append(violations, Violation{
			Severity: "error",
			Type:     "cyclic_dependency",
			File:     cycle[0],
			Message:  fmt.Sprintf("检测到跨包循环依赖: %s", strings.Join(cycle, " -> ")),
			Detail:   "跨包循环依赖会导致编译错误和运行时死锁",
		})
	}

	// 检测超大文件 (>800行)
	for _, f := range data.Files {
		if f.Lines > 800 {
			violations = append(violations, Violation{
				Severity: "info",
				Type:     "large_file",
				File:     f.Name,
				Message:  fmt.Sprintf("文件 %s 过大 (%d 行)，建议拆分", f.Name, f.Lines),
				Detail:   fmt.Sprintf("该文件包含 %d 个符号，平均每行 %.1f 个符号", len(f.Symbols), float64(len(f.Symbols))/float64(f.Lines)),
			})
		}
	}

	// 检测无符号文件（排除入口文件、build-tag桩文件、小文件）
	for _, f := range data.Files {
		if len(f.Symbols) == 0 && f.Lines > 10 {
			// 跳过 main.go 入口文件（Go 规范: main 函数不导出）
			if f.Name == "main.go" {
				continue
			}
			// 跳过 build-tag 回退桩文件
			if strings.Contains(f.Name, "_fallback") {
				continue
			}
			// 跳过小于 30 行的 build-tag 条件编译文件
			if f.Lines < 30 && strings.Contains(f.Name, "_") {
				continue
			}
			violations = append(violations, Violation{
				Severity: "info",
				Type:     "empty_file",
				File:     f.Name,
				Message:  fmt.Sprintf("文件 %s (%d 行) 无导出符号", f.Name, f.Lines),
				Detail:   "该文件可能仅包含内部实现或已被废弃",
			})
		}
	}

	return violations
}

func detectCycles(graph map[string]map[string]bool) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for dep := range graph[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				// 找到环
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := append([]string{}, path[cycleStart:]...)
					cycle = append(cycle, dep)
					cycles = append(cycles, cycle)
				}
				return true
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return false
	}

	for node := range graph {
		if !visited[node] {
			path = []string{}
			dfs(node)
		}
	}

	return cycles
}
