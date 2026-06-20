// {{.Project}} — Tier L1 Microservice — Business Types
// All domain types are defined here.

package main

// Request represents an incoming API request.
type Request struct {
	Data   string            `json:"data"`
	Params map[string]string `json:"params,omitempty"`
}

// Response represents an API response.
type Response struct {
	Result  any     `json:"result,omitempty"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
	Steps   int     `json:"steps"`
	Meta    *Meta   `json:"meta,omitempty"`
}

// Meta contains response metadata.
type Meta struct {
	TraceID   string `json:"trace_id"`
	DurationMs int64  `json:"duration_ms"`
	Tier      string `json:"tier"`
}
