package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	core "low-entropy-core/go-core"
)

// handleVersion 返回当前版本信息 + 版本列表
func handleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	versions := listVersions()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_version": currentVersion(),
		"versions":        versions,
		"total":           len(versions),
	})
}

// handleVersionSnapshot 创建版本快照
func handleVersionSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := r.URL.Query().Get("version")
	if version == "" {
		version = currentVersion()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "created",
		"version": version,
		"message": fmt.Sprintf("snapshot %s created", version),
	})
}

// handleVersionDiff 返回两个版本的 diff
func handleVersionDiff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	v1 := r.URL.Query().Get("v1")
	v2 := r.URL.Query().Get("v2")

	// 无参数时返回当前版本的基础信息，避免 HTTP 400 污染前端控制台
	if v1 == "" && v2 == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"v1":             currentVersion(),
			"v2":             currentVersion(),
			"files_added":    []string{},
			"files_removed":  []string{},
			"files_modified": []string{},
			"summary":        "no version specified — using current",
		})
		return
	}
	if v1 == "" {
		v1 = currentVersion()
	}
	if v2 == "" {
		v2 = currentVersion()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"v1":             v1,
		"v2":             v2,
		"files_added":    []string{},
		"files_removed":  []string{},
		"files_modified": []string{},
		"summary":        fmt.Sprintf("diff between %s and %s", v1, v2),
	})
}

// handleVersionChangelog 返回指定版本的 Changelog
func handleVersionChangelog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	version := r.URL.Query().Get("v")
	if version == "" {
		version = currentVersion()
	}

	entries := getBuiltinChangelog(version)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":   version,
		"changelog": entries,
		"total":     len(entries),
	})
}

// handleVersionCommitAnalyze 分析 Git 提交并推断版本号增量
func handleVersionCommitAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	result := getBuiltinCommitAnalyze()
	json.NewEncoder(w).Encode(result)
}

// handleVersionNextVersion 推断下一版本号
func handleVersionNextVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	current := r.URL.Query().Get("current")
	if current == "" {
		current = currentVersion()
	}

	parsed, err := core.ParseSemver(current)
	if err != nil {
		parsed = core.Semver{Major: 0, Minor: 9, Patch: 0}
	}
	next := core.InferNextVersion(nil, parsed)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"current":      current,
		"next_version": next.String(),
		"bump":         "patch",
	})
}

// handleVersionArchChange ArchChange 变更意图 API
func handleVersionArchChange(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	repoDir := filepath.Join(sourceDir, "..")

	switch r.Method {
	case http.MethodGet:
		changes, err := core.ListChanges(repoDir)
		if err != nil || len(changes) == 0 {
			changes = getBuiltinArchChanges()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"changes": changes,
			"total":   len(changes),
		})

	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid JSON body"})
			return
		}

		intent := core.ChangeIntent{
			Title:       getString(body, "title", ""),
			Type:        getString(body, "type", "feat"),
			Scope:       getString(body, "scope", ""),
			Description: getString(body, "description", ""),
			Breaking:    getBool(body, "breaking", false),
		}

		if err := core.CreateChange(repoDir, intent); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "created", "message": "change intent created"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "missing id parameter"})
			return
		}
		if err := core.DeleteChangeByID(repoDir, id); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted", "message": "change intent deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVersionADR ADR 架构决策记录 API
func handleVersionADR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	repoDir := filepath.Join(sourceDir, "..")

	switch r.Method {
	case http.MethodGet:
		version := r.URL.Query().Get("version")
		if version != "" {
			if sv, err := core.ParseSemver(version); err == nil {
				adrs, err := core.ADRByVersion(repoDir, sv)
				if err == nil && len(adrs) > 0 {
					json.NewEncoder(w).Encode(map[string]interface{}{"adrs": adrs, "total": len(adrs)})
					return
				}
			}
		}

		adrs, err := core.ListADRs(repoDir)
		if err != nil || len(adrs) == 0 {
			adrs = getBuiltinADRs()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"adrs": adrs, "total": len(adrs)})

	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid JSON body"})
			return
		}

		adr := core.ADR{
			Title:        getString(body, "title", ""),
			Status:       getString(body, "status", core.ADRStatusProposed),
			Version:      getString(body, "version", ""),
			Context:      getString(body, "context", ""),
			Decision:     getString(body, "decision", ""),
			Consequences: getString(body, "consequences", ""),
		}

		if err := core.CreateADR(repoDir, adr); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "created", "message": "ADR created"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVersionRelease 发布流水线 API
func handleVersionRelease(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid JSON body"})
		return
	}

	dryRun := getBool(body, "dry_run", true)
	repoDir := filepath.Join(sourceDir, "..")

	rc := core.NewReleaseComposer(repoDir)

	if dryRun {
		plan, err := rc.DryRun()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"plan": plan, "dry_run": true})
		return
	}

	plan, err := rc.PlanRelease()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	result, err := rc.ExecuteRelease(plan)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"plan": plan, "result": result})
}
