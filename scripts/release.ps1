# arch-cli 本地发布脚本 (PowerShell)
# 用于本地测试发布流程

param(
    [string]$Version = "v1.0.0",
    [switch]$Tag,
    [switch]$PushTag,
    [switch]$BuildOnly
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  arch-cli 发布流程" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 验证版本号格式
if (-not $Version -match '^v\d+\.\d+\.\d+$') {
    Write-Host "错误: 版本号格式错误，应为 vx.y.z (例如 v1.0.0)" -ForegroundColor Red
    exit 1
}

Write-Host "版本: $Version" -ForegroundColor Green
Write-Host ""

# 检查 git status
Write-Host "[1/6] 检查 Git 状态..." -ForegroundColor Yellow
$gitStatus = git status --porcelain
if ($gitStatus) {
    Write-Host "警告: 工作目录不干净" -ForegroundColor Yellow
    Write-Host $gitStatus
    $confirm = Read-Host "是否继续? (y/N)"
    if ($confirm -ne 'y' -and $confirm -ne 'Y') {
        Write-Host "已取消" -ForegroundColor Red
        exit 0
    }
} else {
    Write-Host "✓ 工作目录干净" -ForegroundColor Green
}
Write-Host ""

# 清理旧的构建文件
Write-Host "[2/6] 清理旧的构建文件..." -ForegroundColor Yellow
$buildDir = "dist"
if (Test-Path $buildDir) {
    Remove-Item -Recurse -Force $buildDir
}
New-Item -ItemType Directory -Path $buildDir | Out-Null
Write-Host "✓ 已清理" -ForegroundColor Green
Write-Host ""

# 多平台构建
Write-Host "[3/6] 多平台构建..." -ForegroundColor Yellow

$targets = @(
    @{OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{OS = "linux"; Arch = "amd64"; Ext = "" },
    @{OS = "darwin"; Arch = "amd64"; Ext = "" },
    @{OS = "darwin"; Arch = "arm64"; Ext = "" }
)

foreach ($target in $targets) {
    $os = $target.OS
    $arch = $target.Arch
    $ext = $target.Ext
    $outputName = "arch-cli-${Version}-${os}-${arch}${ext}"
    $outputPath = Join-Path $buildDir $outputName

    Write-Host "  构建 ${os}/${arch}..." -NoNewline

    $env:GOOS = $os
    $env:GOARCH = $arch
    $env:CGO_ENABLED = "0"

    $versionNum = $Version -replace '^v', ''
    $ldFlags = "-X main.version=${versionNum} -s -w"

    Push-Location "cmd/arch-cli"
    try {
        go build -tags lecore_tier4 -ldflags $ldFlags -o "../../${outputPath}" .
        if ($LASTEXITCODE -eq 0) {
            Write-Host " ✓" -ForegroundColor Green
        } else {
            Write-Host " ✗ 构建失败" -ForegroundColor Red
            exit 1
        }
    } finally {
        Pop-Location
    }
}
Write-Host ""

# 生成 SHA256 校验和
Write-Host "[4/6] 生成校验和..." -ForegroundColor Yellow
Push-Location $buildDir
$checksumFile = "checksums.sha256"
Get-ChildItem -File | ForEach-Object {
    $hash = (Get-FileHash -Path $_.Name -Algorithm SHA256).Hash.ToLower()
    "${hash}  $($_.Name)" | Out-File -Append -FilePath $checksumFile -Encoding utf8
}
Pop-Location
Write-Host "✓ 已生成 $checksumFile" -ForegroundColor Green
Write-Host ""

# 创建标签（如果需要）
if ($Tag) {
    Write-Host "[5/6] 创建 Git 标签..." -ForegroundColor Yellow
    git tag -a $Version -m "Release $Version"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ 标签已创建: $Version" -ForegroundColor Green
    } else {
        Write-Host "✗ 标签创建失败" -ForegroundColor Red
        exit 1
    }

    if ($PushTag) {
        Write-Host "  推送标签到远程..." -NoNewline
        git push origin $Version
        if ($LASTEXITCODE -eq 0) {
            Write-Host " ✓" -ForegroundColor Green
        } else {
            Write-Host " ✗ 推送失败" -ForegroundColor Red
            exit 1
        }
    }
}
Write-Host ""

# 完成
Write-Host "[6/6] 完成!" -ForegroundColor Green
Write-Host ""
Write-Host "构建产物:" -ForegroundColor Cyan
Get-ChildItem $buildDir | ForEach-Object {
    $size = [math]::Round($_.Length / 1KB, 2)
    Write-Host "  $($_.Name) (${size} KB)"
}
Write-Host ""
Write-Host "发布步骤:" -ForegroundColor Yellow
Write-Host "  1. 前往 GitHub Actions 页面"
Write-Host "  2. 触发 Release 工作流（使用 $Version）"
Write-Host "  3. 或推送标签: git push origin $Version"
Write-Host ""
Write-Host "本地安装测试:" -ForegroundColor Cyan
$localExe = Join-Path $buildDir "arch-cli-${Version}-windows-amd64.exe"
if (Test-Path $localExe) {
    Copy-Item $localExe "arch-cli.exe" -Force
    Write-Host "  已复制: .\arch-cli.exe" -ForegroundColor Green
    Write-Host "  测试: .\arch-cli.exe version" -ForegroundColor Gray
}
Write-Host ""

