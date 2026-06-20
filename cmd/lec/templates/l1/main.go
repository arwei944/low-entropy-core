// {{.Project}} — Tier L1 Microservice
// Build: go build -tags lecore_tier1 -o server .
// Tier: L1 (Microservice) — 10-100 files, single service
//
// Low-Entropy Core: 4-primitive architecture (Atom/Port/Adapter/Composer)
// All business logic MUST use one of the 4 primitives. No raw func allowed.

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

	core "{{.CoreModule}}"
)

// ─── Business Types ───
// Define your domain types here. They flow through the pipeline.

type Request struct {
	Data string `json:"data"`
}

type Response struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Steps   int    `json:"steps"`
}

// ─── Ports (Validation) ───

type RequestPort struct{}

func (p *RequestPort) Validate(ctx context.Context, req Request) (Request, error) {
	if req.Data == "" {
		return req, fmt.Errorf("data field is required")
	}
	return req, nil
}

// ─── Atoms (Pure Computation) ───

func ProcessAtom() core.Atom[Request, Response] {
	return core.Atom[Request, Response](func(req Request) Response {
		// TODO: Replace with your business logic
		return Response{
			Result:  "processed: " + req.Data,
			Success: true,
		}
	})
}

// ─── Adapters (Side Effects) ───

type LogAdapter struct{}

func (a *LogAdapter) Execute(ctx context.Context, resp Response) (Response, error) {
	fmt.Printf("[LogAdapter] result=%s success=%v\n", resp.Result, resp.Success)
	return resp, nil
}

// ─── Pipeline ───

func buildPipeline(obs core.ObservationAdapter) core.Composer[Request] {
	return core.NewPipeline[Request](obs,
		// Step 1: Port — validate input
		core.PortAsStep(&RequestPort{}),

		// Step 2: Atom — pure computation (Request → Response)
		core.AdapterAsStep(core.NewAdapter[Request, Response](func(ctx context.Context, req Request) (Response, error) {
			return ProcessAtom()(req), nil
		})),

		// Step 3: Adapter — logging
		core.AdapterAsStep(&LogAdapter{}),
	)
}

// ─── HTTP Handler ───

func makeHandler(pipeline core.Composer[Request], obs core.ObservationAdapter) http.HandlerFunc {
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

		ctx := context.Background()
		_, steps, err := pipeline.Run(ctx, req)

		if err != nil {
			json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
			return
		}

		// Get the final response from observation steps
		var resp Response
		if len(steps) > 0 {
			// The last step contains our response
			lastStep := steps[len(steps)-1]
			if lastStep.Output != nil {
				if r, ok := lastStep.Output.(Response); ok {
					resp = r
				}
			}
		}
		resp.Steps = len(steps)
		json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	fmt.Println("=== {{.Project}} (Tier L1 Microservice) ===")

	// Observation adapter (in-memory for development)
	obs := &core.InMemoryObservationAdapter{}

	// Build pipeline
	pipeline := buildPipeline(obs)

	// HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/process", makeHandler(pipeline, obs))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","tier":"l1"}`)
	})

	// Start server
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Shutdown(context.Background())
	}()

	fmt.Printf("Server listening on %s\n", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Server error: %v\n", err)
	}
}
