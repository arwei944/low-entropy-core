// arch-cli - 自动升级命令实现
//
// 提供 upgrade check/upgrade/upgrade install 命令
// 以及启动时自动检查功能。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"low-entropy-core/go-core/arch"
)

// UpgradeState 升级状态存储
type UpgradeState struct {
	LastCheckTime   time.Time `json:"last_check_time"`
	LastCheckResult *arch.UpgradeCheckResult `json:"last_result,omitempty"`
	SkippedVersion  string    `json:"skipped_version,omitempty"`
}

var upgradeState *UpgradeState

// getUpgradeStatePath 获取状态文件路径
func getUpgradeStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".low-entropy-core")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "upgrade-state.json")
}

// loadUpgradeState 加载升级状态
func loadUpgradeState() *UpgradeState {
	if upgradeState != nil {
		return upgradeState
	}

	path := getUpgradeStatePath()
	if path == "" {
		return &UpgradeState{}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return &UpgradeState{}
	}

	var state UpgradeState
	if err := json.Unmarshal(b, &state); err != nil {
		return &UpgradeState{}
	}

	upgradeState = &state
	return &state
}

// saveUpgradeState 保存升级状态
func saveUpgradeState(state *UpgradeState) error {
	upgradeState = state
	path := getUpgradeStatePath()
	if path == "" {
		return fmt.Errorf("无法获取状态路径")
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// ──────────────────────────────────────────────
// upgrade check 命令
// ──────────────────────────────────────────────
func cmdUpgradeCheck(args []string) {
	force := hasFlag(args, "--force")
	jsonOut := hasFlag(args, "--json")

	state := loadUpgradeState()
	config := arch.DefaultUpgradeConfig()

	// 检查是否需要跳过（非强制模式）
	if !force {
		// 检查是否在间隔时间内
		sinceLastCheck := time.Since(state.LastCheckTime)
		if sinceLastCheck < time.Duration(config.CheckInterval)*time.Hour {
			if jsonOut {
				b, _ := json.MarshalIndent(state.LastCheckResult, "", "  ")
				fmt.Println(string(b))
				return
			}
			fmt.Printf("距离上次检查: %s (自动检查间隔: %d小时)\n",
				sinceLastCheck.Round(time.Minute), config.CheckInterval)
			fmt.Println("使用 --force 强制检查")
			if state.LastCheckResult != nil {
				fmt.Printf("上次检查结果: 当前 v%s, 最新 v%s\n",
					state.LastCheckResult.CurrentVersion,
					state.LastCheckResult.LatestVersion)
			}
			return
		}
	}

	fmt.Printf("正在检查更新 (当前版本: v%s)...\n", version)

	result, err := arch.CheckUpgrade(version, config)
	if err != nil {
		fatal("检查失败:", err)
	}

	// 更新状态
	state.LastCheckTime = time.Now()
	state.LastCheckResult = result
	saveUpgradeState(state)

	if jsonOut {
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("")
	if result.Available {
		fmt.Printf("🎉 发现新版本: v%s → v%s\n", version, result.LatestVersion)
		if result.Release != nil {
			fmt.Println("")
			fmt.Println("发布说明:")
			if len(result.Release.Body) > 200 {
				fmt.Println("  " + result.Release.Body[:200] + "...")
			} else {
				fmt.Println("  " + result.Release.Body)
			}
			if len(result.Release.Assets) > 0 {
				fmt.Println("")
				fmt.Println("可用资产:")
				for _, a := range result.Release.Assets {
					fmt.Printf("  - %s (%d bytes)\n", a.Name, a.Size)
				}
			}
		}
		fmt.Println("")
		fmt.Println("运行以下命令升级:")
		fmt.Println("  arch upgrade")
	} else {
		fmt.Printf("✅ 已是最新版本 (v%s)\n", version)
	}
}

// ──────────────────────────────────────────────
// upgrade download 命令
// ──────────────────────────────────────────────
func cmdUpgradeDownload(args []string) {
	config := arch.DefaultUpgradeConfig()

	fmt.Printf("正在检查更新 (当前版本: v%s)...\n", version)

	result, err := arch.CheckUpgrade(version, config)
	if err != nil {
		fatal("检查失败:", err)
	}

	if !result.Available {
		fmt.Printf("✅ 已是最新版本 (v%s)\n", version)
		return
	}

	fmt.Printf("下载 v%s...\n", result.LatestVersion)

	tmpPath, err := arch.PerformUpgrade(version, config)
	if err != nil {
		fatal("下载失败:", err)
	}

	fmt.Printf("✅ 已下载到: %s\n", tmpPath)
	fmt.Println("")
	fmt.Println("运行以下命令安装:")
	fmt.Println("  arch upgrade install")
}

// ──────────────────────────────────────────────
// upgrade install 命令
// ──────────────────────────────────────────────
func cmdUpgradeInstall(args []string) {
	newPath := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		newPath = args[0]
	}

	if newPath == "" {
		// 尝试从临时目录查找
		config := arch.DefaultUpgradeConfig()
		result, err := arch.CheckUpgrade(version, config)
		if err == nil && result.Available && result.Release != nil {
			asset := arch.GetMatchingAsset(result.Release)
			if asset != nil {
				tmpPath := filepath.Join(os.TempDir(), asset.Name)
				if _, err := os.Stat(tmpPath); err == nil {
					newPath = tmpPath
				}
			}
		}
	}

	if newPath == "" {
		fatal("请指定要安装的二进制文件路径，或先运行 'arch upgrade download'")
	}

	if _, err := os.Stat(newPath); err != nil {
		fatal("文件不存在:", newPath)
	}

	fmt.Printf("正在安装 v%s...\n", version)

	if err := arch.InstallUpgrade(newPath); err != nil {
		fatal("安装失败:", err)
	}

	fmt.Println("✅ 升级成功！")
	fmt.Println("")
	fmt.Println("请重新运行 arch-cli 以使用新版本。")
}

// ──────────────────────────────────────────────
// upgrade (主命令) - 完整升级流程
// ──────────────────────────────────────────────
func cmdUpgrade(args []string) {
	if len(args) == 0 {
		// 默认: 检查 → 下载 → 安装
		config := arch.DefaultUpgradeConfig()

		fmt.Printf("正在检查更新 (当前版本: v%s)...\n", version)

		result, err := arch.CheckUpgrade(version, config)
		if err != nil {
			fatal("检查失败:", err)
		}

		if !result.Available {
			fmt.Printf("✅ 已是最新版本 (v%s)\n", version)
			return
		}

		fmt.Printf("发现新版本 v%s\n", result.LatestVersion)
		fmt.Printf("正在下载...\n")

		tmpPath, err := arch.PerformUpgrade(version, config)
		if err != nil {
			fatal("下载失败:", err)
		}

		fmt.Printf("正在安装...\n")

		if err := arch.InstallUpgrade(tmpPath); err != nil {
			fatal("安装失败:", err)
		}

		fmt.Println("✅ 升级成功！")
		fmt.Println("")
		fmt.Println("请重新运行 arch-cli 以使用新版本。")
		return
	}

	// 子命令
	switch args[0] {
	case "check":
		cmdUpgradeCheck(args[1:])
	case "download":
		cmdUpgradeDownload(args[1:])
	case "install":
		cmdUpgradeInstall(args[1:])
	default:
		fmt.Println("未知子命令:", args[0])
		fmt.Println("")
		fmt.Println("用法:")
		fmt.Println("  arch upgrade              完整升级流程 (检查→下载→安装)")
		fmt.Println("  arch upgrade check        检查更新")
		fmt.Println("  arch upgrade download     下载新版本")
		fmt.Println("  arch upgrade install      安装已下载的版本")
	}
}

// ──────────────────────────────────────────────
// autoCheckUpgrade 启动时自动检查 (后台)
// ──────────────────────────────────────────────
func autoCheckUpgrade() {
	state := loadUpgradeState()
	config := arch.DefaultUpgradeConfig()

	if !config.AutoCheck {
		return
	}

	// 检查是否在间隔时间内
	sinceLastCheck := time.Since(state.LastCheckTime)
	if sinceLastCheck < time.Duration(config.CheckInterval)*time.Hour {
		return
	}

	// 后台检查
	go func() {
		result, err := arch.CheckUpgrade(version, config)
		if err != nil {
			return
		}

		// 更新状态
		state.LastCheckTime = time.Now()
		state.LastCheckResult = result
		saveUpgradeState(state)

		// 如果有新版本，提示用户
		if result.Available {
			fmt.Println("")
			fmt.Printf("⚠️  发现新版本: v%s → v%s\n", version, result.LatestVersion)
			fmt.Println("运行 'arch upgrade' 升级")
		}
	}()
}

