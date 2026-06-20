// {{.Project}} — Business Types

package main

// Request represents an incoming API request.
type Request struct {
	ID     string            `json:"id"`
	Data   string            `json:"data"`
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

// Response represents an API response.
type Response struct {
	ID      string `json:"id"`
	Result  any    `json:"result,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Steps   int    `json:"steps"`
}

// Meta contains response metadata.
type Meta struct {
	TraceID    string `json:"trace_id"`
	DurationMs int64  `json:"duration_ms"`
	Tier       string `json:"tier"`
	Version    string `json:"version"`
}
