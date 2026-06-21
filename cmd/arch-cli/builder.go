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

// buildArchData 扫描目录并构建完整架构数据
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

	// 排序
	sort.Slice(data.Files, func(i, j int) bool {
		return data.Files[i].Name < data.Files[j].Name
	})

	// 构建符号→文件索引：记录每个导出符号定义在哪个文件
	symbolToFile := make(map[string]string)
	for _, f := range data.Files {
		for _, s := range f.Symbols {
			// 如果多个文件定义同名符号，保留第一个（通常不会发生）
			if _, exists := symbolToFile[s.Name]; !exists {
				symbolToFile[s.Name] = f.Name
			}
		}
	}

	// 基于符号引用的跨文件依赖分析：
	// 重新解析每个文件，收集所有标识符引用，匹配到定义文件
	for i := range data.Files {
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, data.Files[i].Path, nil, 0)
		if err != nil {
			continue
		}
		refs := collectCalledFunctions(astFile)
		seen := make(map[string]bool)
		seen[data.Files[i].Name] = true // 跳过自身
		// 过滤：只保留同包内其他文件定义的导出符号
		for _, ref := range refs {
			// 只关注首字母大写的导出符号（同包内引用其他文件的导出符号）
			if len(ref) == 0 || ref[0] < 'A' || ref[0] > 'Z' {
				continue
			}
			if defFile, ok := symbolToFile[ref]; ok && !seen[defFile] {
				seen[defFile] = true
				data.Files[i].DependsOn = append(data.Files[i].DependsOn, defFile)
			}
		}
		sort.Strings(data.Files[i].DependsOn)
	}

	// 计算被依赖关系
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

	// 统计
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
