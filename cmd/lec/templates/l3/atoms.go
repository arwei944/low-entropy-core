// {{.Project}} — Atom Implementations (Pure Computation)
// Atoms are pure functions with NO side effects. Input → Output, deterministic.

package main

import (
	"strings"

	core "{{.CoreModule}}"
)

// ProcessDataAtom processes the input data and produces a response.
// This is a pure function: same input always produces same output.
func ProcessDataAtom() core.Atom[Request, Response] {
	return core.Atom[Request, Response](func(req Request) Response {
		// TODO: Replace with your actual business logic
		result := map[string]any{
			"original": req.Data,
			"processed": strings.ToUpper(req.Data),
			"length":   len(req.Data),
		}

		return Response{
			ID:      req.ID,
			Result:  result,
			Success: true,
		}
	})
}

// TransformAtom applies a transformation to the data.
func TransformAtom(transform func(string) string) core.Atom[Request, Request] {
	return core.Atom[Request, Request](func(req Request) Request {
		req.Data = transform(req.Data)
		return req
	})
}

// ValidateDataAtom checks data integrity (pure validation, no I/O).
func ValidateDataAtom() core.Atom[Request, Request] {
	return core.Atom[Request, Request](func(req Request) Request {
		if len(req.Data) > 10000 {
			req.Data = req.Data[:10000] // Truncate
		}
		return req
	})
}
