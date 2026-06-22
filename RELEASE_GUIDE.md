# arch-cli 发布指南

## 概述

arch-cli 现在支持完整的自动化发布流程，包括：

- **GitHub Actions 自动发布**：推送标签或手动触发
- **多平台构建**：Windows, Linux, macOS (amd64/arm64)
- **自动升级**：用户可通过 `arch upgrade` 一键更新
- **SHA256 校验**：确保下载文件完整性

## 发布流程

### 方式一：推送标签自动发布（推荐）

```powershell
# 1. 确保工作目录干净
git status

# 2. 创建并推送标签
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 3. GitHub Actions 会自动触发发布流程
```

### 方式二：使用本地发布脚本

```powershell
# 1. 运行发布脚本（自动构建多平台）
.\scripts\release.ps1 -Version v1.0.0 -Tag -PushTag

# 2. 或仅构建不推送
.\scripts\release.ps1 -Version v1.0.0 -BuildOnly
```

### 方式三：GitHub Actions 手动触发

1. 前往仓库的 Actions 页面
2. 选择 "Release" 工作流
3. 点击 "Run workflow"
4. 输入版本号（如 v1.0.0）并运行

## 配置仓库地址

**重要**：首次发布前，需要修改默认的仓库地址：

```go
// go-core/arch/upgrade.go
func DefaultUpgradeConfig() *UpgradeConfig {
	return &UpgradeConfig{
		// 修改为实际的仓库地址
		RepoURL: "https://api.github.com/repos/YOUR_USERNAME/YOUR_REPO/releases/latest",
		// ...
	}
}
```

## 验证发布

发布完成后，验证以下内容：

1. **Release 页面**：检查 Releases 页面是否有新版本
2. **资产文件**：确认 4 个平台的二进制文件都已上传
3. **校验和**：验证 checksums 文件正确
4. **版本号**：下载后运行 `arch-cli version` 确认

## 用户使用指南

### 用户使用指南

#### 一键安装（推荐）

##### Windows (一条命令)

```powershell
irm https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex
```

##### Linux/macOS (一条命令)

```bash
curl -fsSL https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash
```

#### 手动安装

##### Windows (PowerShell)

```powershell
# 下载
Invoke-WebRequest -Uri "https://github.com/USERNAME/REPO/releases/download/v1.0.0/arch-cli-v1.0.0-windows-amd64.exe" -OutFile "arch-cli.exe"

# 移动到 PATH
Move-Item arch-cli.exe C:\Windows\System32\

# 验证
arch-cli version
```

##### Linux/macOS

```bash
# 下载
curl -L -o arch-cli https://github.com/USERNAME/REPO/releases/download/v1.0.0/arch-cli-v1.0.0-linux-amd64

# 执行权限
chmod +x arch-cli

# 移动到 PATH
sudo mv arch-cli /usr/local/bin/

# 验证
arch-cli version
```

### 升级

```bash
# 检查更新
arch-cli upgrade check

# 完整升级（自动下载+安装）
arch-cli upgrade

# 分步操作
arch-cli upgrade download
arch-cli upgrade install
```

## 文件结构

```
.github/
└── workflows/
    ├── ci.yml          # 持续集成
    └── release.yml     # 发布流程

scripts/
└── release.ps1       # 本地发布脚本

cmd/
└── arch-cli/
    ├── main.go        # 主入口（支持版本注入）
    └── upgrade.go     # 升级命令

go-core/
└── arch/
    └── upgrade.go     # 升级核心逻辑
```

## 版本号规范

遵循语义化版本 (Semantic Versioning):

- **主版本 (Major)**：不兼容的 API 修改
- **次版本 (Minor)**：向下兼容的功能性新增
- **修订 (Patch)**：向下兼容的问题修正

格式：`vMAJOR.MINOR.PATCH` (例如 `v1.0.0`, `v1.1.0`, `v2.0.0`)

## Troubleshooting

### GitHub Actions 权限问题

确保仓库 Settings > Actions > General 中：
- "Read and write permissions" 已启用
- "Allow GitHub Actions to create and approve pull requests" 已启用（如需要）

### 构建失败

- 检查 Go 版本是否匹配
- 确认 go.mod/sum 正确
- 查看 Actions 日志详细错误

### 升级功能不工作

- 确认 RepoURL 配置正确
- 检查网络连接
- 验证 Release 资产命名格式正确

