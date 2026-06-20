# lec-migrate.ps1 — Migration Commands
# Part of lec — Low-Entropy Core CLI v0.3.0

function Cmd-Analyze {
    param(
        [string]$ProjectDir = "",
        [string]$Lang = "auto",
        [string]$Output = "text",
        [switch]$Detailed
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec analyze <project-dir> [-Lang auto|go|python|java|ts|rust] [-Output json|text] [-Detailed]"; return }
    $resolvedDir = Resolve-Path $ProjectDir -ErrorAction SilentlyContinue
    if (-not $resolvedDir) { Write-Host "[ERROR] Directory not found: $ProjectDir" -ForegroundColor Red; return }
    if ($Lang -eq "auto") {
        $goMod = Join-Path $resolvedDir "go.mod"
        $reqTxt = Join-Path $resolvedDir "requirements.txt"
        $pomXml = Join-Path $resolvedDir "pom.xml"
        $pkgJson = Join-Path $resolvedDir "package.json"
        $cargoToml = Join-Path $resolvedDir "Cargo.toml"
        if (Test-Path $goMod) { $Lang = "go" }
        elseif (Test-Path $reqTxt) { $Lang = "python" }
        elseif (Test-Path $pomXml) { $Lang = "java" }
        elseif ((Test-Path $pkgJson) -and (Test-Path (Join-Path $resolvedDir "tsconfig.json"))) { $Lang = "typescript" }
        elseif (Test-Path $cargoToml) { $Lang = "rust" }
        else { $Lang = "unknown" }
    }
    Write-Header "LEC Analyze"
    Write-Info "Project : $resolvedDir"
    Write-Info "Language: $Lang"
    Write-Host ""
    $extMap = @{ go=".go"; python=".py"; java=".java"; typescript=".ts"; rust=".rs" }
    $ext = $extMap[$Lang]
    $files = @()
    if ($ext) {
        $files = Get-ChildItem -Path $resolvedDir -Recurse -Filter "*$ext" -File |
            Where-Object { $_.FullName -notmatch "\\vendor\\" -and $_.FullName -notmatch "\\node_modules\\" -and $_.FullName -notmatch "\\.lec\\" }
    }
    Write-Host "  Files found: $($files.Count)"
    if ($Detailed) {
        foreach ($f in $files) {
            $rel = $f.FullName.Substring($resolvedDir.Path.Length + 1)
            $lines = (Get-Content $f.FullName).Count
            Write-Host "    $rel ($lines lines)"
        }
    }
    Write-Host ""
    Write-Warn "Full analysis requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: file-system scan (language detection + file listing)"
}

function Cmd-Pattern {
    param(
        [string]$ProjectDir = "",
        [string]$Output = "text",
        [switch]$Detailed,
        [switch]$GateOnly,
        [double]$Threshold = 0.4
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec pattern <project-dir> [-Output json|text] [-Detailed] [-GateOnly] [-Threshold 0.4]"; return }
    Write-Header "LEC Pattern Recognition"
    Write-Info "Project: $ProjectDir"
    Write-Host ""
    Write-Warn "Pattern recognition requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: file-system scan (placeholder)"
}

function Cmd-Plan {
    param(
        [string]$ProjectDir = "",
        [string]$Tier = "auto",
        [switch]$RiskOnly,
        [switch]$Detailed
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec plan <project-dir> [-Tier l0|l1|l3] [-RiskOnly] [-Detailed]"; return }
    Write-Header "LEC Migration Plan"
    Write-Info "Project: $ProjectDir"
    Write-Info "Target Tier: $Tier"
    Write-Host ""
    Write-Warn "Plan generation requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: file-system scan (placeholder)"
}

function Cmd-Migrate {
    param(
        [string]$ProjectDir = "",
        [string]$Tier = "auto",
        [string]$Lang = "auto",
        [string]$CorePath = "",
        [switch]$DryRun,
        [switch]$Force,
        [string]$Step = "all",
        [string]$Only = "all",
        [string]$Skip = "none",
        [switch]$Detailed
    )
    if (-not $ProjectDir) {
        Write-Host "Usage: lec migrate <project-dir> [-Tier l0|l1|l3] [-Lang auto|go|python|java|ts|rust] [-DryRun] [-Force] [-Only phase] [-Skip phase]"
        return
    }
    Write-Header "LEC Migrate"
    Write-Info "Project : $ProjectDir"
    Write-Info "Tier    : $Tier"
    Write-Info "Lang    : $Lang"
    Write-Info "DryRun  : $DryRun"
    Write-Host ""
    if ($DryRun) { Write-Warn "DRY RUN - No files will be modified" }
    Write-Info "Phase 1: Parsing source code..."
    Write-Info "Phase 2: Recognizing patterns..."
    Write-Info "Phase 3: Running constraint gates..."
    Write-Info "Phase 4: Generating migration plan..."
    Write-Info "Phase 5: Transforming code..."
    Write-Info "Phase 6: Generating shims..."
    Write-Info "Phase 7: Validating..."
    Write-Info "Phase 8: Writing atomic logs..."
    Write-Host ""
    Write-Warn "Full migration requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: dry-run placeholder"
}

function Cmd-Log {
    param(
        [string]$ProjectDir = "",
        [string]$SubCommand = "show",
        [string]$Phase = "",
        [string]$File = "",
        [string]$Step = "",
        [int]$Last = 0,
        [string]$Format = "text",
        [string]$OutputFormat = "text"
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec log <project-dir> show|verify|export|replay|stats"; return }
    $logDir = Join-Path $ProjectDir ".lec\migration"
    if (-not (Test-Path $logDir)) {
        Write-Warn "No migration log found at: $logDir"
        return
    }
    switch ($SubCommand) {
        "show" {
            Write-Header "LEC Migration Log"
            Get-ChildItem -Path $logDir -Filter "*.log" | ForEach-Object {
                Write-Host ("  {0} ({1} KB)" -f $_.Name, [math]::Round($_.Length/1KB, 1))
            }
        }
        "verify" {
            Write-Header "LEC Log Verify"
            Write-Warn "Integrity verification requires compiled Go binary"
        }
        "export" {
            Write-Header "LEC Log Export"
            Write-Warn "Export requires compiled Go binary"
        }
        "replay" {
            Write-Header "LEC Log Replay"
            Write-Warn "Replay requires compiled Go binary"
        }
        "stats" {
            Write-Header "LEC Log Stats"
            $logFiles = Get-ChildItem -Path $logDir -Filter "*.log"
            $totalSize = ($logFiles | Measure-Object -Property Length -Sum).Sum
            Write-Host ("  Sessions: {0}" -f $logFiles.Count)
            Write-Host ("  Total size: {0} KB" -f [math]::Round($totalSize/1KB, 1))
        }
        default { Write-Host "Unknown subcommand: $SubCommand" }
    }
}

function Cmd-Validate {
    param(
        [string]$ProjectDir = "",
        [switch]$Detailed,
        [switch]$Fix
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec validate <project-dir> [-Detailed] [-Fix]"; return }
    Write-Header "LEC Validate"
    Write-Info "Project: $ProjectDir"
    Write-Host ""
    Write-Warn "Validation requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: file-system scan (placeholder)"
}

function Cmd-Rollback {
    param(
        [string]$ProjectDir = "",
        [string]$Step = "last",
        [switch]$All,
        [switch]$DryRun
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec rollback <project-dir> [-Step id|last] [-All] [-DryRun]"; return }
    Write-Header "LEC Rollback"
    Write-Info "Project: $ProjectDir"
    Write-Info "Step   : $Step"
    Write-Host ""
    if ($DryRun) { Write-Warn "DRY RUN - No files will be modified" }
    Write-Warn "Rollback requires compiled Go binary (go-core/migrate)"
    Write-Info "Current mode: file-system scan (placeholder)"
}

function Cmd-Shim {
    param(
        [string]$ProjectDir = "",
        [string]$SubCommand = "list",
        [string]$Strategy = "wrapper"
    )
    if (-not $ProjectDir) { Write-Host "Usage: lec shim <project-dir> generate|list|remove [-Strategy wrapper|delegate|facade]"; return }
    Write-Header "LEC Shim Manager"
    Write-Info "Project: $ProjectDir"
    Write-Host ""
    switch ($SubCommand) {
        "generate" {
            Write-Info "Strategy: $Strategy"
            Write-Warn "Shim generation requires compiled Go binary"
        }
        "list" {
            $shimDir = Join-Path $ProjectDir ".lec\shims"
            if (Test-Path $shimDir) {
                Get-ChildItem -Path $shimDir -Recurse -File | ForEach-Object {
                    $rel = $_.FullName.Substring((Resolve-Path $ProjectDir).Path.Length + 1)
                    Write-Host "  $rel"
                }
            } else {
                Write-Info "No shim files found"
            }
        }
        "remove" {
            Write-Warn "Shim removal requires compiled Go binary"
        }
        default { Write-Host "Unknown subcommand: $SubCommand" }
    }
}
