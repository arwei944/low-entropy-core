# arch-cli 安装指南

## 一键安装（推荐）

任何人在任何位置，只需要一条命令即可完成安装！

### Windows

```powershell
# 使用 PowerShell（推荐）
irm https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex

# 或使用 iwr
iwr -useb https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex
```

### Linux/macOS

```bash
# 使用 curl
curl -fsSL https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash

# 或使用 wget
wget -qO- https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash
```

安装完成后，**重启终端**，然后运行 `arch-cli version` 验证安装！

---

## 安装特性

| 特性 | 说明 |
|------|------|
| **跨平台** | 支持 Windows、Linux、macOS |
| **自动检测** | 自动检测系统/架构 |
| **自动配置** | 自动添加到 PATH |
| **本地降级** | GitHub 不可用时支持本地编译（需要 Go） |

---

## 高级用法

### 指定版本安装

```powershell
# Windows
irm https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex -Version v1.0.0

# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash -s -- -v v1.0.0
```

### 指定安装目录

```powershell
# Windows
irm https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex -InstallDir "C:\tools"

# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash -s -- -d ~/tools
```

---

## 验证安装

```bash
# 查看版本
arch-cli version

# 查看帮助
arch-cli help

# 测试分析
arch-cli analyze --dir .
```

---

## 升级

安装完成后，使用内置升级命令：

```bash
# 检查更新
arch-cli upgrade check

# 一键升级
arch-cli upgrade
```

---

## 手动安装（备用）

如果一键脚本不可用，可以手动下载：

1. 访问 GitHub Releases 页面
2. 下载对应平台的二进制文件
3. 解压/移动到 PATH 目录

详细说明见 [RELEASE_GUIDE.md](./RELEASE_GUIDE.md)

---

## 常见问题

### Q: 安装后提示找不到命令？

A: 重启终端或刷新环境变量：
- Windows: `$env:Path = [System.Environment]::GetEnvironmentVariable('Path','User') + ';' + [System.Environment]::GetEnvironmentVariable('Path','Machine')`
- Linux/macOS: `source ~/.bashrc` 或 `source ~/.zshrc`

### Q: PowerShell 脚本执行失败？

A: 临时允许脚本执行：
```powershell
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
```

### Q: 安装脚本访问 GitHub 失败？

A: 脚本会自动降级为本地编译模式，前提是安装了 Go。

---

## 卸载

```powershell
# Windows
Remove-Item "$env:LOCALAPPDATA\arch-cli" -Recurse -Force
# 然后手动从 PATH 移除

# Linux/macOS
rm -rf ~/.local/bin/arch-cli
```

