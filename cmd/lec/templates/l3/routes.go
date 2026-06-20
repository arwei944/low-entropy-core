// {{.Project}} — HTTP Route Handlers
// Routes connect HTTP endpoints to Pipeline execution.

package main

import (
	"context"
	"encoding/json"
	"net/http"

	core "{{.CoreModule}}"
)

// registerRoutes sets up all HTTP routes.
func registerRoutes(mux *http.ServeMux, pipeline core.Composer[Request], obs core.ObservationAdapter) {
	mux.HandleFunc("/api/process", processHandler(pipeline, obs))
	mux.HandleFunc("/api/health", healthHandler())
}

// processHandler handles POST /api/process
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

		ctx := r.Context()
		_, steps, err := pipeline.Run(ctx, req)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
			return
		}

		// Extract response from last step
		var resp Response
		if len(steps) > 0 {
			last := steps[len(steps)-1]
			if last.Output != nil {
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

// healthHandler handles GET /api/health
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"tier":   "l3",
		})
	}
}
