//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateAdopt 根据迁移计划采用新模块。
func MigrateAdopt(plan *MigrationPlan) error {
	if plan == nil {
		return fmt.Errorf("migration plan is nil")
	}

	for i := range plan.Modules {
		m := &plan.Modules[i]
		if m.Status == ModuleDone || m.Status == ModuleSkipped {
			continue
		}

		m.Status = ModuleInProgress

		bridgeCode := GenerateTierBridge(plan.FromTier, plan.ToTier, m.ModuleName)
		if bridgeCode != "" && !strings.HasPrefix(bridgeCode, "// No standard bridge") {
			m.Status = ModuleDone
		} else {
			m.Status = ModuleDone
		}
	}

	return nil
}

// MigrateAdoptWithOutput 执行迁移采用并将兼容层代码写入目录。
func MigrateAdoptWithOutput(plan *MigrationPlan, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for i := range plan.Modules {
		m := &plan.Modules[i]
		if m.Status == ModuleDone || m.Status == ModuleSkipped {
			continue
		}

		bridgeCode := GenerateTierBridge(plan.FromTier, plan.ToTier, m.ModuleName)
		if bridgeCode != "" && !strings.HasPrefix(bridgeCode, "// No standard bridge") {
			filename := filepath.Join(outputDir, fmt.Sprintf("bridge_%s.go", m.ModuleName))
			if err := os.WriteFile(filename, []byte(bridgeCode), 0644); err != nil {
				return fmt.Errorf("write bridge %s: %w", filename, err)
			}
		}
		m.Status = ModuleDone
	}

	return nil
}

// MigrateAdoptSingle 采用单个模块并返回兼容层代码。
func MigrateAdoptSingle(fromTier, toTier ComplexityTier, moduleName string) (string, error) {
	bridgeCode := GenerateTierBridge(fromTier, toTier, moduleName)
	if strings.HasPrefix(bridgeCode, "// No standard bridge") {
		return "", fmt.Errorf("no bridge available for module %s", moduleName)
	}
	return bridgeCode, nil
}