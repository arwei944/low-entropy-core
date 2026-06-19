//go:build lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// App 统一应用入口。
// 一行代码初始化所有子系统：持久化、EventStore、Observation、Guardian、Scheduler、HTTP。
type App struct {
	Config       AppConfig
	Storage      StorageBackend
	EventStore   *PersistentEventStore
	Observation  *ObservationPipeline
	Scheduler    *SchedulerComposer
	Guardian     *EntropyWatcher
	HTTP         *http.Server
	obsAPI       *ObservationAPI
	shutdownHooks []func() error
}

// NewApp 创建并初始化所有子系统。
// 按依赖顺序组装：Storage → EventStore → Observation → Guardian → Scheduler → HTTP。
func NewApp(cfg AppConfig) (*App, error) {
	app := &App{Config: cfg}

	// 0. 可观测性 (v0.9.0)
	var obsProv *ObservabilityProvider
	if cfg.ObservabilityEnabled {
		obsProv = NewNoOpObservabilityProvider()
		// 用户可后续注入真实的 OpenTelemetry/Prometheus/slog Provider
	}

	// 1. 持久化后端 (v0.9.0: 支持多后端)
	storage, err := NewStorageBackendFromConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("app: storage: %w", err)
	}
	app.Storage = storage
	app.shutdownHooks = append(app.shutdownHooks, storage.Close)

	// 2. EventStore (v0.9.0: 支持多后端)
	es, err := NewPersistentEventStore(app.Storage)
	if err != nil {
		return nil, fmt.Errorf("app: eventstore: %w", err)
	}
	app.EventStore = es

	// 3. Observation
	obsCfg := DefaultObservationPipelineConfig()
	obsCfg.BufferSize = cfg.ObservationBufferSize
	app.Observation = NewObservationPipeline(obsCfg)
	app.Observation.Start(context.Background())

	// 4. Guardian
	if cfg.GuardianEnabled {
		app.Guardian = NewEntropyWatcherWithThresholds(cfg.EntropyCeiling*0.5, cfg.EntropyCeiling*0.75, cfg.EntropyCeiling)
	}

	// 5. Scheduler
	if cfg.SchedulerEnabled {
		pool := NewAgentPool()
		queue := NewTaskQueue()
		transport := NewInProcHandoffTransport()
		handoffObs := &InMemoryObservationAdapter{}
		handoff := NewHandoffComposer(handoffObs, NewInMemorySnapshotAdapter(), transport)
		app.Scheduler = NewSchedulerComposer(pool, queue, handoff, handoffObs)
	}

	// 6. HTTP
	if cfg.HTTPAddr != "" {
		mux := http.NewServeMux()
		app.obsAPI = NewObservationAPI(app.Observation, nil)
		app.obsAPI.RegisterHandlers(mux)

		// 健康检查端点 (v0.9.0: 增强版)
		healthCheckers := map[string]func(context.Context) error{
			"storage": storage.HealthCheck,
		}
		mux.Handle("/health", HealthHandler(healthCheckers))

		// 7. Remote RPC — 暴露 JSON-RPC 端点供远程 Agent 调用
		remoteObs := &InMemoryObservationAdapter{}
		defaultPipeline := NewPipeline[any](remoteObs)
		// v0.9.0: 注入可观测性
		if obsProv != nil {
			defaultPipeline.WithObservability(obsProv)
		}
		remote := NewRemoteComposer(defaultPipeline, remoteObs)
		remote.RegisterRemoteHandlers(mux)

		// v0.9.0: HTTP 可观测性中间件
		var handler http.Handler = mux
		if obsProv != nil {
			handler = HTTPMiddleware(mux, obsProv)
		}

		app.HTTP = &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: handler,
		}
	}

	return app, nil
}

// Start 启动所有服务（HTTP 服务器在后台 goroutine 运行）。
func (app *App) Start() error {
	if app.HTTP != nil {
		go func() {
			log.Printf("[app] HTTP server listening on %s", app.HTTP.Addr)
			if err := app.HTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[app] HTTP error: %v", err)
			}
		}()
	}
	return nil
}

// WaitForShutdown 等待 SIGINT/SIGTERM 信号并优雅关闭所有子系统。
func (app *App) WaitForShutdown(timeout time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("[app] shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if app.HTTP != nil {
		app.HTTP.Shutdown(ctx)
	}
	if app.Observation != nil {
		app.Observation.Stop()
	}
	for _, hook := range app.shutdownHooks {
		hook()
	}
	log.Printf("[app] shutdown complete")
}