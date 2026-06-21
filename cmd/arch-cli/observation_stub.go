//go:build !(lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

package main

import "net/http"

// registerObservationHandlers 空实现（低 tier 无 Observation API）
func registerObservationHandlers(mux *http.ServeMux) {
	// no-op: Observation API requires lecore_tier4+
}
