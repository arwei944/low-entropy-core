# lec-templates-l3.ps1 — Tier Generator: L3
# Part of lec — Low-Entropy Core CLI v0.3.0

function Generate-L3 {
    param([string]$Dir, [string]$CoreMod, [string]$CoreTag)
    $types = @"
// Business Types — Low-Entropy Core Tier L3

package main

type Request struct {
	ID     string            ` + "`" + "json:id" + "`" + `
	Data   string            ` + "`" + "json:data" + "`" + `
	Action string            ` + "`" + "json:action" + "`" + `
	Params map[string]string ` + "`" + "json:params,omitempty" + "`" + `
}

type Response struct {
	ID      string ` + "`" + "json:id" + "`" + `
	Result  any    ` + "`" + "json:result,omitempty" + "`" + `
	Success bool   ` + "`" + "json:success" + "`" + `
	Error   string ` + "`" + "json:error,omitempty" + "`" + `
	Steps   int    ` + "`" + "json:steps" + "`" + `
}
"@
    Write-File -Path "$Dir\types.go" -Content $types
    Write-Ok "types.go"

    $ports = @"
// Port Implementations (Validation Gateways)
// Ports sit at system boundaries and validate input/output contracts.

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
"@
    Write-File -Path "$Dir\ports.go" -Content $ports
    Write-Ok "ports.go"

    $adapters = @"
// Adapter Implementations (Side-Effect Boundaries)
// Adapters are the ONLY place where side effects are allowed.

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
"@
    Write-File -Path "$Dir\adapters.go" -Content $adapters
    Write-Ok "adapters.go"

    $atoms = @"
// Atom Implementations (Pure Computation)
// Atoms are pure functions with NO side effects.

package main

import (
	"strings"

	core "$CoreMod"
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
"@
    Write-File -Path "$Dir\atoms.go" -Content $atoms
    Write-Ok "atoms.go"

    $routes = @"
// HTTP Route Handlers
// Routes connect HTTP endpoints to Pipeline execution.

package main

import (
	"encoding/json"
	"net/http"

	core "$CoreMod"
)

func registerRoutes(mux *http.ServeMux, pipeline core.Composer[Request], obs core.ObservationAdapter) {
	mux.HandleFunc("/api/process", processHandler(pipeline))
	mux.HandleFunc("/api/health", healthHandler())
	mux.HandleFunc("/api/observation/steps", observationHandler(obs))
}

func processHandler(pipeline core.Composer[Request]) http.HandlerFunc {
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

func observationHandler(obs core.ObservationAdapter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(obs.GetSteps())
	}
}
"@
    Write-File -Path "$Dir\routes.go" -Content $routes
    Write-Ok "routes.go"

    $main = @"
// Tier L3 Large Service — Low-Entropy Core
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

func main() {
	fmt.Println("=== Low-Entropy Core (Tier L3 Large Service) ===")

	obs := &core.InMemoryObservationAdapter{}
	pipeline := buildProcessPipeline(obs)

	mux := http.NewServeMux()
	registerRoutes(mux, pipeline, obs)

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
"@
    Write-File -Path "$Dir\main.go" -Content $main
    Write-Ok "main.go"
}
