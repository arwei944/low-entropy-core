//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

package core

import (
	"testing"
	"time"
)

func TestInMemoryIdempotentStore_SetGet(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 1*time.Minute)
	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %v", val)
	}
}

func TestInMemoryIdempotentStore_Miss(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected miss for nonexistent key")
	}
}

func TestInMemoryIdempotentStore_Delete(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 1*time.Minute)
	store.Delete("key1")

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestInMemoryIdempotentStore_Clear(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("k1", "v1", 1*time.Minute)
	store.Set("k2", "v2", 1*time.Minute)
	store.Clear()

	_, ok1 := store.Get("k1")
	_, ok2 := store.Get("k2")
	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestInMemoryIdempotentStore_Expiry(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestInMemoryIdempotentStore_Overwrite(t *testing.T) {
	store := NewInMemoryIdempotentStore()

	store.Set("key1", "old", 1*time.Minute)
	store.Set("key1", "new", 1*time.Minute)

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "new" {
		t.Errorf("expected 'new', got %v", val)
	}
}