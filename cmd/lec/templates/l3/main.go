// {{.Project}} — Tier L3 Large Service
// Build: go build -tags lecore_tier3 -o server .
// Tier: L3 (Large Service) — 100+ files, distributed
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

func main() {
	fmt.Println("=== {{.Project}} (Tier L3 Large Service) ===")

	// Observation adapter
	obs := &core.InMemoryObservationAdapter{}

	// Build pipelines
	processPipeline := buildProcessPipeline(obs)

	// HTTP routes
	mux := http.NewServeMux()
	registerRoutes(mux, processPipeline, obs)

	// Health & Observation endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"tier":   "l3",
			"time":   time.Now().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/api/observation/steps", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(obs.GetSteps())
	})

	// Start server
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
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
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Server error: %v\n", err)
	}
}

// buildProcessPipeline constructs the main business pipeline.
func buildProcessPipeline(obs core.ObservationAdapter) core.Composer[Request] {
	return core.NewPipeline[Request](obs,
		// Port: Validate input
		core.PortAsStep(&RequestPort{}),

		// Atom: Process data
		core.AdapterAsStep(core.NewAdapter[Request, Response](func(ctx context.Context, req Request) (Response, error) {
			return ProcessDataAtom()(req), nil
		})),

		// Adapter: Persist result
		core.AdapterAsStep(&PersistAdapter{}),

		// Adapter: Log
		core.AdapterAsStep(&LogAdapter{}),
	)
}
