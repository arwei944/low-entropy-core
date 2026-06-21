package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// SimulateResult 模拟运行结果
type SimulateResult struct {
	Package   string    `json:"package"`
	Action    string    `json:"action"` // "build", "test", "bench", "vet"
	Status    string    `json:"status"` // "pass", "fail", "error"
	Output    string    `json:"output"`
	Duration  string    `json:"duration"`
	TestCount int       `json:"test_count"`
	PassCount int       `json:"pass_count"`
	FailCount int       `json:"fail_count"`
	Coverage  string    `json:"coverage,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// handleSimulate 执行代码模拟运行
// POST /api/simulate?pkg=./go-core&action=test
// GET /api/simulate?pkg=./go-core&action=build
func handleSimulate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	pkg := r.URL.Query().Get("pkg")
	action := r.URL.Query().Get("action")
	if pkg == "" {
		pkg = "."
	}
	if action == "" {
		action = "test"
	}

	result := SimulateResult{
		Package:   pkg,
		Action:    action,
		Timestamp: time.Now(),
	}

	start := time.Now()

	var cmd *exec.Cmd
	switch action {
	case "build":
		cmd = exec.Command("go", "build", pkg)
	case "test":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-v", "-timeout=60s")
	case "test-race":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-race", "-v", "-timeout=120s")
	case "test-coverage":
		cmd = exec.Command("go", "test", pkg, "-count=1", "-cover", "-v", "-timeout=60s")
	case "bench":
		cmd = exec.Command("go", "test", pkg, "-bench=.", "-benchmem", "-timeout=120s")
	case "vet":
		cmd = exec.Command("go", "vet", pkg)
	default:
		cmd = exec.Command("go", "build", pkg)
	}

	cmd.Dir = sourceDir
	output, err := cmd.CombinedOutput()
	result.Output = strings.TrimSpace(string(output))
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		result.Status = "fail"
		result.Error = err.Error()

		// 解析测试输出
		if strings.Contains(result.Output, "--- PASS") || strings.Contains(result.Output, "--- FAIL") {
			result.Status = "fail"
			lines := strings.Split(result.Output, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "--- PASS:") {
					result.PassCount++
					result.TestCount++
				} else if strings.HasPrefix(line, "--- FAIL:") {
					result.FailCount++
					result.TestCount++
				} else if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") {
					// 汇总行
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						// 解析覆盖率
						for _, p := range parts {
							if strings.Contains(p, "coverage:") {
								result.Coverage = strings.TrimSuffix(p, ",")
							}
						}
					}
				}
			}
		}
	} else {
		result.Status = "pass"
		// 解析成功的测试输出
		lines := strings.Split(result.Output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "--- PASS:") {
				result.PassCount++
				result.TestCount++
			} else if strings.HasPrefix(line, "ok") {
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.Contains(p, "coverage:") {
						result.Coverage = strings.TrimSuffix(p, ",")
					}
				}
			}
		}
	}

	json.NewEncoder(w).Encode(result)
}

// parsePercent 解析百分比字符串
func parsePercent(s string) (float64, error) {
	s = strings.TrimSuffix(s, "%")
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
