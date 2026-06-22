# arch-cli 一键安装脚本 (Windows PowerShell)
# 使用方法:
#   iwr -useb https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex
# 或
#   irm https://raw.githubusercontent.com/USERNAME/REPO/main/install.ps1 | iex

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\arch-cli",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  arch-cli 一键安装" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 配置仓库地址（需要修改为实际地址）
$repoOwner = "USERNAME"
$repoName = "REPO"
$apiUrl = "https://api.github.com/repos/$repoOwner/$repoName/releases/latest"

# 确定架构和系统
$os = "windows"
$arch = "amd64"
$ext = ".exe"

Write-Host "系统: $os/$arch" -ForegroundColor Green
Write-Host ""

# 创建安装目录
if (-not (Test-Path $InstallDir)) {
    Write-Host "[1/5] 创建安装目录..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Host "✓ $InstallDir" -ForegroundColor Green
} else {
    Write-Host "[1/5] 安装目录已存在" -ForegroundColor Yellow
}
Write-Host ""

# 获取最新版本信息
Write-Host "[2/5] 获取版本信息..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri $apiUrl -Headers @{"Accept"="application/vnd.github.v3+json"} -ErrorAction Stop
    $latestVersion = $response.tag_name
    $assets = $response.assets

    if ($Version -eq "latest") {
        $installVersion = $latestVersion
    } else {
        $installVersion = $Version
    }

    Write-Host "✓ 最新版本: $latestVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ 获取版本信息失败: $_" -ForegroundColor Red
    Write-Host ""
    Write-Host "如果仓库还未发布，请先发布一个 Release" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "测试模式：使用本地编译的版本" -ForegroundColor Gray
    $localExe = "arch-cli.exe"
    if (Test-Path $localExe) {
        Write-Host "找到本地文件: $localExe" -ForegroundColor Green
        $installVersion = "v1.0.0"
    } else {
        Write-Host "未找到本地文件，先编译..." -ForegroundColor Yellow
        go build -tags lecore_tier4 -o $localExe ./cmd/arch-cli
        if (-not (Test-Path $localExe)) {
            Write-Host "✗ 编译失败" -ForegroundColor Red
            exit 1
        }
        $installVersion = "v1.0.0"
    }
    $assets = $null
}
Write-Host ""

# 下载文件
Write-Host "[3/5] 下载 arch-cli..." -ForegroundColor Yellow
$targetName = "arch-cli-${installVersion}-${os}-${arch}${ext}"
$installPath = Join-Path $InstallDir "arch-cli.exe"

if ($assets) {
    # 从 GitHub 下载
    $asset = $assets | Where-Object { $_.name -eq $targetName }
    if (-not $asset) {
        Write-Host "✗ 未找到匹配的资产: $targetName" -ForegroundColor Red
        Write-Host "可用资产:" -ForegroundColor Yellow
        $assets | ForEach-Object { Write-Host "  - $($_.name)" -ForegroundColor Gray }
        exit 1
    }

    $downloadUrl = $asset.browser_download_url
    Write-Host "从 $downloadUrl 下载..."

    $webClient = New-Object System.Net.WebClient
    $webClient.DownloadFile($downloadUrl, $installPath)
    Write-Host "✓ 已下载" -ForegroundColor Green
} else {
    # 使用本地文件（测试模式）
    $localExe = "arch-cli.exe"
    if (Test-Path $localExe) {
        Copy-Item $localExe $installPath -Force
        Write-Host "✓ 已复制本地文件" -ForegroundColor Green
    } else {
        Write-Host "✗ 找不到本地文件" -ForegroundColor Red
        exit 1
    }
}
Write-Host ""

# 检查文件
Write-Host "[4/5] 验证安装..." -ForegroundColor Yellow
if (Test-Path $installPath) {
    $fileSize = (Get-Item $installPath).Length
    $sizeKB = [math]::Round($fileSize / 1KB, 2)
    Write-Host "✓ 文件: $installPath (${sizeKB} KB)" -ForegroundColor Green
} else {
    Write-Host "✗ 安装失败，文件不存在" -ForegroundColor Red
    exit 1
}
Write-Host ""

# 添加到 PATH
Write-Host "[5/5] 配置环境变量..." -ForegroundColor Yellow
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
    Write-Host "✓ 已添加到用户 PATH" -ForegroundColor Green
    Write-Host ""
    Write-Host "提示: 请重启终端或运行以下命令使环境变量生效:" -ForegroundColor Yellow
    Write-Host "  `$env:Path = [System.Environment]::GetEnvironmentVariable('Path','User') + ';' + [System.Environment]::GetEnvironmentVariable('Path','Machine')"
} else {
    Write-Host "✓ 已在 PATH 中" -ForegroundColor Green
}
Write-Host ""

# 完成
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  安装完成! ($installVersion)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "下一步:" -ForegroundColor Yellow
Write-Host "  1. 重启终端或刷新环境变量"
Write-Host "  2. 运行: arch-cli version"
Write-Host "  3. 运行: arch-cli help"
Write-Host ""
Write-Host "测试命令:" -ForegroundColor Gray
$testCommand = "& `"$installPath`" version"
$result = Invoke-Expression $testCommand 2>&1
Write-Host "  arch-cli version -> $result" -ForegroundColor Green
Write-Host ""

