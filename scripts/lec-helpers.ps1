# lec-helpers.ps1 — Helper Functions
# Part of lec — Low-Entropy Core CLI v0.3.0

function Write-Header {
    param([string]$Text)
    Write-Host ""
    Write-Host "  $Text" -ForegroundColor Cyan
    Write-Host ""
}

function Write-Ok {
    param([string]$Text)
    Write-Host "  [OK] $Text" -ForegroundColor Green
}

function Write-Err {
    param([string]$Text)
    Write-Host "  [ERR] $Text" -ForegroundColor Red
}

function Write-Warn {
    param([string]$Text)
    Write-Host "  [WARN] $Text" -ForegroundColor Yellow
}

function Write-Info {
    param([string]$Text)
    Write-Host "  [INFO] $Text" -ForegroundColor Gray
}

function Write-File {
    param([string]$Path, [string]$Content)
    $dir = Split-Path -Parent $Path
    if ($dir -and -not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    # Write without BOM
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText((Resolve-Path -Path ".").Path + "\" + $Path, $Content, $utf8NoBom)
}

function Get-TierLabel {
    param([string]$T)
    switch ($T) {
        "l0" { return "Prototype" }
        "l1" { return "Microservice" }
        "l3" { return "Large Service" }
        default { return "Unknown" }
    }
}

function Get-TierNum {
    param([string]$T)
    switch ($T) {
        "l0" { return 0 }
        "l1" { return 1 }
        "l3" { return 3 }
        default { return 0 }
    }
}

function Get-CoreTag {
    param([string]$T)
    switch ($T) {
        "l0" { return "lecore_tier0" }
        "l1" { return "lecore_tier1" }
        "l3" { return "lecore_tier3" }
        default { return "lecore_tier0" }
    }
}

function Get-CoreModule {
    param([string]$Path)
    if ($Path) {
        return $Path
    }
    return "low-entropy-core/go-core"
}

function Get-ProjectTier {
    param([string]$Dir)
    $goMod = Join-Path $Dir "go.mod"
    if (-not (Test-Path $goMod)) { return $null }
    $content = Get-Content $goMod -Raw
    if ($content -match "lecore_tier0") { return "l0" }
    if ($content -match "lecore_tier1") { return "l1" }
    if ($content -match "lecore_tier3") { return "l3" }
    # Check CLAUDE.md
    $claude = Join-Path $Dir "CLAUDE.md"
    if (Test-Path $claude) {
        $c = Get-Content $claude -Raw
        if ($c -match "Tier L0") { return "l0" }
        if ($c -match "Tier L1") { return "l1" }
        if ($c -match "Tier L3") { return "l3" }
    }
    return $null
}

function Get-GoFiles {
    param([string]$Dir)
    return Get-ChildItem -Path $Dir -Filter "*.go" -Recurse -File |
        Where-Object { $_.FullName -notmatch "[\\/]go-core[\\/]" } |
        Select-Object -ExpandProperty FullName
}

function Resolve-CorePath {
    param([string]$Dir)
    $goMod = Join-Path $Dir "go.mod"
    if (Test-Path $goMod) {
        $content = Get-Content $goMod -Raw
        if ($content -match "replace\s+\S+\s+=>\s+(\S+)") {
            return $Matches[1]
        }
    }
    return ""
}
