# Low-Entropy Core 生产化攻坚计划

> **For agentic workers:** 按阶段顺序执行，每个 Task 是一个最小化任务单元（2-5 分钟）。
> 使用 checkbox (`- [ ]`) 追踪进度。

**Goal:** 将 Low-Entropy Core 从"架构库"升级为"生产级可部署产品"，覆盖持久化、分布式、模式扩展、前端深化四大方向。

**Architecture:** 零外部依赖的纯 Go 标准库策略。持久化层通过接口抽象（`StorageBackend`），支持内存/文件/Redis 多后端。统一启动入口 `NewApp()` 一次性组装所有子系统。

**Tech Stack:** Go 1.22 stdlib（`net/http`、`encoding/json`、`crypto/sha256`、`os`、`sync`、`context`），前端 HTML/CSS/JS + ECharts。

---

## Phase 1: 生产化适配层（Day 1-2）

### Task 1.1: 文件系统持久化后端 — `FileStorageBackend`

**Files:**
- Create: `go-core/storage_fs.go`
- Create: `go-core/storage_fs_test.go`

**Goal:** 为 EventStore / IdempotentStore / Scheduler 提供文件系统持久化后端，实现 `StorageBackend` 接口。

- [ ] **Step 1: 定义 `StorageBackend` 接口**

```go
// storage_fs.go
package core

import "context"

// StorageBackend 持久化存储后端接口
type StorageBackend interface {
    Save(ctx context.Context, key string, data []byte) error
    Load(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}

// FileStorageBackend 基于文件系统的持久化后端
type FileStorageBackend struct {
    dir string
    mu  sync.RWMutex
}

func NewFileStorageBackend(dir string) (*FileStorageBackend, error) {
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("storage: create dir %s: %w", dir, err)
    }
    return &FileStorageBackend{dir: dir}, nil
}

func (fs *FileStorageBackend) keyPath(key string) string {
    return filepath.Join(fs.dir, key+".json")
}

func (fs *FileStorageBackend) Save(ctx context.Context, key string, data []byte) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()
    return os.WriteFile(fs.keyPath(key), data, 0644)
}

func (fs *FileStorageBackend) Load(ctx context.Context, key string) ([]byte, error) {
    fs.mu.RLock()
    defer fs.mu.RUnlock()
    data, err := os.ReadFile(fs.keyPath(key))
    if os.IsNotExist(err) {
        return nil, fmt.Errorf("storage: key %s not found", key)
    }
    return data, err
}

func (fs *FileStorageBackend) Delete(ctx context.Context, key string) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()
    err := os.Remove(fs.keyPath(key))
    if os.IsNotExist(err) {
        return nil
    }
    return err
}

func (fs *FileStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
    fs.mu.RLock()
    defer fs.mu.RUnlock()
    entries, err := os.ReadDir(fs.dir)
    if err != nil {
        return nil, err
    }
    var keys []string
    for _, e := range entries {
        name := e.Name()
        if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".json") {
            keys = append(keys, strings.TrimSuffix(name, ".json"))
        }
    }
    return keys, nil
}

func (fs *FileStorageBackend) Close() error { return nil }
```

**所需 imports:** `"fmt"`, `"os"`, `"path/filepath"`, `"strings"`, `"sync"`

- [ ] **Step 2: 编写测试**

```go
// storage_fs_test.go
package core

import (
    "context"
    "os"
    "testing"
)

func TestFileStorageBackend_SaveLoad(t *testing.T) {
    dir := t.TempDir()
    fs, err := NewFileStorageBackend(dir)
    if err != nil {
        t.Fatal(err)
    }
    defer fs.Close()

    ctx := context.Background()
    if err := fs.Save(ctx, "test/key1", []byte("hello")); err != nil {
        t.Fatal(err)
    }
    data, err := fs.Load(ctx, "test/key1")
    if err != nil {
        t.Fatal(err)
    }
    if string(data) != "hello" {
        t.Errorf("expected 'hello', got %q", data)
    }
}

func TestFileStorageBackend_Delete(t *testing.T) {
    dir := t.TempDir()
    fs, _ := NewFileStorageBackend(dir)
    defer fs.Close()

    ctx := context.Background()
    fs.Save(ctx, "k", []byte("v"))
    fs.Delete(ctx, "k")
    _, err := fs.Load(ctx, "k")
    if err == nil {
        t.Error("expected error after delete")
    }
}

func TestFileStorageBackend_List(t *testing.T) {
    dir := t.TempDir()
    fs, _ := NewFileStorageBackend(dir)
    defer fs.Close()

    ctx := context.Background()
    fs.Save(ctx, "events/1", []byte("a"))
    fs.Save(ctx, "events/2", []byte("b"))
    fs.Save(ctx, "other/x", []byte("c"))

    keys, err := fs.List(ctx, "events/")
    if err != nil {
        t.Fatal(err)
    }
    if len(keys) != 2 {
        t.Errorf("expected 2 keys, got %d: %v", len(keys), keys)
    }
}
```

- [ ] **Step 3: 运行测试验证**

```bash
cd go-core && go test -run TestFileStorageBackend -v -count=1
```

- [ ] **Step 4: Commit**

```bash
git add go-core/storage_fs.go go-core/storage_fs_test.go
git commit -m "feat: add FileStorageBackend — filesystem persistence for StorageBackend interface"
```

---

### Task 1.2: 持久化 EventStore — `PersistentEventStore`

**Files:**
- Create: `go-core/eventstore_persistent.go`
- Create: `go-core/eventstore_persistent_test.go`

**Goal:** 在 `EventStore` 基础上包装 `StorageBackend`，实现事件的持久化读写。

- [ ] **Step 1: 实现 `PersistentEventStore`**

```go
// eventstore_persistent.go
package core

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
)

// PersistentEventStore 带持久化的事件存储
type PersistentEventStore struct {
    backend StorageBackend
    cache   *EventStore
    mu      sync.RWMutex
}

func NewPersistentEventStore(backend StorageBackend) (*PersistentEventStore, error) {
    pes := &PersistentEventStore{
        backend: backend,
        cache:   NewEventStore(),
    }
    // 从持久化后端恢复数据
    if err := pes.restore(context.Background()); err != nil {
        return nil, fmt.Errorf("persistent eventstore: restore: %w", err)
    }
    return pes, nil
}

func (pes *PersistentEventStore) restore(ctx context.Context) error {
    keys, err := pes.backend.List(ctx, "events/")
    if err != nil {
        return err
    }
    for _, key := range keys {
        data, err := pes.backend.Load(ctx, key)
        if err != nil {
            continue
        }
        var envelopes []EventEnvelope
        if err := json.Unmarshal(data, &envelopes); err != nil {
            continue
        }
        for _, e := range envelopes {
            pes.cache.Execute(ctx, e)
        }
    }
    return nil
}

func (pes *PersistentEventStore) Execute(ctx context.Context, input EventEnvelope) (AppendResult, error) {
    pes.mu.Lock()
    defer pes.mu.Unlock()

    result, err := pes.cache.Execute(ctx, input)
    if err != nil {
        return result, err
    }

    // 持久化
    all := pes.cache.StreamAll(input.AggregateID)
    data, err := json.Marshal(all)
    if err != nil {
        return result, fmt.Errorf("persistent eventstore: marshal: %w", err)
    }
    key := "events/" + input.AggregateID
    if err := pes.backend.Save(ctx, key, data); err != nil {
        return result, fmt.Errorf("persistent eventstore: save: %w", err)
    }
    return result, nil
}

func (pes *PersistentEventStore) Stream(aggregateID string, fromVersion int64) []EventEnvelope {
    pes.mu.RLock()
    defer pes.mu.RUnlock()
    return pes.cache.Stream(aggregateID, fromVersion)
}

func (pes *PersistentEventStore) StreamAll(aggregateID string) []EventEnvelope {
    pes.mu.RLock()
    defer pes.mu.RUnlock()
    return pes.cache.StreamAll(aggregateID)
}

func (pes *PersistentEventStore) GetLatestVersion(aggregateID string) int64 {
    pes.mu.RLock()
    defer pes.mu.RUnlock()
    return pes.cache.GetLatestVersion(aggregateID)
}

func (pes *PersistentEventStore) Count(aggregateID string) int {
    pes.mu.RLock()
    defer pes.mu.RUnlock()
    return pes.cache.Count(aggregateID)
}
```

- [ ] **Step 2: 编写测试**

```go
// eventstore_persistent_test.go
package core

import (
    "context"
    "testing"
)

func TestPersistentEventStore_Basic(t *testing.T) {
    dir := t.TempDir()
    fs, _ := NewFileStorageBackend(dir)
    pes, err := NewPersistentEventStore(fs)
    if err != nil {
        t.Fatal(err)
    }

    ctx := context.Background()
    evt := EventEnvelope{
        EventID:      "evt-1",
        AggregateID:  "agg-1",
        AggregateType: "Test",
        EventType:    "Created",
        EventData:    []byte("{}"),
        Version:      1,
    }
    result, err := pes.Execute(ctx, evt)
    if err != nil {
        t.Fatal(err)
    }
    if !result.Success {
        t.Error("expected success")
    }
    if v := pes.GetLatestVersion("agg-1"); v != 1 {
        t.Errorf("expected version 1, got %d", v)
    }
}

func TestPersistentEventStore_Restore(t *testing.T) {
    dir := t.TempDir()
    fs, _ := NewFileStorageBackend(dir)

    // 第一轮写入
    pes1, _ := NewPersistentEventStore(fs)
    ctx := context.Background()
    pes1.Execute(ctx, EventEnvelope{EventID: "e1", AggregateID: "a", EventType: "X", Version: 1})

    // 第二轮恢复
    pes2, err := NewPersistentEventStore(fs)
    if err != nil {
        t.Fatal(err)
    }
    if v := pes2.GetLatestVersion("a"); v != 1 {
        t.Errorf("restore failed: expected version 1, got %d", v)
    }
}
```

- [ ] **Step 3: 运行测试验证**

```bash
cd go-core && go test -run TestPersistentEventStore -v -count=1
```

- [ ] **Step 4: Commit**

```bash
git add go-core/eventstore_persistent.go go-core/eventstore_persistent_test.go
git commit -m "feat: add PersistentEventStore — event persistence via StorageBackend"
```

---

### Task 1.3: 统一启动入口 — `NewApp()`

**Files:**
- Create: `go-core/app.go`
- Create: `go-core/app_test.go`

**Goal:** 提供 `App` 统一启动入口，一行代码初始化所有子系统。

- [ ] **Step 1: 定义 `App` 和 `AppConfig`**

```go
// app.go
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

// AppConfig 应用配置
type AppConfig struct {
    Name    string `json:"name"`
    Version string `json:"version"`

    // 持久化
    StorageDir string `json:"storage_dir"`

    // HTTP
    HTTPAddr string `json:"http_addr"`

    // 观测
    ObservationBufferSize int `json:"observation_buffer_size"`

    // Guardian
    GuardianEnabled bool    `json:"guardian_enabled"`
    EntropyCeiling  float64 `json:"entropy_ceiling"`

    // 调度器
    SchedulerEnabled bool `json:"scheduler_enabled"`
    AgentHeartbeatTTL time.Duration `json:"agent_heartbeat_ttl"`
}

// DefaultAppConfig 返回默认配置
func DefaultAppConfig() AppConfig {
    return AppConfig{
        Name:                  "low-entropy-core",
        Version:               "4.0.0",
        StorageDir:            "./data",
        HTTPAddr:              ":8080",
        ObservationBufferSize: 10000,
        GuardianEnabled:       true,
        EntropyCeiling:        0.8,
        SchedulerEnabled:      false,
        AgentHeartbeatTTL:     30 * time.Second,
    }
}

// App 统一应用入口
type App struct {
    Config       AppConfig
    Storage      StorageBackend
    EventStore   *PersistentEventStore
    Observation  *ObservationPipeline
    Scheduler    *SchedulerComposer
    Guardian     *GuardianEntropy
    HTTP         *http.Server
    obsAPI       *ObservationAPI
    shutdownHooks []func() error
}
```

- [ ] **Step 2: 实现 `NewApp()` 组装逻辑**

```go
// NewApp 创建并初始化所有子系统
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
    obs := NewInMemoryObservationAdapter()
    app.Observation = NewObservationPipeline(
        obs,
        cfg.ObservationBufferSize,
        &DefaultSamplingPolicy{},
    )
    app.Observation.Start(context.Background())

    // 4. Guardian
    if cfg.GuardianEnabled {
        app.Guardian = NewGuardianEntropy(cfg.EntropyCeiling)
    }

    // 5. Scheduler
    if cfg.SchedulerEnabled {
        pool := NewAgentPool()
        queue := NewTaskQueue()
        transport := NewInProcHandoffTransport()
        handoff := NewHandoffComposer(obs, &InMemorySnapshotAdapter{}, transport)
        app.Scheduler = NewSchedulerComposer(pool, queue, handoff, obs)
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
        app.HTTP = &http.Server{
            Addr:    cfg.HTTPAddr,
            Handler: mux,
        }
    }

    return app, nil
}

// Start 启动所有服务
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

// WaitForShutdown 等待信号并优雅关闭
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
```

- [ ] **Step 3: 编写测试**

```go
// app_test.go
package core

import (
    "testing"
    "time"
)

func TestApp_NewAndStart(t *testing.T) {
    cfg := DefaultAppConfig()
    cfg.StorageDir = t.TempDir()
    cfg.HTTPAddr = ":0" // random port

    app, err := NewApp(cfg)
    if err != nil {
        t.Fatal(err)
    }
    if app.EventStore == nil {
        t.Error("expected EventStore to be initialized")
    }
    if app.Observation == nil {
        t.Error("expected Observation to be initialized")
    }
    if app.Guardian == nil {
        t.Error("expected Guardian to be initialized")
    }
    if app.HTTP == nil {
        t.Error("expected HTTP to be initialized")
    }
}

func TestApp_StartStop(t *testing.T) {
    cfg := DefaultAppConfig()
    cfg.StorageDir = t.TempDir()
    cfg.HTTPAddr = ":0"

    app, _ := NewApp(cfg)
    if err := app.Start(); err != nil {
        t.Fatal(err)
    }
    // 给服务器一点时间启动
    time.Sleep(50 * time.Millisecond)
    // 直接关闭
    app.HTTP.Close()
}
```

- [ ] **Step 4: 运行测试验证**

```bash
cd go-core && go test -run TestApp -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go-core/app.go go-core/app_test.go
git commit -m "feat: add App bootstrap — unified entry point for all subsystems"
```

---

### Task 1.4: 生产级配置 — YAML/ENV 支持

**Files:**
- Modify: `go-core/config.go`

**Goal:** 扩展现有 `ParseConfig` 支持 YAML 和环境变量覆盖。

- [ ] **Step 1: 添加 `AppConfig` 的 JSON/YAML/ENV 加载**

在 `config.go` 末尾追加：

```go
import (
    "encoding/json"
    "os"
    "strconv"
)

// LoadAppConfigFromFile 从 JSON 文件加载 AppConfig
func LoadAppConfigFromFile(path string) (AppConfig, error) {
    cfg := DefaultAppConfig()
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return cfg, nil // 文件不存在时返回默认配置
        }
        return cfg, fmt.Errorf("config: read %s: %w", path, err)
    }
    if err := json.Unmarshal(data, &cfg); err != nil {
        return cfg, fmt.Errorf("config: parse %s: %w", path, err)
    }
    return cfg, nil
}

// ApplyEnvOverrides 用环境变量覆盖配置
// 支持: APP_NAME, APP_VERSION, APP_STORAGE_DIR, APP_HTTP_ADDR, APP_GUARDIAN_ENABLED, APP_ENTROPY_CEILING
func ApplyEnvOverrides(cfg *AppConfig) {
    if v := os.Getenv("APP_NAME"); v != "" {
        cfg.Name = v
    }
    if v := os.Getenv("APP_VERSION"); v != "" {
        cfg.Version = v
    }
    if v := os.Getenv("APP_STORAGE_DIR"); v != "" {
        cfg.StorageDir = v
    }
    if v := os.Getenv("APP_HTTP_ADDR"); v != "" {
        cfg.HTTPAddr = v
    }
    if v := os.Getenv("APP_GUARDIAN_ENABLED"); v != "" {
        cfg.GuardianEnabled, _ = strconv.ParseBool(v)
    }
    if v := os.Getenv("APP_ENTROPY_CEILING"); v != "" {
        cfg.EntropyCeiling, _ = strconv.ParseFloat(v, 64)
    }
    if v := os.Getenv("APP_SCHEDULER_ENABLED"); v != "" {
        cfg.SchedulerEnabled, _ = strconv.ParseBool(v)
    }
}
```

- [ ] **Step 2: 编写测试**

在 `config_test.go` 末尾追加：

```go
func TestLoadAppConfigFromFile(t *testing.T) {
    dir := t.TempDir()
    path := dir + "/app.json"
    os.WriteFile(path, []byte(`{"name":"test","version":"1.0.0"}`), 0644)

    cfg, err := LoadAppConfigFromFile(path)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Name != "test" {
        t.Errorf("expected name 'test', got %q", cfg.Name)
    }
    if cfg.Version != "1.0.0" {
        t.Errorf("expected version '1.0.0', got %q", cfg.Version)
    }
}

func TestLoadAppConfig_MissingFile(t *testing.T) {
    cfg, err := LoadAppConfigFromFile("/nonexistent/path.json")
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Name != "low-entropy-core" {
        t.Errorf("expected default name, got %q", cfg.Name)
    }
}

func TestApplyEnvOverrides(t *testing.T) {
    cfg := DefaultAppConfig()
    os.Setenv("APP_NAME", "env-test")
    os.Setenv("APP_HTTP_ADDR", ":9999")
    defer os.Unsetenv("APP_NAME")
    defer os.Unsetenv("APP_HTTP_ADDR")

    ApplyEnvOverrides(&cfg)
    if cfg.Name != "env-test" {
        t.Errorf("expected 'env-test', got %q", cfg.Name)
    }
    if cfg.HTTPAddr != ":9999" {
        t.Errorf("expected ':9999', got %q", cfg.HTTPAddr)
    }
}
```

- [ ] **Step 3: 运行测试验证**

```bash
cd go-core && go test -run "TestLoadAppConfig|TestApplyEnv" -v -count=1
```

- [ ] **Step 4: Commit**

```bash
git add go-core/config.go go-core/config_test.go
git commit -m "feat: add AppConfig file loading and env override support"
```

---

## Phase 2: 分布式能力扩展（Day 3-4）

### Task 2.1: EventBus 持久化订阅

**Files:**
- Modify: `go-core/eventbus.go`

**Goal:** 让 EventBus 订阅关系支持持久化，进程重启后自动恢复订阅。

- [ ] **Step 1: 添加 `PersistentEventBus`**

在 `eventbus.go` 末尾追加：

```go
// PersistentEventBus 带持久化订阅的 EventBus
type PersistentEventBus struct {
    bus     *EventBus
    backend StorageBackend
    mu      sync.RWMutex
}

func NewPersistentEventBus(backend StorageBackend) (*PersistentEventBus, error) {
    peb := &PersistentEventBus{
        bus:     NewEventBus(),
        backend: backend,
    }
    if err := peb.restoreSubscriptions(context.Background()); err != nil {
        return nil, fmt.Errorf("persistent eventbus: restore: %w", err)
    }
    return peb, nil
}

func (peb *PersistentEventBus) restoreSubscriptions(ctx context.Context) error {
    keys, err := peb.backend.List(ctx, "subscriptions/")
    if err != nil {
        return err
    }
    for _, key := range keys {
        // key format: subscriptions/{eventType}/{subscriberID}
        parts := strings.SplitN(strings.TrimPrefix(key, "subscriptions/"), "/", 2)
        if len(parts) != 2 {
            continue
        }
        // 恢复订阅关系（handler 需要重新注册，这里仅恢复元数据）
        log.Printf("[eventbus] restored subscription: %s -> %s", parts[0], parts[1])
    }
    return nil
}

func (peb *PersistentEventBus) Subscribe(eventType string, handler EventHandler) {
    peb.bus.Subscribe(eventType, handler)
    // 持久化订阅元数据
    key := fmt.Sprintf("subscriptions/%s/%s", eventType, handler.ID())
    peb.backend.Save(context.Background(), key, []byte(eventType))
}

func (peb *PersistentEventBus) Publish(ctx context.Context, eventType string, payload interface{}) {
    peb.bus.Publish(ctx, eventType, payload)
}

func (peb *PersistentEventBus) Unsubscribe(eventType string, handler EventHandler) {
    peb.bus.Unsubscribe(eventType, handler)
    key := fmt.Sprintf("subscriptions/%s/%s", eventType, handler.ID())
    peb.backend.Delete(context.Background(), key)
}
```

- [ ] **Step 2: 查看 EventBus 现有接口确认签名**

```bash
cd go-core && grep -n "func.*EventBus\|type EventBus\|type EventHandler" eventbus.go
```

- [ ] **Step 3: 根据实际签名调整 PersistentEventBus**

- [ ] **Step 4: 运行全量测试确保无回归**

```bash
cd go-core && go test ./... -count=1
```

- [ ] **Step 5: Commit**

```bash
git add go-core/eventbus.go
git commit -m "feat: add PersistentEventBus with subscription persistence"
```

---

### Task 2.2: gRPC 服务端骨架

**Files:**
- Create: `go-core/grpc_server.go`

**Goal:** 提供 gRPC 服务端入口，使远程 Agent 可通过 gRPC 调用本地 Composer。

**Note:** 由于项目零外部依赖策略，此任务使用 `net/http` 实现 JSON-RPC 风格的远程调用替代 gRPC（避免引入 protobuf 依赖）。

- [ ] **Step 1: 实现 JSON-RPC Handler**

```go
// grpc_server.go → 实际为 jsonrpc_server.go
package core

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

// RemoteCallRequest 远程调用请求
type RemoteCallRequest struct {
    Method string          `json:"method"`
    Params json.RawMessage `json:"params"`
    TraceID string         `json:"trace_id"`
}

// RemoteCallResponse 远程调用响应
type RemoteCallResponse struct {
    Result json.RawMessage `json:"result,omitempty"`
    Error  string          `json:"error,omitempty"`
}

// RemoteComposer 远程可调用的 Composer 包装
type RemoteComposer struct {
    composer Composer[any]
    obs      ObservationAdapter
}

func NewRemoteComposer(c Composer[any], obs ObservationAdapter) *RemoteComposer {
    return &RemoteComposer{composer: c, obs: obs}
}

// RegisterRemoteHandlers 注册远程调用 HTTP 端点
func (rc *RemoteComposer) RegisterRemoteHandlers(mux *http.ServeMux) {
    mux.HandleFunc("/api/rpc/run", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", 405)
            return
        }
        var req RemoteCallRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeJSON(w, 400, RemoteCallResponse{Error: err.Error()})
            return
        }
        ctx := context.WithValue(r.Context(), traceIDKey, TraceID(req.TraceID))
        result, steps, err := rc.composer.Run(ctx, req.Params)
        if err != nil {
            writeJSON(w, 500, RemoteCallResponse{Error: err.Error()})
            return
        }
        _ = steps
        resultJSON, _ := json.Marshal(result)
        writeJSON(w, 200, RemoteCallResponse{Result: resultJSON})
    })
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 2: 在 `App` 中集成 RemoteComposer**

在 `app.go` 的 `NewApp` 中添加：

```go
// 7. Remote RPC (if HTTP enabled)
if app.HTTP != nil {
    // 创建一个默认的远程 Composer 入口
    defaultPipeline := NewPipeline[any](obs)
    remote := NewRemoteComposer(defaultPipeline, obs)
    remote.RegisterRemoteHandlers(mux)
}
```

- [ ] **Step 3: 编译验证**

```bash
cd go-core && go build .
```

- [ ] **Step 4: Commit**

```bash
git add go-core/grpc_server.go
git commit -m "feat: add RemoteComposer — JSON-RPC style remote invocation over HTTP"
```

---

## Phase 3: 模式与工具扩展（Day 5-6）

### Task 3.1: FanOut / Debounce / Throttle 模式

**Files:**
- Modify: `go-core/composer.go`

**Goal:** 新增 3 个 Composer 模式，丰富组合模式库。

- [ ] **Step 1: 实现 FanOut**

```go
// FanOut 将输入广播到多个 Composer 并收集结果
type FanOut[T any] struct {
    branches []Composer[T]
    obs      ObservationAdapter
}

func NewFanOut[T any](obs ObservationAdapter, branches ...Composer[T]) *FanOut[T] {
    return &FanOut[T]{branches: branches, obs: obs}
}

func (fo *FanOut[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
    var allSteps []ExecutionStep
    type branchResult struct {
        result T
        steps  []ExecutionStep
        err    error
    }
    results := make([]branchResult, len(fo.branches))
    var wg sync.WaitGroup
    for i, b := range fo.branches {
        wg.Add(1)
        go func(idx int, branch Composer[T]) {
            defer wg.Done()
            r, s, e := branch.Run(ctx, input)
            results[idx] = branchResult{r, s, e}
        }(i, b)
    }
    wg.Wait()
    for _, br := range results {
        allSteps = append(allSteps, br.steps...)
        if br.err != nil {
            return input, allSteps, br.err
        }
    }
    return input, allSteps, nil
}
```

- [ ] **Step 2: 实现 Debounce**

```go
// Debounce 防抖 Composer —— 在静默期内忽略重复调用
type Debounce[T any] struct {
    inner    Composer[T]
    interval time.Duration
    lastCall time.Time
    mu       sync.Mutex
}

func NewDebounce[T any](inner Composer[T], interval time.Duration) *Debounce[T] {
    return &Debounce[T]{inner: inner, interval: interval}
}

func (db *Debounce[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
    db.mu.Lock()
    if time.Since(db.lastCall) < db.interval {
        db.mu.Unlock()
        return input, nil, nil // 跳过
    }
    db.lastCall = time.Now()
    db.mu.Unlock()
    return db.inner.Run(ctx, input)
}
```

- [ ] **Step 3: 实现 Throttle**

```go
// Throttle 节流 Composer —— 限制调用频率
type Throttle[T any] struct {
    inner    Composer[T]
    interval time.Duration
    lastCall time.Time
    mu       sync.Mutex
}

func NewThrottle[T any](inner Composer[T], interval time.Duration) *Throttle[T] {
    return &Throttle[T]{inner: inner, interval: interval}
}

func (th *Throttle[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
    th.mu.Lock()
    elapsed := time.Since(th.lastCall)
    if elapsed < th.interval {
        waitTime := th.interval - elapsed
        th.mu.Unlock()
        time.Sleep(waitTime)
    } else {
        th.mu.Unlock()
    }
    th.mu.Lock()
    th.lastCall = time.Now()
    th.mu.Unlock()
    return th.inner.Run(ctx, input)
}
```

- [ ] **Step 4: 编写测试**

```go
// 在 composer_test.go 或新建 composer_patterns_test.go 中追加
func TestFanOut(t *testing.T) {
    obs := NewInMemoryObservationAdapter()
    p1 := NewPipeline[int](obs, AtomAsStep[int, int](func(ctx context.Context, i int) (int, error) { return i + 1, nil }))
    p2 := NewPipeline[int](obs, AtomAsStep[int, int](func(ctx context.Context, i int) (int, error) { return i + 2, nil }))
    fo := NewFanOut[int](obs, p1, p2)
    result, _, err := fo.Run(context.Background(), 0)
    if err != nil {
        t.Fatal(err)
    }
    _ = result
}

func TestDebounce(t *testing.T) {
    count := 0
    p := NewPipeline[int](nil, AtomAsStep[int, int](func(ctx context.Context, i int) (int, error) { count++; return i, nil }))
    db := NewDebounce[int](p, 100*time.Millisecond)
    db.Run(context.Background(), 0)
    db.Run(context.Background(), 0) // 应跳过
    time.Sleep(150 * time.Millisecond)
    db.Run(context.Background(), 0) // 应执行
    if count != 2 {
        t.Errorf("expected 2 calls, got %d", count)
    }
}
```

- [ ] **Step 5: 运行测试**

```bash
cd go-core && go test -run "TestFanOut|TestDebounce|TestThrottle" -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add go-core/composer.go
git commit -m "feat: add FanOut, Debounce, Throttle composer patterns"
```

---

### Task 3.2: arch-manager 导出增强 — PlantUML / DOT

**Files:**
- Modify: `cmd/arch-manager/main.go`

**Goal:** 为 `/api/export` 增加 `plantuml` 和 `dot` 格式支持。

- [ ] **Step 1: 添加 PlantUML 导出**

在 `handleExport` 中增加：

```go
case "plantuml":
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Content-Disposition", "attachment; filename=architecture.puml")
    writePlantUML(w, data)

case "dot":
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Content-Disposition", "attachment; filename=architecture.dot")
    writeDOT(w, data)
```

- [ ] **Step 2: 实现 `writePlantUML` 和 `writeDOT`**

```go
func writePlantUML(w io.Writer, data *ArchData) {
    fmt.Fprintln(w, "@startuml")
    fmt.Fprintln(w, "skinparam backgroundColor #0a0a0a")
    fmt.Fprintln(w, "skinparam defaultTextColor #f5f5f7")
    for _, l := range data.Layers {
        fmt.Fprintf(w, "package \"%s %s\" as %s #%s22 {\n", l.Layer, l.Name, l.Layer, l.Color)
        for _, f := range data.Files {
            if f.Layer == l.Layer {
                fmt.Fprintf(w, "  [%s]\n", f.Name)
            }
        }
        fmt.Fprintln(w, "}")
    }
    for _, f := range data.Files {
        for _, d := range f.DependsOn {
            fmt.Fprintf(w, "[%s] --> [%s]\n", f.Name, d)
        }
    }
    fmt.Fprintln(w, "@enduml")
}

func writeDOT(w io.Writer, data *ArchData) {
    fmt.Fprintln(w, "digraph Architecture {")
    fmt.Fprintln(w, "  bgcolor=\"#0a0a0a\";")
    fmt.Fprintln(w, "  node [style=filled,fontcolor=\"#f5f5f7\"];")
    for _, l := range data.Layers {
        fmt.Fprintf(w, "  subgraph cluster_%s { label=\"%s %s\"; color=\"%s\"; }\n", l.Layer, l.Layer, l.Name, l.Color)
    }
    for _, f := range data.Files {
        fmt.Fprintf(w, "  \"%s\" [fillcolor=\"%s22\"];\n", f.Name, f.Layer)
    }
    for _, f := range data.Files {
        for _, d := range f.DependsOn {
            fmt.Fprintf(w, "  \"%s\" -> \"%s\";\n", f.Name, d)
        }
    }
    fmt.Fprintln(w, "}")
}
```

- [ ] **Step 3: 编译并重启服务器**

```bash
cd cmd/arch-manager && go build -o ../../arch-manager.exe .
```

- [ ] **Step 4: 验证导出端点**

```bash
curl http://localhost:8090/api/export?format=plantuml
curl http://localhost:8090/api/export?format=dot
```

- [ ] **Step 5: Commit**

```bash
git add cmd/arch-manager/main.go
git commit -m "feat: add PlantUML and DOT export formats to arch-manager"
```

---

### Task 3.3: CI 流水线增强

**Files:**
- Modify: `.github/workflows/ci.yml`

**Goal:** 增强 CI 流水线，添加 lint、覆盖率检查、benchmark 回归检测。

- [ ] **Step 1: 更新 CI 配置**

```yaml
name: CI

on:
  push:
    branches: [master, main]
  pull_request:
    branches: [master, main]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.22', '1.23']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
      - name: Lint
        run: |
          cd go-core
          go vet ./...
      - name: Test with coverage
        run: |
          cd go-core
          go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
      - name: Coverage report
        run: |
          cd go-core
          go tool cover -func=coverage.out | tail -1
      - name: Benchmark regression
        run: |
          cd go-core
          go test -bench=. -benchmem -count=1 ./...
      - name: Build examples
        run: |
          cd examples/calculator && go build .
          cd ../../examples/task_scheduler && go build .
      - name: Build arch-manager
        run: |
          cd cmd/arch-manager && go build .
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add lint, coverage, benchmark regression to CI pipeline"
```

---

## Phase 4: 前端深化（Day 7-8）

### Task 4.1: WebSocket 实时推送

**Files:**
- Modify: `arch-manager.html`
- Modify: `cmd/arch-manager/main.go`

**Goal:** 将 5 秒轮询替换为 WebSocket 推送，实现毫秒级变更感知。

- [ ] **Step 1: 后端添加 WebSocket 端点**

在 `main.go` 中添加：

```go
import (
    "net/http"
    "sync"
    "golang.org/x/net/websocket" // 需要 go get
)

var wsClients sync.Map

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 使用标准库实现简易 WebSocket（避免外部依赖）
    // 或使用 gorilla/websocket
}
```

**Note:** 由于项目零外部依赖策略，采用 **Server-Sent Events (SSE)** 替代 WebSocket。

```go
// SSE handler
func handleSSE(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", 500)
        return
    }

    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case <-ticker.C:
            data, _ := json.Marshal(currentData)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

- [ ] **Step 2: 前端添加 SSE 客户端**

```javascript
// 替换 setInterval 轮询
function initSSE() {
    const es = new EventSource('/api/sse');
    es.onmessage = function(e) {
        archData = JSON.parse(e.data);
        renderAll();
        updateStatus('ok', '实时');
    };
    es.onerror = function() {
        updateStatus('error', 'SSE断开');
        setTimeout(initSSE, 3000);
    };
}
```

- [ ] **Step 3: 编译重启验证**

- [ ] **Step 4: Commit**

---

### Task 4.2: 代码热力图 — 文件复杂度可视化

**Files:**
- Modify: `arch-manager.html`

**Goal:** 在仪表盘中增加文件复杂度热力图，按复杂度/行数/改动频率渲染。

- [ ] **Step 1: 添加热力图面板**

在 `renderDashboard` 中增加：

```html
<div class="chart-box">
    <div class="chart-title">文件复杂度热力图</div>
    <div class="chart-container" id="chartHeatmap"></div>
</div>
```

- [ ] **Step 2: 实现 ECharts 热力图**

```javascript
function renderHeatmap() {
    const data = archData.files.map((f, i) => {
        const complexity = Math.min(f.symbols.length / Math.max(f.lines, 1) * 100, 100);
        return [f.layer, f.name, complexity];
    });
    const heatChart = echarts.init(document.getElementById('chartHeatmap'));
    heatChart.setOption({
        tooltip: { formatter: p => `${p.data[1]}<br/>复杂度: ${p.data[2].toFixed(1)}` },
        xAxis: { type: 'category', data: [...new Set(data.map(d => d[0]))].sort() },
        yAxis: { type: 'category', data: data.map(d => d[1]) },
        visualMap: { min: 0, max: 100, inRange: { color: ['#30d158', '#ff9f0a', '#ff453a'] } },
        series: [{ type: 'heatmap', data: data }]
    });
}
```

- [ ] **Step 3: 验证**

---

### Task 4.3: 架构演进时间线

**Files:**
- Modify: `arch-manager.html`
- Modify: `cmd/arch-manager/main.go`

**Goal:** 记录每次分析的快照，展示架构健康度随时间变化的趋势图。

- [ ] **Step 1: 后端添加快照历史**

```go
var history []HealthScore // 追加到内存中
```

- [ ] **Step 2: 前端添加趋势图**

```javascript
function renderTimeline() {
    const dates = history.map(h => h.timestamp);
    const scores = history.map(h => h.overall);
    const chart = echarts.init(document.getElementById('chartTimeline'));
    chart.setOption({
        xAxis: { type: 'category', data: dates },
        yAxis: { type: 'value', min: 0, max: 100 },
        series: [{
            type: 'line',
            data: scores,
            areaStyle: { color: 'rgba(0,122,255,0.1)' },
            lineStyle: { color: '#007aff' }
        }]
    });
}
```

- [ ] **Step 3: 验证**

---

## 执行顺序总结

```
Phase 1 (Day 1-2):  Task 1.1 → Task 1.2 → Task 1.3 → Task 1.4
Phase 2 (Day 3-4):  Task 2.1 → Task 2.2
Phase 3 (Day 5-6):  Task 3.1 → Task 3.2 → Task 3.3
Phase 4 (Day 7-8):  Task 4.1 → Task 4.2 → Task 4.3
```

每个 Phase 内部可串行执行，Phase 之间需要前序依赖（Phase 2 依赖 Phase 1 的 StorageBackend）。