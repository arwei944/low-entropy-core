# {{.Project}} — AI Agent Development Constraints

> **TRAE / Claude / Cursor and other AI agents MUST follow these rules when modifying this project.**

---

## 1. Architecture Tier

This is a **Tier {{.Tier}} ({{.TierLabel}})** project using Low-Entropy Core.

{{- if eq .Tier "l0"}}
- Scope: <10 files, prototypes, scripts
- Pipeline-only, no HTTP server
- Build tag: `lecore_tier0`
{{- else if eq .Tier "l1"}}
- Scope: 10-100 files, single microservice
- Pipeline + HTTP server + Observation
- Build tag: `lecore_tier1`
{{- else}}
- Scope: 100+ files, distributed service
- Full stack: Pipeline + HTTP + Guardian + Observation + EventStore
- Build tag: `lecore_tier3`
{{- end}}

---

## 2. Four Primitives (MANDATORY)

ALL business logic MUST use exactly these four primitives. **No raw func allowed.**

| Primitive | Interface | Purpose |
|-----------|-----------|---------|
| **Atom** | `Atom[In, Out]` | Pure computation, no side effects |
| **Port** | `Port[In, Out]` | Boundary validation (input/output contracts) |
| **Adapter** | `Adapter[In, Out]` | Side-effect boundary (I/O, DB, external API) — ONLY place for side effects |
| **Composer** | `Composer[T]` | Orchestration of multiple Steps (Pipeline, Branch, Parallel) |

### Conversion to Step
- `core.AtomAsStep(atom)` — wrap Atom as Step
- `core.PortAsStep(port)` — wrap Port as Step
- `core.AdapterAsStep(adapter)` — wrap Adapter as Step

---

## 3. Code Rules

- All business logic in `package main` (or sub-packages for L3+)
- File naming: `snake_case.go`
- Exported types/functions: PascalCase
- Tests: `*_test.go` in same directory
- **NEVER** use `fmt.Println` in production code (use Observation Pipeline)
- **NEVER** import higher-layer packages from lower-layer code

---

## 4. Pre-Commit Checklist

Before any code change, AI agents MUST:

1. **Identify the primitive type**: Is this an Atom, Port, Adapter, or Composer?
2. **Check dependency direction**: Are all imports from same or lower layers?
3. **Run tests**: `go test ./...`
4. **Verify no raw func**: Every business function must be wrapped as one of the 4 primitives

---

## 5. Quick Reference

```bash
{{- if eq .Tier "l0"}}
# Run
go run main.go
{{- else}}
# Build and run
go build -tags {{.CoreTag}} -o server .
./server
{{- end}}

# Test
go test ./...

# Architecture check
curl http://localhost:8090/api/violations
curl http://localhost:8090/api/primitives
```
