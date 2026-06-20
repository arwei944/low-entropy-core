#!/usr/bin/env bash
# lec — Low-Entropy Core project scaffolder
# Usage: bash lec.sh init [-t tier] [-m module] [-d desc] <project-name>
# Tiers: l0 (prototype), l1 (microservice), l3 (large-service)

set -euo pipefail

TIER="l0"
MODULE=""
DESC=""
REMOTE=""

# ─── Parse arguments ───
CMD="${1:-}"
if [ "$CMD" != "init" ]; then
    echo ""
    echo "  lec — Low-Entropy Core project scaffolder"
    echo "  Usage: bash lec.sh init [options] <project-name>"
    echo ""
    echo "  Options:"
    echo "    -t <tier>    Target tier: l0 (default), l1, l3"
    echo "    -m <module>  Go module name (e.g. github.com/you/myproject)"
    echo "    -d <desc>    Project description"
    echo "    -r <remote>  Git remote URL"
    echo ""
    echo "  Tiers:"
    echo "    l0  Prototype      (<10 files, scripts, PoC)"
    echo "    l1  Microservice    (10-100 files, single service)"
    echo "    l3  Large Service   (100+ files, distributed)"
    echo ""
    echo "  Examples:"
    echo "    bash lec.sh init myproject"
    echo "    bash lec.sh init -t l1 -m github.com/you/myproject myproject"
    echo "    bash lec.sh init -t l3 -m github.com/org/service -r git@github.com:org/service.git myservice"
    echo ""
    exit 1
fi
shift

while getopts "t:m:d:r:" opt; do
    case $opt in
        t) TIER="$OPTARG" ;;
        m) MODULE="$OPTARG" ;;
        d) DESC="$OPTARG" ;;
        r) REMOTE="$OPTARG" ;;
    esac
done
shift $((OPTIND-1))

NAME="${1:-}"
if [ -z "$NAME" ]; then
    echo "  Error: project name is required"
    exit 1
fi

if [ -z "$MODULE" ]; then
    MODULE="$NAME"
fi

case "$TIER" in
    l0) TIER_LABEL="Prototype"; TIER_NUM=0 ;;
    l1) TIER_LABEL="Microservice"; TIER_NUM=1 ;;
    l3) TIER_LABEL="Large Service"; TIER_NUM=3 ;;
    *) echo "  Error: unknown tier '$TIER'. Use l0, l1, or l3."; exit 1 ;;
esac

if [ -z "$DESC" ]; then
    DESC="A Low-Entropy Core $TIER_LABEL project"
fi

CORE_MODULE="low-entropy-core/go-core"
case "$TIER" in
    l0) CORE_TAG="lecore_tier0" ;;
    l1) CORE_TAG="lecore_tier1" ;;
    l3) CORE_TAG="lecore_tier3" ;;
esac

DIR="./$NAME"
if [ -d "$DIR" ]; then
    echo "  Error: directory '$DIR' already exists"
    exit 1
fi

echo ""
echo "  ⚡ Low-Entropy Core Scaffolder"
echo ""
echo "  Tier:      $TIER ($TIER_LABEL)"
echo "  Project:   $NAME"
echo "  Module:    $MODULE"
echo "  Directory: $DIR"
echo ""

mkdir -p "$DIR"

# ─── go.mod ───
cat > "$DIR/go.mod" << EOF
module $MODULE

go 1.22

require $CORE_MODULE v0.10.0

replace $CORE_MODULE => ../../go-core
EOF
echo "  ✓ go.mod"

# ─── .gitignore ───
cat > "$DIR/.gitignore" << 'EOF'
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
EOF
echo "  ✓ .gitignore"

# ─── CLAUDE.md ───
cat > "$DIR/CLAUDE.md" << CLAUDE_EOF
# $NAME — AI Agent Development Constraints

> **TRAE / Claude / Cursor and other AI agents MUST follow these rules when modifying this project.**

---

## 1. Architecture Tier

This is a **Tier $TIER ($TIER_LABEL)** project using Low-Entropy Core.

EOF

case "$TIER" in
    l0) cat >> "$DIR/CLAUDE.md" << 'CLAUDE_L0'
- Scope: <10 files, prototypes, scripts
- Pipeline-only, no HTTP server
- Build tag: `lecore_tier0`
CLAUDE_L0
    ;;
    l1) cat >> "$DIR/CLAUDE.md" << 'CLAUDE_L1'
- Scope: 10-100 files, single microservice
- Pipeline + HTTP server + Observation
- Build tag: `lecore_tier1`
CLAUDE_L1
    ;;
    l3) cat >> "$DIR/CLAUDE.md" << 'CLAUDE_L3'
- Scope: 100+ files, distributed service
- Full stack: Pipeline + HTTP + Guardian + Observation + EventStore
- Build tag: `lecore_tier3`
CLAUDE_L3
    ;;
esac

cat >> "$DIR/CLAUDE.md" << 'CLAUDE_RULES'

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
EOF

case "$TIER" in
    l0) echo "# Run" >> "$DIR/CLAUDE.md"; echo "go run main.go" >> "$DIR/CLAUDE.md" ;;
    *) echo "# Build and run" >> "$DIR/CLAUDE.md"; echo "go build -tags $CORE_TAG -o server ." >> "$DIR/CLAUDE.md"; echo "./server" >> "$DIR/CLAUDE.md" ;;
esac

cat >> "$DIR/CLAUDE.md" << 'CLAUDE_END'

# Test
go test ./...

# Architecture check
curl http://localhost:8090/api/violations
curl http://localhost:8090/api/primitives
```
CLAUDE_END

echo "  ✓ CLAUDE.md"

# ─── README.md ───
cat > "$DIR/README.md" << README_EOF
# $NAME

$DESC

> Built with [Low-Entropy Core](https://github.com/arwei944/low-entropy-core) — 4-primitive architecture (Atom/Port/Adapter/Composer).

## Quick Start

\`\`\`bash
README_EOF

case "$TIER" in
    l0) echo "go run main.go" >> "$DIR/README.md" ;;
    *) echo "go build -tags $CORE_TAG -o server ." >> "$DIR/README.md"; echo "./server" >> "$DIR/README.md" ;;
esac

cat >> "$DIR/README.md" << README_END
\`\`\`

## Architecture

- **Tier**: $TIER ($TIER_LABEL)
- **Primitives**: Atom / Port / Adapter / Composer
- **Layer**: L$TIER_NUM

## Development

\`\`\`bash
# Test
go test ./...
\`\`\`

## License

MIT
README_END

echo "  ✓ README.md"

# ─── Tier-specific source files ───

if [ "$TIER" = "l0" ]; then
    cat > "$DIR/main.go" << 'L0_MAIN'
// Tier L0 Prototype — Low-Entropy Core
// Build: go run main.go
// All business logic MUST use one of the 4 primitives (Atom/Port/Adapter/Composer).

package main

import (
	"context"
	"fmt"

	core "low-entropy-core/go-core"
)

// ─── Business State ───
type State struct {
	Input  string
	Output string
	OK     bool
	Err    string
}

func main() {
	fmt.Println("=== Low-Entropy Core (Tier L0 Prototype) ===")

	tier := core.AutoDetect(".")
	fmt.Printf("Detected tier: %s (L%d)\n", tier, tier)

	obs := &core.NoOpObservationAdapter{}

	// ─── Pipeline: Port → Atom → Adapter ───
	pipeline := core.NewPipeline[State](obs,

		// Port: Input validation
		core.PortAsStep(core.NewPort[State, State](func(ctx context.Context, input State) (State, error) {
			if input.Input == "" {
				return input, fmt.Errorf("input cannot be empty")
			}
			input.OK = true
			return input, nil
		})),

		// Atom: Pure computation (no side effects)
		core.AtomAsStep(core.Atom[State, State](func(s State) State {
			// TODO: Replace with your business logic
			s.Output = "processed: " + s.Input
			return s
		})),

		// Adapter: Side-effect boundary
		core.AdapterAsStep(core.NewAdapter[State, State](func(ctx context.Context, input State) (State, error) {
			fmt.Printf("  [Adapter] Result: %s\n", input.Output)
			return input, nil
		})),
	)

	ctx := context.Background()
	result, steps, err := pipeline.Run(ctx, State{Input: "hello world"})
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("\nResult: %s\n", result.Output)
	fmt.Printf("Steps: %d\n", len(steps))
	for _, s := range steps {
		fmt.Printf("  [%s/%s] %s (%dms)\n", s.Unit, s.Pattern, s.Action, s.DurationMs)
	}
}
L0_MAIN
    echo "  ✓ main.go"

elif [ "$TIER" = "l1" ]; then
    cat > "$DIR/types.go" << 'L1_TYPES'
// Business Types

package main

type Request struct {
	Data   string            `json:"data"`
	Params map[string]string `json:"params,omitempty"`
}

type Response struct {
	Result  any    `json:"result,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Steps   int    `json:"steps"`
}
L1_TYPES

    cat > "$DIR/main.go" << L1_MAIN
// Tier L1 Microservice — Low-Entropy Core
// Build: go build -tags lecore_tier1 -o server .
// All business logic MUST use one of the 4 primitives (Atom/Port/Adapter/Composer).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "low-entropy-core/go-core"
)

// ─── Port: Validation ───
type RequestPort struct{}

func (p *RequestPort) Validate(ctx context.Context, req Request) (Request, error) {
	if req.Data == "" {
		return req, fmt.Errorf("data field is required")
	}
	return req, nil
}

// ─── Atom: Pure Computation ───
func ProcessAtom() core.Atom[Request, Response] {
	return core.Atom[Request, Response](func(req Request) Response {
		// TODO: Replace with your business logic
		return Response{Result: "processed: " + req.Data, Success: true}
	})
}

// ─── Adapter: Side Effects ───
type LogAdapter struct{}

func (a *LogAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	fmt.Printf("[LogAdapter] result=%s success=%v\n", resp.Result, resp.Success)
	return resp, nil
}

// ─── Pipeline ───
func buildPipeline(obs core.ObservationAdapter) core.Composer[Request] {
	return core.NewPipeline[Request](obs,
		core.PortAsStep(&RequestPort{}),
		core.AdapterAsStep(core.NewAdapter[Request, Response](func(ctx context.Context, req Request) (Response, error) {
			return ProcessAtom()(req), nil
		})),
		core.AdapterAsStep(&LogAdapter{}),
	)
}

func main() {
	fmt.Println("=== Low-Entropy Core (Tier L1 Microservice) ===")

	obs := &core.InMemoryObservationAdapter{}
	pipeline := buildPipeline(obs)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/process", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, steps, err := pipeline.Run(r.Context(), req)
		if err != nil {
			json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
			return
		}
		var resp Response
		if len(steps) > 0 {
			if last := steps[len(steps)-1]; last.Output != nil {
				if r, ok := last.Output.(Response); ok {
					resp = r
				}
			}
		}
		resp.Steps = len(steps)
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","tier":"l1"}`)
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	server := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		server.Shutdown(context.Background())
	}()

	fmt.Printf("Server listening on %s\n", addr)
	server.ListenAndServe()
}
L1_MAIN
    echo "  ✓ types.go"
    echo "  ✓ main.go"

elif [ "$TIER" = "l3" ]; then
    cat > "$DIR/types.go" << 'L3_TYPES'
// Business Types

package main

type Request struct {
	ID     string            `json:"id"`
	Data   string            `json:"data"`
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

type Response struct {
	ID      string `json:"id"`
	Result  any    `json:"result,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Steps   int    `json:"steps"`
}
L3_TYPES

    cat > "$DIR/ports.go" << 'L3_PORTS'
// Port Implementations (Validation Gateways)

package main

import (
	"context"
	"fmt"
)

type RequestPort struct{}

func (p *RequestPort) Validate(ctx context.Context, req Request) (Request, error) {
	if req.Data == "" {
		return req, fmt.Errorf("data field is required")
	}
	if req.Action == "" {
		req.Action = "process"
	}
	if req.ID == "" {
		req.ID = "auto-generated"
	}
	return req, nil
}

type ResponsePort struct{}

func (p *ResponsePort) Validate(ctx context.Context, resp Response) (Response, error) {
	if !resp.Success && resp.Error == "" {
		return resp, fmt.Errorf("failed response must have an error message")
	}
	return resp, nil
}
L3_PORTS

    cat > "$DIR/adapters.go" << 'L3_ADAPTERS'
// Adapter Implementations (Side-Effect Boundaries)

package main

import (
	"context"
	"fmt"
)

type LogAdapter struct{}

func (a *LogAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	fmt.Printf("[LogAdapter] id=%s success=%v\n", resp.ID, resp.Success)
	return resp, nil
}

type PersistAdapter struct{}

func (a *PersistAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	// TODO: Replace with actual persistence (PostgreSQL, Redis, etc.)
	fmt.Printf("[PersistAdapter] id=%s persisted\n", resp.ID)
	return resp, nil
}

type NotifyAdapter struct{}

func (a *NotifyAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	// TODO: Replace with actual notification (email, webhook, etc.)
	fmt.Printf("[NotifyAdapter] id=%s notification sent\n", resp.ID)
	return resp, nil
}
L3_ADAPTERS

    cat > "$DIR/atoms.go" << 'L3_ATOMS'
// Atom Implementations (Pure Computation)

package main

import (
	"strings"

	core "low-entropy-core/go-core"
)

func ProcessDataAtom() core.Atom[Request, Response] {
	return core.Atom[Request, Response](func(req Request) Response {
		// TODO: Replace with your actual business logic
		result := map[string]any{
			"original":  req.Data,
			"processed": strings.ToUpper(req.Data),
			"length":    len(req.Data),
		}
		return Response{ID: req.ID, Result: result, Success: true}
	})
}

func TransformAtom(transform func(string) string) core.Atom[Request, Request] {
	return core.Atom[Request, Request](func(req Request) Request {
		req.Data = transform(req.Data)
		return req
	})
}
L3_ATOMS

    cat > "$DIR/routes.go" << 'L3_ROUTES'
// HTTP Route Handlers

package main

import (
	"context"
	"encoding/json"
	"net/http"

	core "low-entropy-core/go-core"
)

func registerRoutes(mux *http.ServeMux, pipeline core.Composer[Request], obs core.ObservationAdapter) {
	mux.HandleFunc("/api/process", processHandler(pipeline, obs))
	mux.HandleFunc("/api/health", healthHandler())
}

func processHandler(pipeline core.Composer[Request], obs core.ObservationAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, steps, err := pipeline.Run(r.Context(), req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
			return
		}
		var resp Response
		if len(steps) > 0 {
			if last := steps[len(steps)-1]; last.Output != nil {
				if r, ok := last.Output.(Response); ok {
					resp = r
				}
			}
		}
		resp.Steps = len(steps)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "tier": "l3"})
	}
}
L3_ROUTES

    cat > "$DIR/main.go" << L3_MAIN
// Tier L3 Large Service — Low-Entropy Core
// Build: go build -tags lecore_tier3 -o server .
// All business logic MUST use one of the 4 primitives (Atom/Port/Adapter/Composer).

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "low-entropy-core/go-core"
)

func main() {
	fmt.Println("=== Low-Entropy Core (Tier L3 Large Service) ===")

	obs := &core.InMemoryObservationAdapter{}
	pipeline := buildProcessPipeline(obs)

	mux := http.NewServeMux()
	registerRoutes(mux, pipeline, obs)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "tier": "l3", "time": time.Now().Format(time.RFC3339)})
	})
	mux.HandleFunc("/api/observation/steps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(obs.GetSteps())
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	server := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	fmt.Printf("Server listening on %s\n", addr)
	server.ListenAndServe()
}

func buildProcessPipeline(obs core.ObservationAdapter) core.Composer[Request] {
	return core.NewPipeline[Request](obs,
		core.PortAsStep(&RequestPort{}),
		core.AdapterAsStep(core.NewAdapter[Request, Response](func(ctx context.Context, req Request) (Response, error) {
			return ProcessDataAtom()(req), nil
		})),
		core.AdapterAsStep(&PersistAdapter{}),
		core.AdapterAsStep(&LogAdapter{}),
	)
}
L3_MAIN

    echo "  ✓ types.go"
    echo "  ✓ ports.go"
    echo "  ✓ adapters.go"
    echo "  ✓ atoms.go"
    echo "  ✓ routes.go"
    echo "  ✓ main.go"
fi

# ─── Git init ───
echo ""
echo "  Initializing git repository..."
cd "$DIR"
git init > /dev/null 2>&1
git add . > /dev/null 2>&1
git commit -m "init: Low-Entropy Core $TIER_LABEL project" > /dev/null 2>&1

if [ -n "$REMOTE" ]; then
    git remote add origin "$REMOTE" > /dev/null 2>&1
    echo "  ✓ Remote: $REMOTE"
fi

cd - > /dev/null

echo ""
echo "  Project '$NAME' created successfully!"
echo ""
echo "  Quick start:"
echo "    cd $NAME"
if [ "$TIER" = "l0" ]; then
    echo "    go run main.go"
else
    echo "    go build -tags $CORE_TAG -o server ."
    echo "    ./server"
fi
echo ""
