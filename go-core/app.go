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

	// 1. 持久化后端
	if cfg.StorageDir != "" {
		storage, err := NewFileStorageBackend(cfg.StorageDir)
		if err != nil {
			return nil, fmt.Errorf("app: storage: %w", err)
		}
		app.Storage = storage
		app.shutdownHooks = append(app.shutdownHooks, storage.Close)
	}

	// 2. EventStore
	if app.Storage != nil {
		es, err := NewPersistentEventStore(app.Storage)
		if err != nil {
			return nil, fmt.Errorf("app: eventstore: %w", err)
		}
		app.EventStore = es
	}

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
		// 创建独立的观测适配器用于 Handoff
		handoffObs := &InMemoryObservationAdapter{}
		handoff := NewHandoffComposer(handoffObs, NewInMemorySnapshotAdapter(), transport)
		app.Scheduler = NewSchedulerComposer(pool, queue, handoff, handoffObs)
	}

	// 6. HTTP
	if cfg.HTTPAddr != "" {
		mux := http.NewServeMux()
		app.obsAPI = NewObservationAPI(app.Observation, nil)
		app.obsAPI.RegisterHandlers(mux)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","version":"` + cfg.Version + `"}`))
		})

		// 7. Remote RPC — 暴露 JSON-RPC 端点供远程 Agent 调用
		remoteObs := &InMemoryObservationAdapter{}
		defaultPipeline := NewPipeline[any](remoteObs)
		remote := NewRemoteComposer(defaultPipeline, remoteObs)
		remote.RegisterRemoteHandlers(mux)

		app.HTTP = &http.Server{
			Addr:    cfg.HTTPAddr,
			Handler: mux,
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