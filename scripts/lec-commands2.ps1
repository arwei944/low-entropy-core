# lec-commands2.ps1 — Commands: version, list, help
# Part of lec — Low-Entropy Core CLI v0.3.0

function Cmd-Version {
    Write-Header "lec v$LEC_VERSION — Low-Entropy Core CLI"
    Write-Host "  A scaffolding and management tool for Low-Entropy Core projects."
    Write-Host "  https://github.com/arwei944/low-entropy-core"
    Write-Host ""
}

function Cmd-List {
    Write-Header "lec-managed projects"

    $found = $false
    Get-ChildItem -Directory -Depth 1 | ForEach-Object {
        $tier = Get-ProjectTier $_.FullName
        if ($tier) {
            $found = $true
            $goFiles = (Get-GoFiles $_.FullName).Count
            $label = Get-TierLabel $tier
            Write-Host ("  {0,-30} Tier {1} ({2})  {3} .go files" -f $_.Name, $tier.ToUpper(), $label, $goFiles)
        }
    }

    if (-not $found) {
        Write-Warn "No lec-managed projects found in current directory."
    }
    Write-Host ""
}

function Cmd-Help {
    Write-Host ""
    Write-Host "  lec — Low-Entropy Core CLI v$LEC_VERSION" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Project Commands:" -ForegroundColor White
    Write-Host "    init <dir> [-Tier l0|l1|l3]          Create new LEC project"
    Write-Host "    add <type> <name> [-Target dir]       Add primitive (atom/port/adapter/composer)"
    Write-Host "    check <dir> [-Detailed]               Check architecture constraints"
    Write-Host "    upgrade <dir> [-Tier t]               Upgrade project tier"
    Write-Host "    list                                  List lec-managed projects"
    Write-Host "    version                               Show version"
    Write-Host ""
    Write-Host "  Migration Commands:" -ForegroundColor White
    Write-Host "    analyze <dir> [-Lang l] [-Detailed]  Parse project source code"
    Write-Host "    pattern <dir> [-GateOnly] [-Threshold] Recognize four-primitive patterns"
    Write-Host "    plan <dir> [-Tier t] [-RiskOnly]      Generate migration plan"
    Write-Host "    migrate <dir> [-DryRun] [-Force]      Execute full migration"
    Write-Host "    log <dir> show|verify|export|replay|stats  Query migration logs"
    Write-Host "    validate <dir> [-Fix]                 Validate architecture constraints"
    Write-Host "    rollback <dir> [-Step id] [-All]       Rollback migration steps"
    Write-Host "    shim <dir> generate|list|remove       Manage shim files"
    Write-Host ""
    Write-Host "  Quick start:" -ForegroundColor Yellow
    Write-Host "    .\lec.ps1 init myproject"
    Write-Host "    .\lec.ps1 init -Tier l1 -Module github.com/you/api api-service"
    Write-Host "    .\lec.ps1 add -Type atom -Name CalculatePrice"
    Write-Host "    .\lec.ps1 check"
    Write-Host "    .\lec.ps1 analyze . -Detailed"
    Write-Host "    .\lec.ps1 migrate . -DryRun"
    Write-Host ""
}
