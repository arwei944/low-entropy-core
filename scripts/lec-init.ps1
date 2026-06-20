# lec-init.ps1 — Command: init
# Part of lec — Low-Entropy Core CLI v0.3.0

function Cmd-Init {
    if ($RestArgs.Count -lt 1) {
        Write-Err "Project name is required."
        Write-Host ""
        Write-Host "  Usage: .\lec.ps1 init [-Tier l0|l1|l3] [-Module mod] [-Desc desc] [-Remote url] [-CorePath path] <name>"
        Write-Host ""
        exit 1
    }

    $projName = $RestArgs[0]
    if (-not $Module) { $Module = $projName }

    # Validate tier
    $validTiers = @("l0", "l1", "l3")
    if ($validTiers -notcontains $Tier) {
        Write-Err "Unknown tier '$Tier'. Valid: l0, l1, l3"
        exit 1
    }

    $tierLabel = Get-TierLabel $Tier
    $tierNum = Get-TierNum $Tier
    $coreTag = Get-CoreTag $Tier
    $coreMod = Get-CoreModule $CorePath

    if (-not $Desc) { $Desc = "A Low-Entropy Core $tierLabel project" }

    $dir = ".\$projName"
    if (Test-Path $dir) {
        Write-Err "Directory '$dir' already exists."
        exit 1
    }

    Write-Header "Low-Entropy Core Scaffolder v$LEC_VERSION"
    Write-Info "Tier:      $Tier ($tierLabel)"
    Write-Info "Project:   $projName"
    Write-Info "Module:    $Module"
    Write-Info "Core:      $coreMod"
    Write-Info "Directory: $dir"
    Write-Host ""

    New-Item -ItemType Directory -Path $dir -Force | Out-Null

    # ─── go.mod ───
    $goModContent = "module $Module`n`ngo 1.22`n`nrequire $coreMod v0.10.0`n`nreplace $coreMod => ../../go-core`n"
    Write-File -Path "$dir\go.mod" -Content $goModContent
    Write-Ok "go.mod"

    # ─── .gitignore ───
    $gitignore = @"
*.exe
*.exe~
*.dll
*.so
*.dylib
/server
/bin/
*.test
*.out
coverage.txt
.idea/
.vscode/
*.swp
*.swo
.DS_Store
Thumbs.db
/data/
*.db
*.sqlite
"@
    Write-File -Path "$dir\.gitignore" -Content $gitignore
    Write-Ok ".gitignore"

    # ─── CLAUDE.md ───
    $claude = @"
# $projName — AI Agent Development Constraints

> **TRAE / Claude / Cursor and other AI agents MUST follow these rules when modifying this project.**

---

## 1. Architecture Tier

This is a **Tier $Tier ($tierLabel)** project using Low-Entropy Core.

"@
    switch ($Tier) {
        "l0" { $claude += "- Scope: <10 files, prototypes, scripts`n- Pipeline-only, no HTTP server`n- Build tag: ``lecore_tier0```n" }
        "l1" { $claude += "- Scope: 10-100 files, single microservice`n- Pipeline + HTTP server + Observation`n- Build tag: ``lecore_tier1```n" }
        "l3" { $claude += "- Scope: 100+ files, distributed service`n- Full stack: Pipeline + HTTP + Guardian + Observation + EventStore`n- Build tag: ``lecore_tier3```n" }
    }
    $claude += @"

---

## 2. Four Primitives (MANDATORY)

ALL business logic MUST use exactly these four primitives. **No raw func allowed.**

| Primitive | Interface | Purpose |
|-----------|-----------|---------|
| **Atom** | ``Atom[In, Out]`` | Pure computation, no side effects |
| **Port** | ``Port[In, Out]`` | Boundary validation (input/output contracts) |
| **Adapter** | ``Adapter[In, Out]`` | Side-effect boundary (I/O, DB, external API) — ONLY place for side effects |
| **Composer** | ``Composer[T]`` | Orchestration of multiple Steps (Pipeline, Branch, Parallel) |

### Conversion to Step
- ``core.AtomAsStep(atom)`` — wrap Atom as Step
- ``core.PortAsStep(port)`` — wrap Port as Step
- ``core.AdapterAsStep(adapter)`` — wrap Adapter as Step

---

## 3. Code Rules

- All business logic in ``package main`` (or sub-packages for L3+)
- File naming: ``snake_case.go``
- Exported types/functions: PascalCase
- Tests: ``*_test.go`` in same directory
- **NEVER** use ``fmt.Println`` in production code (use Observation Pipeline)
- **NEVER** import higher-layer packages from lower-layer code

---

## 4. Pre-Commit Checklist

Before any code change, AI agents MUST:

1. **Identify the primitive type**: Is this an Atom, Port, Adapter, or Composer?
2. **Check dependency direction**: Are all imports from same or lower layers?
3. **Run tests**: ``go test ./...``
4. **Verify no raw func**: Every business function must be wrapped as one of the 4 primitives

---

## 5. Quick Reference

``````bash
"@
    switch ($Tier) {
        "l0" { $claude += "go run main.go`n" }
        default { $claude += "go build -tags $coreTag -o server .`n./server`n" }
    }
    $claude += @"
``````
"@
    Write-File -Path "$dir\CLAUDE.md" -Content $claude
    Write-Ok "CLAUDE.md"

    # ─── README.md ───
    $readme = @"
# $projName

$Desc

> Built with [Low-Entropy Core](https://github.com/arwei944/low-entropy-core) — 4-primitive architecture (Atom/Port/Adapter/Composer).

## Quick Start

``````bash
"@
    switch ($Tier) {
        "l0" { $readme += "go run main.go`n" }
        default { $readme += "go build -tags $coreTag -o server .`n./server`n" }
    }
    $readme += @"
``````

## Architecture

- **Tier**: $Tier ($tierLabel)
- **Primitives**: Atom / Port / Adapter / Composer
- **Layer**: L$tierNum

## Development

``````bash
go test ./...
``````

## License

MIT
"@
    Write-File -Path "$dir\README.md" -Content $readme
    Write-Ok "README.md"

    # ─── Source files by tier ───
    if ($Tier -eq "l0") {
        Generate-L0 $dir $coreMod
    } elseif ($Tier -eq "l1") {
        Generate-L1 $dir $coreMod $coreTag
    } elseif ($Tier -eq "l3") {
        Generate-L3 $dir $coreMod $coreTag
    }

    # ─── Git init ───
    Write-Host ""
    Write-Info "Initializing git repository..."
    Push-Location $dir
    git init 2>&1 | Out-Null
    git add . 2>&1 | Out-Null
    git commit -m "init: Low-Entropy Core $tierLabel project" 2>&1 | Out-Null
    if ($Remote) {
        git remote add origin $Remote 2>&1 | Out-Null
        Write-Ok "Remote: $Remote"
    }
    Pop-Location

    Write-Host ""
    Write-Host "  Project '$projName' created successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Quick start:" -ForegroundColor Yellow
    Write-Host "    cd $projName"
    if ($Tier -eq "l0") {
        Write-Host "    go run main.go"
    } else {
        Write-Host "    go build -tags $coreTag -o server ."
        Write-Host "    ./server"
    }
    Write-Host ""
}
