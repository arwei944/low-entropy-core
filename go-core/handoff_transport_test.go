//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInProcHandoffTransport_TransferReceive(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	data := []byte("test snapshot data")
	checksum := "abc123"

	err := transport.Transfer(ctx, checksum, data)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	received, err := transport.Receive(ctx, checksum)
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if string(received) != string(data) {
		t.Errorf("data mismatch: '%s' != '%s'", string(received), string(data))
	}
}

func TestInProcHandoffTransport_ReceiveNotFound(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	_, err := transport.Receive(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestInProcHandoffTransport_Delete(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	transport.Transfer(ctx, "abc", []byte("data"))
	transport.Delete("abc")

	_, err := transport.Receive(ctx, "abc")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInProcHandoffTransport_Count(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()

	transport.Transfer(ctx, "a", []byte("1"))
	transport.Transfer(ctx, "b", []byte("2"))

	if transport.Count() != 2 {
		t.Errorf("expected count=2, got %d", transport.Count())
	}
}

func TestInProcHandoffTransport_Concurrency(t *testing.T) {
	ctx := context.Background()
	transport := NewInProcHandoffTransport()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id)
			transport.Transfer(ctx, key, []byte("data"))
			transport.Receive(ctx, key)
		}(i)
	}
	wg.Wait()

	if transport.Count() != 100 {
		t.Errorf("expected 100 entries, got %d", transport.Count())
	}
}

func TestInMemorySnapshotAdapter_CreateRestore(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "done")
	snap.Artifacts = []Artifact{
		{Path: "a.go", Type: "file", Description: "main"},
	}
	snap.Pending = []WorkItem{
		{ID: "w-1", Title: "test", Priority: "high"},
	}

	adapter := NewInMemorySnapshotAdapter()

	handoff, err := adapter.CreateSnapshot(snap)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if handoff.Token == "" {
		t.Error("expected non-empty token")
	}

	restored, err := adapter.RestoreSnapshot(handoff)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if restored.TaskID != "task-1" {
		t.Errorf("TaskID mismatch: %s", restored.TaskID)
	}
	if len(restored.Artifacts) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(restored.Artifacts))
	}
	if len(restored.Pending) != 1 {
		t.Errorf("expected 1 work item, got %d", len(restored.Pending))
	}
}

func TestInMemorySnapshotAdapter_RestoreNotFound(t *testing.T) {
	adapter := NewInMemorySnapshotAdapter()
	_, err := adapter.RestoreSnapshot(HandoffSnapshot{Token: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestInMemorySnapshotAdapter_Delete(t *testing.T) {
	snap := NewDevSnapshot("task-1", "agent-a", "coding", "done")
	adapter := NewInMemorySnapshotAdapter()

	handoff, _ := adapter.CreateSnapshot(snap)
	adapter.DeleteSnapshot(handoff.Token)

	if adapter.Count() != 0 {
		t.Errorf("expected count=0, got %d", adapter.Count())
	}
}

func TestJSONSnapshotAdapter_CreateRestore(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "low-entropy-test-snapshots")
	defer os.RemoveAll(tmpDir)

	snap := NewDevSnapshot("task-json", "agent-b", "testing", "persisted")
	snap.Artifacts = []Artifact{
		{Path: "test.go", Type: "file", Description: "test file"},
	}

	adapter := NewJSONSnapshotAdapter(tmpDir)

	handoff, err := adapter.CreateSnapshot(snap)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restored, err := adapter.RestoreSnapshot(handoff)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if restored.TaskID != "task-json" {
		t.Errorf("expected TaskID='task-json', got '%s'", restored.TaskID)
	}
	if !restored.VerifyChecksum() {
		t.Error("restored snapshot checksum verification failed")
	}
}

func TestJSONSnapshotAdapter_Delete(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "low-entropy-test-snapshots-delete")
	defer os.RemoveAll(tmpDir)

	snap := NewDevSnapshot("task-del", "agent-c", "review", "delete test")
	adapter := NewJSONSnapshotAdapter(tmpDir)

	handoff, _ := adapter.CreateSnapshot(snap)
	err := adapter.DeleteSnapshot(handoff.Token)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = adapter.RestoreSnapshot(handoff)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}
