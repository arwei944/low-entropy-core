# lec-commands.ps1 — Commands: add, check, upgrade
# Part of lec — Low-Entropy Core CLI v0.3.0

function Cmd-Add {
    $validTypes = @("atom", "port", "adapter", "composer")
    if ($validTypes -notcontains $Type) {
        Write-Err "Type is required. Use: -Type atom|port|adapter|composer"
        Write-Host ""
        Write-Host "  Usage: .\lec.ps1 add -Type <atom|port|adapter|composer> -Name <PascalCase> [-Target dir]"
        Write-Host ""
        Write-Host "  Examples:"
        Write-Host "    .\lec.ps1 add -Type atom -Name CalculatePrice"
        Write-Host "    .\lec.ps1 add -Type port -Name ValidateOrder -Target ./myproject"
        Write-Host "    .\lec.ps1 add -Type adapter -Name SendEmail"
        Write-Host "    .\lec.ps1 add -Type composer -Name OrderPipeline"
        Write-Host ""
        exit 1
    }

    if (-not $Name) {
        Write-Err "Name is required. Use: -Name <PascalCase> (e.g. CalculatePrice)"
        exit 1
    }

    $targetDir = if ($Target) { $Target } else { "." }
    if (-not (Test-Path (Join-Path $targetDir "go.mod"))) {
        Write-Err "Not a Go module (no go.mod found in '$targetDir')."
        exit 1
    }

    $tier = Get-ProjectTier $targetDir
    if (-not $tier) {
        Write-Warn "Cannot detect tier. Assuming L0."
        $tier = "l0"
    }

    $coreMod = Resolve-CorePath $targetDir
    if (-not $coreMod) { $coreMod = "low-entropy-core/go-core" }

    # Determine filename
    $fileName = $Name.ToLower() + ".go"
    $filePath = Join-Path $targetDir $fileName

    if (Test-Path $filePath) {
        Write-Err "File '$fileName' already exists."
        exit 1
    }

    Write-Header "Adding ${Type}: $Name"

    $content = ""

    switch ($Type) {
        "atom" {
            $content = Generate-AtomSnippet $Name $coreMod
        }
        "port" {
            $content = Generate-PortSnippet $Name $coreMod
        }
        "adapter" {
            $content = Generate-AdapterSnippet $Name $coreMod
        }
        "composer" {
            $content = Generate-ComposerSnippet $Name $coreMod
        }
    }

    Write-File -Path $filePath -Content $content
    Write-Ok "$fileName"

    Write-Host ""
    Write-Host "  Usage in pipeline:" -ForegroundColor Yellow
    switch ($Type) {
        "atom"   { Write-Host "    core.AtomAsStep($Name())" }
        "port"   { Write-Host "    core.PortAsStep(&$Name{})" }
        "adapter" { Write-Host "    core.AdapterAsStep(&$Name{})" }
        "composer" { Write-Host "    // Use $Name() as a sub-pipeline or branch" }
    }
    Write-Host ""
}

function Cmd-Check {
    $targetDir = if ($Target) { $Target } else { "." }
    $goMod = Join-Path $targetDir "go.mod"
    if (-not (Test-Path $goMod)) {
        Write-Err "Not a Go module (no go.mod found in '$targetDir')."
        exit 1
    }

    $tier = Get-ProjectTier $targetDir
    if (-not $tier) {
        Write-Warn "Cannot detect tier. Running basic checks."
    }

    Write-Header "Low-Entropy Core Architecture Check"

    $goFiles = Get-GoFiles $targetDir
    $totalViolations = 0
    $totalFiles = $goFiles.Count
    $primitiveCounts = @{Atom=0; Port=0; Adapter=0; Composer=0}

    foreach ($file in $goFiles) {
        $content = Get-Content $file -Raw
        $relPath = $file.Substring((Get-Item $targetDir).FullName.Length + 1)
        $fileViolations = 0

        # Check for CLAUDE.md
        # Check for raw func in business code (heuristic)
        if ($Detailed) {
            Write-Info "Checking: $relPath"
        }

        # Count primitives
        if ($content -match "core\.Atom\[") { $primitiveCounts.Atom++ }
        if ($content -match "core\.NewPort\[") { $primitiveCounts.Port++ }
        if ($content -match "core\.NewAdapter\[") { $primitiveCounts.Adapter++ }
        if ($content -match "core\.NewPipeline\[") { $primitiveCounts.Composer++ }

        # Check for AsStep usage
        if ($content -match "func\s+\w+\s*\(" -and $content -notmatch "core\.(AtomAsStep|PortAsStep|AdapterAsStep|NewPort|NewAdapter|NewPipeline)" -and $content -match "package main") {
            # Only warn for main package files that have functions but no primitive references
            $funcMatches = [regex]::Matches($content, "func\s+(?:New\w+|Create\w+|Build\w+|Handle\w+|Process\w+)\s*\(")
            foreach ($m in $funcMatches) {
                $fileViolations++
                if ($Detailed) {
                    Write-Warn "$relPath : possible raw func '$($m.Value)' (should be wrapped as Atom/Port/Adapter)"
                }
            }
        }

        # Check for fmt.Println in non-main files (should use Observation)
        if ($content -match "fmt\.Println" -and $content -notmatch "func main\(\)") {
            $fileViolations++
            if ($Detailed) {
                Write-Warn "$relPath : uses fmt.Println outside main() (should use Observation Pipeline)"
            }
        }

        $totalViolations += $fileViolations
    }

    # Report
    Write-Host "  Files scanned: $totalFiles" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Primitives found:" -ForegroundColor Gray
    Write-Host ("    Atoms:     {0}" -f $primitiveCounts.Atom)
    Write-Host ("    Ports:     {0}" -f $primitiveCounts.Port)
    Write-Host ("    Adapters:  {0}" -f $primitiveCounts.Adapter)
    Write-Host ("    Composers: {0}" -f $primitiveCounts.Composer)
    Write-Host ""

    if ($totalViolations -eq 0) {
        Write-Host "  PASSED — No violations found." -ForegroundColor Green
    } else {
        Write-Host "  FOUND $totalViolations potential violation(s)." -ForegroundColor Yellow
        if (-not $Detailed) {
            Write-Host "  Run with -Detailed for details." -ForegroundColor Gray
        }
    }

    # Check CLAUDE.md exists
    $claude = Join-Path $targetDir "CLAUDE.md"
    if (Test-Path $claude) {
        Write-Ok "CLAUDE.md present"
    } else {
        Write-Warn "CLAUDE.md missing (AI agents won't know the rules)"
    }

    Write-Host ""
}

function Cmd-Upgrade {
    $targetDir = if ($Target) { $Target } else { "." }
    $goMod = Join-Path $targetDir "go.mod"
    if (-not (Test-Path $goMod)) {
        Write-Err "Not a Go module (no go.mod found in '$targetDir')."
        exit 1
    }

    $currentTier = Get-ProjectTier $targetDir
    if (-not $currentTier) {
        Write-Err "Cannot detect current tier."
        exit 1
    }

    # Determine target tier
    $targetTier = ""
    if ($RestArgs.Count -gt 0) {
        $targetTier = $RestArgs[0]
    } else {
        switch ($currentTier) {
            "l0" { $targetTier = "l1" }
            "l1" { $targetTier = "l3" }
            "l3" { Write-Err "Already at highest tier (L3)." ; exit 1 }
        }
    }

    $validUpgrade = @(
        @{from="l0"; to="l1"},
        @{from="l0"; to="l3"},
        @{from="l1"; to="l3"}
    )

    $isValid = $false
    foreach ($u in $validUpgrade) {
        if ($u.from -eq $currentTier -and $u.to -eq $targetTier) {
            $isValid = $true
            break
        }
    }

    if (-not $isValid) {
        Write-Err "Cannot upgrade from $currentTier to $targetTier."
        Write-Host "  Valid upgrades: l0->l1, l0->l3, l1->l3"
        exit 1
    }

    $currentLabel = Get-TierLabel $currentTier
    $targetLabel = Get-TierLabel $targetTier
    $coreTag = Get-CoreTag $targetTier

    Write-Header "Upgrading: $currentLabel ($currentTier) -> $targetLabel ($targetTier)"

    $coreMod = Resolve-CorePath $targetDir
    if (-not $coreMod) { $coreMod = "low-entropy-core/go-core" }

    # Add new files based on target tier
    if ($targetTier -eq "l1" -or $targetTier -eq "l3") {
        # Add types.go if not exists
        $typesPath = Join-Path $targetDir "types.go"
        if (-not (Test-Path $typesPath)) {
            $types = @"
// Business Types

package main

type Request struct {
	Data   string            ` + "`" + "json:data" + "`" + `
	Params map[string]string ` + "`" + "json:params,omitempty" + "`" + `
}

type Response struct {
	Result  any    ` + "`" + "json:result,omitempty" + "`" + `
	Success bool   ` + "`" + "json:success" + "`" + `
	Error   string ` + "`" + "json:error,omitempty" + "`" + `
	Steps   int    ` + "`" + "json:steps" + "`" + `
}
"@
            Write-File -Path $typesPath -Content $types
            Write-Ok "types.go (new)"
        }
    }

    if ($targetTier -eq "l3") {
        # Add ports.go, adapters.go, atoms.go, routes.go if not exist
        foreach ($f in @("ports.go", "adapters.go", "atoms.go", "routes.go")) {
            $p = Join-Path $targetDir $f
            if (-not (Test-Path $p)) {
                $stubContent = "// $f — Low-Entropy Core Tier L3`n// TODO: Implement primitives here`n`npackage main`n"
                Write-File -Path $p -Content $stubContent
                Write-Ok "$f (new)"
            }
        }
    }

    # Update CLAUDE.md
    $claudePath = Join-Path $targetDir "CLAUDE.md"
    if (Test-Path $claudePath) {
        $claudeContent = Get-Content $claudePath -Raw
        $claudeContent = $claudeContent -replace "Tier $currentTier", "Tier $targetTier"
        $claudeContent = $claudeContent -replace "Tier L\d", "Tier L$(Get-TierNum $targetTier)"
        $claudeContent = $claudeContent -replace "lecore_tier\d", $coreTag
        $claudeContent = $claudeContent -replace $currentLabel, $targetLabel
        Write-File -Path $claudePath -Content $claudeContent
        Write-Ok "CLAUDE.md (updated)"
    }

    # Update README.md
    $readmePath = Join-Path $targetDir "README.md"
    if (Test-Path $readmePath) {
        $readmeContent = Get-Content $readmePath -Raw
        $readmeContent = $readmeContent -replace "Tier: $currentTier", "Tier: $targetTier"
        $readmeContent = $readmeContent -replace $currentLabel, $targetLabel
        $readmeContent = $readmeContent -replace "L$(Get-TierNum $currentTier)", "L$(Get-TierNum $targetTier)"
        Write-File -Path $readmePath -Content $readmeContent
        Write-Ok "README.md (updated)"
    }

    Write-Host ""
    Write-Host "  Upgrade complete: $currentTier -> $targetTier" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Next steps:" -ForegroundColor Yellow
    Write-Host "    1. Review and update main.go to use the new primitives"
    Write-Host "    2. Fill in the stub files with your business logic"
    Write-Host "    3. Run: go build -tags $coreTag -o server ."
    Write-Host ""
}
