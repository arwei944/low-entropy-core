// Package arch - 自动升级模块 (L1 Atom)
//
// 提供版本检查、下载、安装的纯函数实现。
package arch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReleaseInfo 版本发布信息
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	Assets      []Asset   `json:"assets"`
}

// Asset 发布资产（二进制文件）
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpgradeCheckResult 升级检查结果
type UpgradeCheckResult struct {
	CurrentVersion string       `json:"current_version"`
	LatestVersion  string       `json:"latest_version"`
	Available      bool         `json:"available"`
	Release        *ReleaseInfo `json:"release,omitempty"`
}

// UpgradeConfig 升级配置
type UpgradeConfig struct {
	RepoURL      string `json:"repo_url"`
	Channel      string `json:"channel"` // stable/beta/dev
	AutoCheck    bool   `json:"auto_check"`
	CheckInterval int    `json:"check_interval_hours"`
}

// DefaultUpgradeConfig 返回默认配置
func DefaultUpgradeConfig() *UpgradeConfig {
	return &UpgradeConfig{
		RepoURL:      "https://api.github.com/repos/low-entropy-core/low-entropy-core/releases/latest",
		Channel:      "stable",
		AutoCheck:    true,
		CheckInterval: 24,
	}
}

// CheckUpgrade 检查是否有新版本 (纯函数)
func CheckUpgrade(currentVersion string, config *UpgradeConfig) (*UpgradeCheckResult, error) {
	if config == nil {
		config = DefaultUpgradeConfig()
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", config.RepoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "low-entropy-core/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &UpgradeCheckResult{
			CurrentVersion: currentVersion,
			LatestVersion:  currentVersion,
			Available:      false,
		}, nil
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 解析版本号，去掉前缀 "v"
	latestVer := strings.TrimPrefix(release.TagName, "v")
	currentVer := strings.TrimPrefix(currentVersion, "v")

	available := compareVersions(latestVer, currentVer) > 0

	return &UpgradeCheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVer,
		Available:      available,
		Release:        &release,
	}, nil
}

// DownloadAsset 下载发布资产
func DownloadAsset(asset *Asset, destPath string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// GetMatchingAsset 获取匹配当前系统的资产
func GetMatchingAsset(release *ReleaseInfo) *Asset {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Windows 特殊处理
	if osName == "windows" {
		for _, a := range release.Assets {
			if strings.Contains(a.Name, "windows") && strings.Contains(a.Name, archName) && strings.HasSuffix(a.Name, ".exe") {
				return &a
			}
		}
	}

	// 通用匹配
	for _, a := range release.Assets {
		if strings.Contains(a.Name, osName) && strings.Contains(a.Name, archName) {
			return &a
		}
	}

	// 兜底：返回第一个
	if len(release.Assets) > 0 {
		return &release.Assets[0]
	}
	return nil
}

// PerformUpgrade 执行升级 (返回新的可执行文件路径)
func PerformUpgrade(currentVersion string, config *UpgradeConfig) (string, error) {
	check, err := CheckUpgrade(currentVersion, config)
	if err != nil {
		return "", err
	}
	if !check.Available {
		return "", fmt.Errorf("当前已是最新版本")
	}

	asset := GetMatchingAsset(check.Release)
	if asset == nil {
		return "", fmt.Errorf("未找到匹配的发布资产")
	}

	// 下载到临时目录
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, asset.Name)
	if err := DownloadAsset(asset, tmpFile); err != nil {
		return "", err
	}

	// 设置执行权限 (非 Windows)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile, 0755); err != nil {
			return "", fmt.Errorf("设置权限失败: %w", err)
		}
	}

	return tmpFile, nil
}

// InstallUpgrade 安装升级 (替换当前可执行文件)
func InstallUpgrade(newPath string) error {
	oldPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前路径失败: %w", err)
	}

	// Windows: 重命名旧文件为 .old
	if runtime.GOOS == "windows" {
		oldPathOld := oldPath + ".old"
		if err := os.Rename(oldPath, oldPathOld); err != nil {
			return fmt.Errorf("重命名旧文件失败: %w", err)
		}
		if err := os.Rename(newPath, oldPath); err != nil {
			os.Rename(oldPathOld, oldPath)
			return fmt.Errorf("替换文件失败: %w", err)
		}
		os.Remove(oldPathOld)
		return nil
	}

	// Unix: 使用 os.Rename 原子替换
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("替换文件失败: %w", err)
	}

	return nil
}

// compareVersions 比较版本号 (返回 1: v1>v2, -1: v1<v2, 0: 相等)
func compareVersions(v1, v2 string) int {
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")

	for i := 0; i < max(len(p1), len(p2)); i++ {
		n1 := 0
		if i < len(p1) {
			fmt.Sscanf(p1[i], "%d", &n1)
		}
		n2 := 0
		if i < len(p2) {
			fmt.Sscanf(p2[i], "%d", &n2)
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

