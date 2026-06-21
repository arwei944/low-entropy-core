//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package main

import (
	"context"
	"net/http"

	core "low-entropy-core/go-core"
)

// registerObservationHandlers 注册 Observation API 路由
// 仅在 lecore_tier4+ 构建标签下编译
func registerObservationHandlers(mux *http.ServeMux) {
	cfg := core.DefaultObservationPipelineConfig()
	pipeline := core.NewObservationPipeline(cfg)
	registry := core.NewArchitectureRegistry()
	api := core.NewObservationAPI(pipeline, registry)
	api.RegisterHandlers(mux)
	pipeline.Start(context.Background())
}
