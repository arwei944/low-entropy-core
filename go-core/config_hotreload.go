package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// HotReload — pipeline configuration hot-reload
// ──────────────────────────────────────────────

// HotReload manages hot-reloading of pipeline configurations.
// When a config file changes, it rebuilds the Composer and atomically
// swaps the active pipeline.
//
// Since we do not depend on fsnotify, HotReload uses a polling approach:
// it periodically reads the config file, computes its SHA256 hash, and
// compares with the previous hash. When the hash changes, it re-parses
// the config, rebuilds the pipeline, and atomically swaps the current
// Composer under a write lock.
//
// The old pipeline is NOT killed — it continues to serve in-flight
// requests. This is a graceful approach where the old pipeline
// naturally drains.
type HotReload struct {
	mu         sync.RWMutex
	current    Composer[any]
	builder    *PipelineBuilder
	configPath string
	env        string
	obs        ObservationAdapter
	cancel     context.CancelFunc
	done       chan struct{}
}

// NewHotReload creates a new HotReload instance.
//
// configPath is the path to the JSON config file.
// builder is the PipelineBuilder used to build pipelines.
// obs is the observation adapter for recording config changes.
func NewHotReload(configPath string, builder *PipelineBuilder, env string, obs ObservationAdapter) *HotReload {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &HotReload{
		configPath: configPath,
		builder:    builder,
		env:        env,
		obs:        obs,
		done:       make(chan struct{}),
	}
}

// Start begins watching the config file for changes.
//
// It reads the config file once synchronously, builds the initial pipeline,
// and returns it. Then it launches a background goroutine that periodically
// checks the file for changes at the given checkInterval.
//
// On each change, the pipeline is rebuilt and atomically swapped in.
// A ConfigChange ExecutionStep is emitted via the observation adapter.
//
// Start must only be called once. Calling Start multiple times on the
// same HotReload instance is undefined behavior.
func (h *HotReload) Start(ctx context.Context, checkInterval time.Duration) (Composer[any], error) {
	// ── Build the initial pipeline ──
	initial, initialHash, err := h.buildFromFile()
	if err != nil {
		return nil, fmt.Errorf("hotreload: initial build failed: %w", err)
	}

	h.mu.Lock()
	h.current = initial
	h.mu.Unlock()

	// ── Launch the watcher goroutine ──
	ctx, h.cancel = context.WithCancel(ctx)

	go h.watch(ctx, checkInterval, initialHash)

	return initial, nil
}

// Stop gracefully stops the hot reload watcher.
// It cancels the internal context, waits for the watcher goroutine
// to finish, and allows any in-progress reload to complete.
func (h *HotReload) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	<-h.done
}

// Current returns the current active Composer.
// This is safe for concurrent use — callers can invoke Run on the
// returned Composer while the watcher is still running.
func (h *HotReload) Current() Composer[any] {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current
}

// ──────────────────────────────────────────────
// Internal watcher logic
// ──────────────────────────────────────────────

// watch is the background goroutine that polls the config file
// for changes. It runs until the context is cancelled.
func (h *HotReload) watch(ctx context.Context, checkInterval time.Duration, lastHash string) {
	defer close(h.done)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentHash, err := computeFileHash(h.configPath)
			if err != nil {
				// Log the error as an observation step but keep watching
				errStep := NewExecutionStep("HotReload", "hash", err.Error(), "ConfigChange")
				errStep.Error = NewStepError("HASH_COMPUTE_FAILED", err.Error(), true)
				h.obs.Record([]ExecutionStep{errStep})
				continue
			}

			if currentHash == lastHash {
				// No change — skip
				continue
			}

			// ── Hash changed — rebuild the pipeline ──
			newPipeline, _, err := h.buildFromFile()
			if err != nil {
				errStep := NewExecutionStep("HotReload", "rebuild", err.Error(), "ConfigChange")
				errStep.Error = NewStepError("REBUILD_FAILED", err.Error(), true)
				h.obs.Record([]ExecutionStep{errStep})
				continue
			}

			// ── Atomically swap the current Composer ──
			h.mu.Lock()
			old := h.current
			h.current = newPipeline
			h.mu.Unlock()

			// ── Record the config change event ──
			changeStep := NewConfigChangeStep(lastHash, currentHash)
			h.obs.Record([]ExecutionStep{changeStep})

			lastHash = currentHash

			// The old pipeline (old) is intentionally NOT stopped.
			// It continues to serve in-flight requests and will
			// be garbage collected when no longer referenced.
			_ = old
		}
	}
}

// buildFromFile reads the config file, parses it, and builds a pipeline
// using the PipelineBuilder. Returns the resulting Composer, the file's
// SHA256 hash, and any error.
func (h *HotReload) buildFromFile() (Composer[any], string, error) {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading config file: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	config, err := ParseConfig(data)
	if err != nil {
		return nil, "", fmt.Errorf("parsing config: %w", err)
	}

	pipeline, err := h.builder.Build(config, h.env)
	if err != nil {
		return nil, "", fmt.Errorf("building pipeline: %w", err)
	}

	return pipeline, hash, nil
}

// ──────────────────────────────────────────────
// File hashing
// ──────────────────────────────────────────────

// computeFileHash computes the SHA256 hash of a file's contents.
// It returns the hex-encoded hash string, or an error if the file
// cannot be read.
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("computeFileHash: open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("computeFileHash: read: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ──────────────────────────────────────────────
// Config change observation
// ──────────────────────────────────────────────

// NewConfigChangeStep creates an ExecutionStep for a config change event.
// The step records the old and new file hashes in its metadata so that
// the observation layer can correlate config changes with their effects
// on pipeline execution.
func NewConfigChangeStep(oldHash, newHash string) ExecutionStep {
	step := NewExecutionStep("HotReload", "reload", "config file changed", "ConfigChange")
	step.Metadata = map[string]interface{}{
		"old_hash": oldHash,
		"new_hash": newHash,
	}
	return step
}