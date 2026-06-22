package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	arch "low-entropy-core/go-core/arch"
)

func parseInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// OriginEntry 表示一个符号的起源信息
type OriginEntry struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Layer      string   `json:"layer"`
	LayerName  string   `json:"layer_name"`
	File       string   `json:"file"`
	FilePath   string   `json:"file_path"`
	Package    string   `json:"package"`
	IsExported bool     `json:"is_exported"`
	Primitive  string   `json:"primitive,omitempty"`
	Doc        string   `json:"doc,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"`
	UsedBy     []string `json:"used_by,omitempty"`
	Ancestry   []string `json:"ancestry,omitempty"`
}

// OriginResponse 溯源响应
type OriginResponse struct {
	Total     int                    `json:"total"`
	ByLayer   map[string]int         `json:"by_layer"`
	ByKind    map[string]int         `json:"by_kind"`
	Symbols   []OriginEntry          `json:"symbols"`
	SymbolMap map[string]*OriginEntry `json:"-"`
}

// handleOrigin 返回符号溯源信息
func handleOrigin(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	query := r.URL.Query().Get("q")
	layerFilter := r.URL.Query().Get("layer")
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 && n < 2000 {
			limit = n
		}
	}

	resp := buildOriginResponse(archData, query, layerFilter, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleOriginDetail 返回单个符号的详细溯源
func handleOriginDetail(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	if archData == nil {
		http.Error(w, `{"error":"no data available"}`, http.StatusServiceUnavailable)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}

	resp := buildOriginDetail(archData, name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func buildOriginResponse(data *arch.ArchData, query, layerFilter string, limit int) OriginResponse {
	resp := OriginResponse{
		ByLayer:   make(map[string]int),
		ByKind:    make(map[string]int),
		SymbolMap: make(map[string]*OriginEntry),
	}

	queryLower := strings.ToLower(query)

	for i := range data.Files {
		file := &data.Files[i]
		for j := range file.Symbols {
			sym := &file.Symbols[j]

			if layerFilter != "" && file.Layer != layerFilter {
				continue
			}
			if queryLower != "" && !strings.Contains(strings.ToLower(sym.Name), queryLower) {
				continue
			}

			primType := inferPrimitiveByName(sym.Name)
			entry := OriginEntry{
				Name:       sym.Name,
				Kind:       sym.Kind,
				Layer:      file.Layer,
				LayerName:  file.LayerName,
				File:       file.Name,
				FilePath:   file.Path,
				Package:    file.Package,
				IsExported: sym.IsExported,
				Primitive:  primType,
				Doc:        truncate(sym.Doc, 200),
				DependsOn:  file.DependsOn,
			}
			resp.Symbols = append(resp.Symbols, entry)
			resp.ByLayer[file.Layer]++
			resp.ByKind[sym.Kind]++
		}
	}

	resp.Total = len(resp.Symbols)

	if len(resp.Symbols) > limit {
		resp.Symbols = resp.Symbols[:limit]
	}

	return resp
}

func buildOriginDetail(data *arch.ArchData, name string) map[string]interface{} {
	// 查找符号
	for i := range data.Files {
		file := &data.Files[i]
		for j := range file.Symbols {
			sym := &file.Symbols[j]
			if sym.Name != name {
				continue
			}

			// 构建祖先链 - 沿着 depends_on 回溯
			visited := make(map[string]bool)
			var ancestry []string
			buildAncestry(data, file.Name, &visited, &ancestry, 0)

			// 查找引用此符号的文件
			var usedBy []string
			for k := range data.Files {
				other := &data.Files[k]
				for _, dep := range other.DependsOn {
					if dep == file.Name {
						usedBy = append(usedBy, other.Name)
						break
					}
				}
			}

			return map[string]interface{}{
				"name":        sym.Name,
				"kind":        sym.Kind,
				"layer":       file.Layer,
				"layer_name":  file.LayerName,
				"file":        file.Name,
				"file_path":   file.Path,
				"package":     file.Package,
				"is_exported": sym.IsExported,
				"doc":         sym.Doc,
				"signature":   sym.Signature,
				"depends_on":  file.DependsOn,
				"used_by":     usedBy,
				"ancestry":    ancestry,
				"signature_metrics": map[string]interface{}{
					"total_ancestors": visited[file.Name],
					"depth":           len(ancestry),
				},
			}
		}
	}

	return map[string]interface{}{
		"error": "symbol not found: " + name,
	}
}

func buildAncestry(data *arch.ArchData, fileName string, visited *map[string]bool, ancestry *[]string, depth int) {
	if depth > 20 {
		return
	}
	if (*visited)[fileName] {
		return
	}
	(*visited)[fileName] = true

	// 找到文件
	for i := range data.Files {
		file := &data.Files[i]
		if file.Name == fileName {
			*ancestry = append(*ancestry, file.Name)
			for _, dep := range file.DependsOn {
				buildAncestry(data, dep, visited, ancestry, depth+1)
			}
			break
		}
	}
}
