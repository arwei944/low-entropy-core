package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ──────────────────────────────────────────────
// JSONSnapshotAdapter — file-based persistence
// ──────────────────────────────────────────────

// JSONSnapshotAdapter persists DevSnapshots as JSON files on disk.
// Snapshots are stored in the directory "snapshots/" with filenames
// following the pattern "{checksum}.json".
type JSONSnapshotAdapter struct {
	mu       sync.RWMutex
	baseDir  string
}

// NewJSONSnapshotAdapter creates a new JSON file adapter.
// The baseDir defaults to "snapshots" if empty.
func NewJSONSnapshotAdapter(baseDir string) *JSONSnapshotAdapter {
	if baseDir == "" {
		baseDir = "snapshots"
	}
	return &JSONSnapshotAdapter{baseDir: baseDir}
}

// CreateSnapshot serializes the DevSnapshot to JSON and saves it to disk.
// Returns the HandoffSnapshot with the checksum as the token.
func (a *JSONSnapshotAdapter) CreateSnapshot(state *DevSnapshot) (HandoffSnapshot, error) {
	checksum, err := state.ComputeChecksum()
	if err != nil {
		return HandoffSnapshot{}, fmt.Errorf("failed to compute checksum: %w", err)
	}

	data, err := state.ToJSON()
	if err != nil {
		return HandoffSnapshot{}, fmt.Errorf("failed to serialize snapshot: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(a.baseDir, 0755); err != nil {
		return HandoffSnapshot{}, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	filePath := filepath.Join(a.baseDir, checksum+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return HandoffSnapshot{}, fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return HandoffSnapshot{
		Token: checksum,
		State: state,
		Meta: map[string]string{
			"file_path": filePath,
			"created_at": state.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}, nil
}

// RestoreSnapshot reads a DevSnapshot from disk by checksum.
func (a *JSONSnapshotAdapter) RestoreSnapshot(snap HandoffSnapshot) (*DevSnapshot, error) {
	checksum := snap.Token
	if checksum == "" {
		return nil, fmt.Errorf("snapshot token (checksum) is empty")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	filePath := filepath.Join(a.baseDir, checksum+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	state, err := DevSnapshotFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize snapshot: %w", err)
	}

	// Verify integrity
	if !state.VerifyChecksum() {
		return nil, fmt.Errorf("snapshot integrity check failed: checksum mismatch")
	}

	return state, nil
}

// DeleteSnapshot removes a snapshot file from disk.
func (a *JSONSnapshotAdapter) DeleteSnapshot(checksum string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	filePath := filepath.Join(a.baseDir, checksum+".json")
	return os.Remove(filePath)
}

// ──────────────────────────────────────────────
// InMemorySnapshotAdapter — memory-based persistence
// ──────────────────────────────────────────────

// InMemorySnapshotAdapter stores DevSnapshots in an in-memory map.
// Thread-safe. Suitable for testing and short-lived processes.
type InMemorySnapshotAdapter struct {
	mu    sync.RWMutex
	store map[string]*DevSnapshot
}

// NewInMemorySnapshotAdapter creates a new in-memory snapshot adapter.
func NewInMemorySnapshotAdapter() *InMemorySnapshotAdapter {
	return &InMemorySnapshotAdapter{
		store: make(map[string]*DevSnapshot),
	}
}

// CreateSnapshot stores the DevSnapshot in memory.
func (a *InMemorySnapshotAdapter) CreateSnapshot(state *DevSnapshot) (HandoffSnapshot, error) {
	checksum, err := state.ComputeChecksum()
	if err != nil {
		return HandoffSnapshot{}, fmt.Errorf("failed to compute checksum: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Store a copy
	copyData, _ := state.ToJSON()
	copyState, _ := DevSnapshotFromJSON(copyData)
	a.store[checksum] = copyState

	return HandoffSnapshot{
		Token: checksum,
		State: state,
		Meta: map[string]string{
			"storage": "in-memory",
		},
	}, nil
}

// RestoreSnapshot retrieves a DevSnapshot by checksum.
func (a *InMemorySnapshotAdapter) RestoreSnapshot(snap HandoffSnapshot) (*DevSnapshot, error) {
	checksum := snap.Token
	if checksum == "" {
		return nil, fmt.Errorf("snapshot token (checksum) is empty")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	state, ok := a.store[checksum]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: checksum=%s", checksum)
	}

	// Return a copy
	copyData, _ := state.ToJSON()
	copyState, err := DevSnapshotFromJSON(copyData)
	if err != nil {
		return nil, fmt.Errorf("failed to copy snapshot: %w", err)
	}
	return copyState, nil
}

// DeleteSnapshot removes a snapshot from memory.
func (a *InMemorySnapshotAdapter) DeleteSnapshot(checksum string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.store, checksum)
}

// Count returns the number of snapshots in the store.
func (a *InMemorySnapshotAdapter) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.store)
}

// ──────────────────────────────────────────────
// SnapshotPersistence helper
// ──────────────────────────────────────────────

// SnapshotPersistence is a unified interface for snapshot storage.
// Both JSONSnapshotAdapter and InMemorySnapshotAdapter implement this.
type SnapshotPersistence interface {
	CreateSnapshot(state *DevSnapshot) (HandoffSnapshot, error)
	RestoreSnapshot(snap HandoffSnapshot) (*DevSnapshot, error)
}

// Ensure interface compliance
var _ SnapshotPersistence = (*JSONSnapshotAdapter)(nil)
var _ SnapshotPersistence = (*InMemorySnapshotAdapter)(nil)

// SnapshotCreateAdapter wraps the CreateSnapshot operation as an Adapter.
type SnapshotCreateAdapter struct {
	persistence SnapshotPersistence
}

// NewSnapshotCreateAdapter creates an adapter for snapshot creation.
func NewSnapshotCreateAdapter(p SnapshotPersistence) *SnapshotCreateAdapter {
	return &SnapshotCreateAdapter{persistence: p}
}

// Execute implements Adapter[*DevSnapshot, HandoffSnapshot].
func (a *SnapshotCreateAdapter) Execute(ctx context.Context, input *DevSnapshot) (HandoffSnapshot, error) {
	return a.persistence.CreateSnapshot(input)
}

// SnapshotRestoreAdapter wraps the RestoreSnapshot operation as an Adapter.
type SnapshotRestoreAdapter struct {
	persistence SnapshotPersistence
}

// NewSnapshotRestoreAdapter creates an adapter for snapshot restoration.
func NewSnapshotRestoreAdapter(p SnapshotPersistence) *SnapshotRestoreAdapter {
	return &SnapshotRestoreAdapter{persistence: p}
}

// Execute implements Adapter[HandoffSnapshot, *DevSnapshot].
func (a *SnapshotRestoreAdapter) Execute(ctx context.Context, input HandoffSnapshot) (*DevSnapshot, error) {
	return a.persistence.RestoreSnapshot(input)
}