# {{.Project}}

{{.Desc}}

> Built with [Low-Entropy Core](https://github.com/arwei944/low-entropy-core) — 4-primitive architecture (Atom/Port/Adapter/Composer).

## Quick Start

```bash
{{- if eq .Tier "l0"}}
go run main.go
{{- else}}
go build -tags {{.CoreTag}} -o server .
./server
{{- end}}
```

## Architecture

- **Tier**: {{.Tier}} ({{.TierLabel}})
- **Primitives**: Atom / Port / Adapter / Composer
- **Layer**: L{{.TierNum}}

## Project Structure

{{- if eq .Tier "l0"}}
```
{{.Project}}/
├── main.go       # Entry point + Pipeline
├── go.mod
├── CLAUDE.md     # AI agent development constraints
└── README.md
```
{{- else if eq .Tier "l1"}}
```
{{.Project}}/
├── main.go       # Entry point + HTTP server
├── types.go      # Business types
├── go.mod
├── CLAUDE.md     # AI agent development constraints
└── README.md
```
{{- else}}
```
{{.Project}}/
├── main.go       # Entry point + HTTP server
├── types.go      # Business types
├── atoms.go      # Atom implementations (pure computation)
├── ports.go      # Port implementations (validation)
├── adapters.go   # Adapter implementations (side effects)
├── routes.go     # HTTP route handlers
├── go.mod
├── CLAUDE.md     # AI agent development constraints
└── README.md
```
{{- end}}

## Development

```bash
# Run architecture check (requires arch-manager)
go run github.com/arwei944/low-entropy-core/cmd/arch-manager

# Run tests
go test ./...
```

## License

MIT
