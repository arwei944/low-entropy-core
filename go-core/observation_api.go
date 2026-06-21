//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ──────────────────────────────────────────────
// ObservationAPI — HTTP API for the X-Ray dashboard
// ──────────────────────────────────────────────

// ObservationAPI serves the observation layer over HTTP.
// It provides 9 endpoints for the X-Ray dashboard covering
// static analysis, dynamic monitoring, and drill-down capabilities.
type ObservationAPI struct {
	pipeline *ObservationPipeline
	registry *ArchitectureRegistry
}

// NewObservationAPI creates a new ObservationAPI handler.
func NewObservationAPI(pipeline *ObservationPipeline, registry *ArchitectureRegistry) *ObservationAPI {
	return &ObservationAPI{pipeline: pipeline, registry: registry}
}

// RegisterHandlers registers all observation API endpoints on the given mux.
func (api *ObservationAPI) RegisterHandlers(mux *http.ServeMux) {
	// Core observation endpoints
	mux.HandleFunc("/api/observation/steps", api.handleSteps)
	mux.HandleFunc("/api/observation/steps/query", api.handleStepsQuery)
	mux.HandleFunc("/api/observation/steps/errors", api.handleStepsErrors)
	mux.HandleFunc("/api/observation/trace/", api.handleTrace)
	mux.HandleFunc("/api/observation/trace-tree", api.handleTraceTree)

	// Aggregation endpoints
	mux.HandleFunc("/api/observation/aggregates", api.handleAggregates)
	mux.HandleFunc("/api/observation/aggregates/query", api.handleAggregatesQuery)

	// Architecture endpoints
	mux.HandleFunc("/api/observation/pipelines", api.handlePipelines)
	mux.HandleFunc("/api/observation/architecture", api.handleArchitecture)
}

// ─── GET /api/observation/steps — all steps ───

func (api *ObservationAPI) handleSteps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	steps := api.pipeline.Store().Query(StepQuery{})
	api.json(w, steps)
}

// ─── GET /api/observation/steps/query?trace_id=&unit=&pattern=&error_only=&since=&limit= ───

func (api *ObservationAPI) handleStepsQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	query := StepQuery{
		TraceID:   q.Get("trace_id"),
		Unit:      q.Get("unit"),
		Pattern:   q.Get("pattern"),
		ErrorOnly: q.Get("error_only") == "true",
	}

	if sinceStr := q.Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			query.Since = t
		}
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			query.Limit = n
		}
	}

	steps := api.pipeline.Store().Query(query)
	api.json(w, steps)
}

// ─── GET /api/observation/steps/errors — error-only steps ───

func (api *ObservationAPI) handleStepsErrors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	steps := api.pipeline.Store().Query(StepQuery{ErrorOnly: true})
	api.json(w, steps)
}

// ─── GET /api/observation/trace/{trace_id} — steps by trace ───

func (api *ObservationAPI) handleTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := r.URL.Path[len("/api/observation/trace/"):]
	if traceID == "" {
		http.Error(w, "trace_id required", http.StatusBadRequest)
		return
	}
	steps := api.pipeline.Store().Query(StepQuery{TraceID: traceID})
	api.json(w, steps)
}

// ─── GET /api/observation/trace-tree — trace tree ───

func (api *ObservationAPI) handleTraceTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	steps := api.pipeline.Store().Query(StepQuery{})
	tree := BuildTraceTree(steps)
	api.json(w, tree)
}

// ─── GET /api/observation/aggregates — all aggregates ───

func (api *ObservationAPI) handleAggregates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if api.pipeline.Aggregator() == nil {
		api.json(w, []AggregateResult{})
		return
	}
	results := api.pipeline.Aggregator().GetResults()
	api.json(w, results)
}

// ─── GET /api/observation/aggregates/query?window=&unit=&pattern= ───

func (api *ObservationAPI) handleAggregatesQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if api.pipeline.Aggregator() == nil {
		api.json(w, []AggregateResult{})
		return
	}

	q := r.URL.Query()
	results := api.pipeline.Aggregator().QueryResults(
		q.Get("window"),
		q.Get("unit"),
		q.Get("pattern"),
	)
	api.json(w, results)
}

// ─── GET /api/observation/pipelines — registered pipelines ───

func (api *ObservationAPI) handlePipelines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if api.registry == nil {
		api.json(w, []PipelineDescriptor{})
		return
	}
	api.json(w, api.registry.List())
}

// ─── GET /api/observation/architecture — full architecture snapshot ───

func (api *ObservationAPI) handleArchitecture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := map[string]any{
		"total_steps": api.pipeline.Store().Count(),
		"sampler_dropped": func() int {
			if api.pipeline.Sampler() != nil {
				return api.pipeline.Sampler().DroppedCount()
			}
			return 0
		}(),
	}

	if api.pipeline.Aggregator() != nil {
		resp["aggregates"] = api.pipeline.Aggregator().GetResults()
	}
	if api.registry != nil {
		resp["pipelines"] = api.registry.List()
	}

	api.json(w, resp)
}

// json writes a JSON response.
func (api *ObservationAPI) json(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}