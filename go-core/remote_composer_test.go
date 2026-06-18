//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteComposer_Run(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	pipeline := NewPipeline[any](obs,
		AtomAsStep[any, any](func(input any) any {
			return map[string]interface{}{"echo": input}
		}),
	)
	rc := NewRemoteComposer(pipeline, obs)

	mux := http.NewServeMux()
	rc.RegisterRemoteHandlers(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reqBody := RemoteCallRequest{
		Method:  "run",
		Params:  json.RawMessage(`"hello-world"`),
		TraceID: "trace-001",
	}
	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(srv.URL+"/api/rpc/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var rpcResp RemoteCallResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error != "" {
		t.Errorf("unexpected error: %s", rpcResp.Error)
	}
	if string(rpcResp.Result) != `{"echo":"hello-world"}` {
		t.Errorf("unexpected result: %s", rpcResp.Result)
	}
}

func TestRemoteComposer_Health(t *testing.T) {
	rc := NewRemoteComposer(nil, nil)
	mux := http.NewServeMux()
	rc.RegisterRemoteHandlers(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/rpc/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRemoteComposer_Error(t *testing.T) {
	obs := &InMemoryObservationAdapter{}
	pipeline := NewPipeline[any](obs,
		AdapterAsStep[any, any](&errorAdapter{}),
	)
	rc := NewRemoteComposer(pipeline, obs)

	mux := http.NewServeMux()
	rc.RegisterRemoteHandlers(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reqBody := RemoteCallRequest{Method: "run", Params: json.RawMessage(`"x"`)}
	body, _ := json.Marshal(reqBody)
	resp, _ := http.Post(srv.URL+"/api/rpc/run", "application/json", bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Errorf("expected 500 for error, got %d", resp.StatusCode)
	}
}

func TestRemoteComposer_MethodNotAllowed(t *testing.T) {
	rc := NewRemoteComposer(nil, nil)
	mux := http.NewServeMux()
	rc.RegisterRemoteHandlers(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/rpc/run")
	defer resp.Body.Close()

	if resp.StatusCode != 405 {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// errorAdapter 总是返回错误的 Adapter，用于测试错误路径。
type errorAdapter struct{}

func (e *errorAdapter) Execute(ctx context.Context, input any) (any, error) {
	return nil, NewStepError("TEST_ERROR", "intentional failure", false)
}