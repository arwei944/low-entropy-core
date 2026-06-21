//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidationResult 表示迁移验证的结果。
type ValidationResult struct {
	Passed     bool
	CheckCount int
	Failures   []ValidationFailure
	Warnings   []string
}

// ValidationFailure 表示单个验证失败。
type ValidationFailure struct {
	Stage   string
	Message string
	Detail  string
}

// MigrateValidate 执行三重验证：编译、测试、架构合规。
func MigrateValidate(root string, targetTier ComplexityTier) *ValidationResult {
	result := &ValidationResult{Passed: true}

	result.checkCompile(root, targetTier)
	result.checkTests(root, targetTier)
	result.checkCompliance(root, targetTier)

	result.Passed = len(result.Failures) == 0
	return result
}

func (r *ValidationResult) checkCompile(root string, targetTier ComplexityTier) {
	tag := fmt.Sprintf("lecore_tier%d", targetTier)

	goCoreDir := filepath.Join(root, "go-core")
	if _, err := os.Stat(goCoreDir); os.IsNotExist(err) {
		goCoreDir = root
	}

	cmd := exec.Command("go", "build", "-tags", tag, ".")
	cmd.Dir = goCoreDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		r.Failures = append(r.Failures, ValidationFailure{
			Stage:   "compile",
			Message: fmt.Sprintf("Build failed with tag %s: %v", tag, err),
			Detail:  string(output),
		})
	} else {
		r.CheckCount++
	}
}

func (r *ValidationResult) checkTests(root string, targetTier ComplexityTier) {
	tag := fmt.Sprintf("lecore_tier%d", targetTier)

	goCoreDir := filepath.Join(root, "go-core")
	if _, err := os.Stat(goCoreDir); os.IsNotExist(err) {
		goCoreDir = root
	}

	cmd := exec.Command("go", "test", "-tags", tag, "-count=1", ".")
	cmd.Dir = goCoreDir
	output, err := cmd.CombinedOutput()

	if err != nil {
		r.Failures = append(r.Failures, ValidationFailure{
			Stage:   "test",
			Message: fmt.Sprintf("Tests failed with tag %s: %v", tag, err),
			Detail:  string(output),
		})
	} else {
		r.CheckCount++
	}
}

func (r *ValidationResult) checkCompliance(root string, _ ComplexityTier) {
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(content)

		// 检测常见的低 tier 模式
		lowPatterns := []string{
			"json.NewEncoder",
			"map[string]any",
		}
		for _, pat := range lowPatterns {
			if strings.Contains(text, pat) {
				r.Warnings = append(r.Warnings,
					fmt.Sprintf("%s: uses %s", path, pat))
			}
		}
		return nil
	})

	r.CheckCount++
}

// ValidateModule 验证单个模块的迁移。
func ValidateModule(root string, targetTier ComplexityTier) *ValidationResult {
	result := &ValidationResult{Passed: true}

	result.checkCompile(root, targetTier)
	result.checkTests(root, targetTier)

	result.Passed = len(result.Failures) == 0
	return result
}