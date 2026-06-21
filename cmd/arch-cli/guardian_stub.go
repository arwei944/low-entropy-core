//go:build !(lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7)

package main

import "net/http"

func initGuardian() {}

func handleGuardianSnapshot(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func handleGuardianSSE(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func handleGuardianThresholds(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func handleGuardianThresholdsPut(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func handleGuardianDrift(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func handleGuardianHistory(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"Guardian requires lecore_tier4+"}`, http.StatusNotImplemented)
}

func recordSnapshot() {}
