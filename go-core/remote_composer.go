//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"encoding/json"
	"net/http"
)

// ctxKey is a private context key type for RemoteComposer.
type remoteCtxKey string

const remoteCtxKeyTraceID remoteCtxKey = "trace_id"

// RemoteCallRequest JSON-RPC 风格远程调用请求。
type RemoteCallRequest struct {
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	TraceID string          `json:"trace_id"`
}

// RemoteCallResponse JSON-RPC 风格远程调用响应。
type RemoteCallResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// RemoteComposer 将 Composer 暴露为 HTTP JSON-RPC 端点。
// 远程 Agent 可通过 POST /api/rpc/run 调用本地 Composer。
type RemoteComposer struct {
	composer Composer[any]
	obs      ObservationAdapter
}

// NewRemoteComposer 创建远程可调用的 Composer 包装。
func NewRemoteComposer(c Composer[any], obs ObservationAdapter) *RemoteComposer {
	return &RemoteComposer{composer: c, obs: obs}
}

// RegisterRemoteHandlers 注册远程调用 HTTP 端点。
// 注册路径:
//   POST /api/rpc/run  — 执行 Composer
//   GET  /api/rpc/health — 健康检查
func (rc *RemoteComposer) RegisterRemoteHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/rpc/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONResponse(w, 405, RemoteCallResponse{Error: "method not allowed"})
			return
		}
		var req RemoteCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, 400, RemoteCallResponse{Error: "invalid json: " + err.Error()})
			return
		}
		ctx := context.WithValue(r.Context(), remoteCtxKeyTraceID, TraceID(req.TraceID))
		result, steps, err := rc.composer.Run(ctx, req.Params)
		if rc.obs != nil && len(steps) > 0 {
			rc.obs.Record(steps)
		}
		if err != nil {
			writeJSONResponse(w, 500, RemoteCallResponse{Error: err.Error()})
			return
		}
		resultJSON, _ := json.Marshal(result)
		writeJSONResponse(w, 200, RemoteCallResponse{Result: resultJSON})
	})

	mux.HandleFunc("/api/rpc/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, 200, map[string]string{"status": "ok"})
	})
}

// writeJSONResponse 写入 JSON 响应。
func writeJSONResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}