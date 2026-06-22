package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 架构数据构建
// ============================================================================

// buildArchData 扫描目录并构建完整架构数据。
//
// 依赖图构建分为三阶段，确保拓扑图中有真实有意义的依赖边：
//
//   阶段 A: buildImportIndex —— 从文件路径反向构建「包路径 → 文件名」索引
//   阶段 B: import-based    —— 对每个文件，解析其 imports → 定位目标文件名
//   阶段 C: symbol-based    —— 保留原有 AST 函数调用名 → 定义文件 的匹配逻辑（补充）
func buildArchData(dir string) (*ArchData, error) {
	data := &ArchData{
		GeneratedAt: time.Now(),
		Files:       make([]FileInfo, 0),
		SymbolKinds: make(map[string]int),
	}

	type fileResult struct {
		info FileInfo
		err  error
	}

	results := make(chan fileResult, 100)
	var wg sync.WaitGroup

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// 跳过 cmd 目录
		if strings.Contains(path, string(filepath.Separator)+"cmd"+string(filepath.Separator)) {
			return nil
		}

		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			info, err := parseFile(p)
			results <- fileResult{info, err}
		}(path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			log.Printf("WARN: %v", r.err)
			continue
		}
		data.Files = append(data.Files, r.info)
	}

	// 排序文件（稳定输出）
	sort.Slice(data.Files, func(i, j int) bool {
		return data.Files[i].Name < data.Files[j].Name
	})

	// ============================================================
	// 阶段 A：构建 import → 文件 的反向索引
	// ============================================================
	pkgPathIndex, pkgBaseIndex := buildImportIndex(data.Files)

	// 构建符号→文件索引（阶段 C 使用）：记录每个导出符号定义在哪个文件
	symbolToFile := make(map[string]string)
	for _, f := range data.Files {
		for _, s := range f.Symbols {
			if _, exists := symbolToFile[s.Name]; !exists {
				symbolToFile[s.Name] = f.Name
			}
		}
	}

	// ============================================================
	// 阶段 B：基于 import 推断跨文件依赖
	// 阶段 C：保留原有 symbol-based 依赖分析（作为补充）
	// ============================================================
	for i := range data.Files {
		seen := make(map[string]bool)
		seen[data.Files[i].Name] = true
		deps := make([]string, 0)

		// ---- 阶段 B: import-based 依赖 ----
		for _, imp := range data.Files[i].Imports {
			// 只关注内部包
			if !strings.Contains(imp, "low-entropy-core/") {
				continue
			}
			// 先在完整包路径索引查找
			if names, ok := pkgPathIndex[imp]; ok {
				for _, n := range names {
					if !seen[n] {
						seen[n] = true
						deps = append(deps, n)
					}
				}
				continue
			}
			// 退化：去 low-entropy-core 前缀的基础路径查找
			base := strings.TrimPrefix(imp, "low-entropy-core/")
			if names, ok := pkgBaseIndex[base]; ok {
				for _, n := range names {
					if !seen[n] {
						seen[n] = true
						deps = append(deps, n)
					}
				}
			}
		}

		// ---- 阶段 C: symbol-based 依赖（补充）----
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, data.Files[i].Path, nil, 0)
		if err == nil {
			refs := collectCalledFunctions(astFile)
			for _, ref := range refs {
				// 只关注导出符号（首字母大写）
				if len(ref) == 0 || ref[0] < 'A' || ref[0] > 'Z' {
					continue
				}
				if defFile, ok := symbolToFile[ref]; ok && !seen[defFile] {
					seen[defFile] = true
					deps = append(deps, defFile)
				}
			}
		}

		sort.Strings(deps)
		data.Files[i].DependsOn = deps
	}

	// ============================================================
	// 阶段 D：从 DependsOn 反向计算 DependedBy
	// ============================================================
	dependedBy := make(map[string]map[string]bool)
	for _, f := range data.Files {
		for _, dep := range f.DependsOn {
			if dependedBy[dep] == nil {
				dependedBy[dep] = make(map[string]bool)
			}
			dependedBy[dep][f.Name] = true
		}
	}
	for i := range data.Files {
		if deps, ok := dependedBy[data.Files[i].Name]; ok {
			for dep := range deps {
				data.Files[i].DependedBy = append(data.Files[i].DependedBy, dep)
			}
			sort.Strings(data.Files[i].DependedBy)
		}
	}

	// ============================================================
	// 统计
	// ============================================================
	data.TotalFiles = len(data.Files)
	layerStats := make(map[string]*LayerStat)
	for _, f := range data.Files {
		data.TotalLines += f.Lines
		data.TotalSymbols += len(f.Symbols)
		for _, s := range f.Symbols {
			data.SymbolKinds[s.Kind]++
		}
		if _, ok := layerStats[f.Layer]; !ok {
			layerStats[f.Layer] = &LayerStat{
				Layer: f.Layer,
				Name:  f.LayerName,
				Color: getLayerInfo(f.Name).Color,
			}
		}
		ls := layerStats[f.Layer]
		ls.Files++
		ls.Lines += f.Lines
		ls.Symbols += len(f.Symbols)
	}
	for _, ls := range layerStats {
		data.Layers = append(data.Layers, *ls)
	}
	sort.Slice(data.Layers, func(i, j int) bool {
		return data.Layers[i].Layer < data.Layers[j].Layer
	})

	// 填充 Primitives 字段（检测四原语）
	data.Primitives = detectPrimitives(data)

	return data, nil
}

// diffArchData 比较两个架构数据快照
func diffArchData(old, new *ArchData) map[string]interface{} {
	changes := map[string]interface{}{
		"files_added":    []string{},
		"files_removed":  []string{},
		"files_modified": []string{},
	}

	if old == nil || new == nil {
		changes["message"] = "initial build"
		return changes
	}

	oldFiles := make(map[string]FileInfo)
	for _, f := range old.Files {
		oldFiles[f.Name] = f
	}
	newFiles := make(map[string]FileInfo)
	for _, f := range new.Files {
		newFiles[f.Name] = f
	}

	for name := range newFiles {
		if _, ok := oldFiles[name]; !ok {
			changes["files_added"] = append(changes["files_added"].([]string), name)
		}
	}
	for name, of := range oldFiles {
		if nf, ok := newFiles[name]; ok {
			if of.Lines != nf.Lines || len(of.Symbols) != len(nf.Symbols) {
				changes["files_modified"] = append(changes["files_modified"].([]string), name)
			}
		} else {
			changes["files_removed"] = append(changes["files_removed"].([]string), name)
		}
	}

	return changes
}
