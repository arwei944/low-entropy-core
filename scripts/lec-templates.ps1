# lec-templates.ps1 — Tier Generators: L0 and L1
# Part of lec — Low-Entropy Core CLI v0.3.0

function Generate-L0 {
    param([string]$Dir, [string]$CoreMod)
    $main = @"
// Tier L0 Prototype — Low-Entropy Core
// Build: go run main.go
// All business logic MUST use one of the 4 primitives (Atom/Port/Adapter/Composer).

package main

import (
	"context"
	"fmt"

	core "$CoreMod"
)

// ─── Business State ───
// This type flows through the entire pipeline.
type State struct {
	Input  string
	Output string
	OK     bool
	Err    string
}

func main() {
	fmt.Println("=== Low-Entropy Core (Tier L0 Prototype) ===")

	// Auto-detect tier
	tier := core.AutoDetect(".")
	fmt.Printf("Detected tier: %s (L%d)\n", tier, tier)

	// Observation adapter (NoOp for zero-overhead in prototype)
	obs := &core.NoOpObservationAdapter{}

	// ─── Build Pipeline: Port → Atom → Adapter ───
	pipeline := core.NewPipeline[State](obs,

		// Port: Input validation (boundary contract)
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

		// Adapter: Side-effect boundary (logging, persistence, external calls)
		core.AdapterAsStep(core.NewAdapter[State, State](func(ctx context.Context, input State) (State, error) {
			fmt.Printf("  [Adapter] Result: %s\n", input.Output)
			return input, nil
		})),
	)

	// ─── Execute ───
	ctx := context.Background()
	result, steps, err := pipeline.Run(ctx, State{Input: "hello world"})
	if err != nil {
		fmt.Printf("Pipeline error: %v\n", err)
		return
	}

	fmt.Printf("\nResult: %s\n", result.Output)
	fmt.Printf("Steps executed: %d\n", len(steps))
	for _, s := range steps {
		fmt.Printf("  [%s/%s] %s (%dms)\n", s.Unit, s.Pattern, s.Action, s.DurationMs)
	}
}
"@
    Write-File -Path "$Dir\main.go" -Content $main
    Write-Ok "main.go"
}

function Generate-L1 {
    param([string]$Dir, [string]$CoreMod, [string]$CoreTag)
    $types = @"
// Business Types — Low-Entropy Core Tier L1

package main

// Request represents an incoming API request.
type Request struct {
	Data   string            ` + "`" + "json:data" + "`" + `
	Params map[string]string ` + "`" + "json:params,omitempty" + "`" + `
}

// Response represents an API response.
type Response struct {
	Result  any    ` + "`" + "json:result,omitempty" + "`" + `
	Success bool   ` + "`" + "json:success" + "`" + `
	Error   string ` + "`" + "json:error,omitempty" + "`" + `
	Steps   int    ` + "`" + "json:steps" + "`" + `
}
"@
    Write-File -Path "$Dir\types.go" -Content $types
    Write-Ok "types.go"

    $main = @"
// Tier L1 Microservice — Low-Entropy Core
// Build: go build -tags $CoreTag -o server .
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

	core "$CoreMod"
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
		fmt.Println("\nShutting down...")
		server.Shutdown(context.Background())
	}()

	fmt.Printf("Server listening on %s\n", addr)
	server.ListenAndServe()
}
"@
    Write-File -Path "$Dir\main.go" -Content $main
    Write-Ok "main.go"
}
