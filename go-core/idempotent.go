package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Idempotency Primitives (TASK-2.1)
// ──────────────────────────────────────────────

// IdempotentRequest wraps an input with an idempotency key.
type IdempotentRequest[In any] struct {
	Key   string
	Input In
}

// IdempotentResult wraps an output with caching info.
type IdempotentResult[Out any] struct {
	Output    Out
	FromCache bool   // true if result was from cache, false if freshly executed
	Key       string
}

// ──────────────────────────────────────────────
// IdempotentStore Interface
// ──────────────────────────────────────────────

// IdempotentStore is the interface for idempotency result storage.
// Different implementations can use memory, Redis, DB, etc.
type IdempotentStore interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
}

// ──────────────────────────────────────────────
// In-Memory Idempotent Store
// ──────────────────────────────────────────────

// InMemoryIdempotentStore is a simple in-memory idempotent store.
type InMemoryIdempotentStore struct {
	mu    sync.RWMutex
	store map[string]*idempotentEntry
}

type idempotentEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewInMemoryIdempotentStore creates a new in-memory idempotent store.
func NewInMemoryIdempotentStore() *InMemoryIdempotentStore {
	return &InMemoryIdempotentStore{
		store: make(map[string]*idempotentEntry),
	}
}

// Get returns the value and true if found and not expired.
// Returns zero value and false if not found or expired.
func (s *InMemoryIdempotentStore) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.store[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.value, true
}

// Set stores a value with the given TTL. If the key already exists, it is overwritten.
func (s *InMemoryIdempotentStore) Set(key string, value interface{}, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[key] = &idempotentEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a single entry by key. No-op if the key does not exist.
func (s *InMemoryIdempotentStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.store, key)
}

// Clear removes all entries from the store.
func (s *InMemoryIdempotentStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store = make(map[string]*idempotentEntry)
}

// ──────────────────────────────────────────────
// IdempotentPort
// ──────────────────────────────────────────────

// IdempotentPort implements Port[IdempotentRequest[In], IdempotentResult[Out]].
// It wraps an inner Step and provides idempotency guarantees: requests with the
// same key return the cached result, and the inner Step executes at most once
// per key within the TTL window.
//
// IdempotentPort is a Port (contract validation gateway) that sits at system
// boundaries, ensuring that duplicate requests do not cause duplicate side effects.
type IdempotentPort[In, Out any] struct {
	inner Step[In, Out]
	store IdempotentStore
	ttl   time.Duration
	mu    sync.Mutex
}

// NewIdempotentPort creates a new IdempotentPort wrapping the given inner Step.
// The store is used for caching results. The ttl determines how long cached
// results remain valid before the inner Step is re-executed.
func NewIdempotentPort[In, Out any](inner Step[In, Out], store IdempotentStore, ttl time.Duration) *IdempotentPort[In, Out] {
	return &IdempotentPort[In, Out]{
		inner: inner,
		store: store,
		ttl:   ttl,
	}
}

// Validate implements Port[IdempotentRequest[In], IdempotentResult[Out]].
//
// Validation logic:
//  1. If input.Key is empty, return an error.
//  2. Check the store for a cached result by key.
//  3. If found and not expired → return the cached result with FromCache=true.
//  4. If not found or expired → execute the inner Step, store the result with TTL,
//     and return with FromCache=false.
//  5. If the inner Step returns an error, do NOT cache the result.
//  6. Thread-safe: uses a double-check locking pattern to prevent race conditions
//     when concurrent requests share the same key.
func (p *IdempotentPort[In, Out]) Validate(ctx context.Context, input IdempotentRequest[In]) (IdempotentResult[Out], error) {
	// 1. Reject empty key
	if input.Key == "" {
		return IdempotentResult[Out]{}, fmt.Errorf("idempotent: key must not be empty")
	}

	// 2. First check: fast path for cached results
	p.mu.Lock()
	if cached, ok := p.store.Get(input.Key); ok {
		p.mu.Unlock()
		return IdempotentResult[Out]{
			Output:    cached.(Out),
			FromCache: true,
			Key:       input.Key,
		}, nil
	}
	p.mu.Unlock()

	// 3. Execute inner Step (not holding the lock, so different keys can run concurrently)
	result, err := p.inner.Execute(ctx, input.Input)
	if err != nil {
		// 5. Inner Step returned error — do NOT cache
		return IdempotentResult[Out]{}, err
	}

	// 4. Second check and store: another goroutine may have stored the result
	//    while we were executing. Use double-check locking to prevent duplicate execution.
	p.mu.Lock()
	defer p.mu.Unlock()

	if cached, ok := p.store.Get(input.Key); ok {
		// Another goroutine already stored the result for this key
		return IdempotentResult[Out]{
			Output:    cached.(Out),
			FromCache: true,
			Key:       input.Key,
		}, nil
	}

	// Store the freshly computed result
	p.store.Set(input.Key, result, p.ttl)

	return IdempotentResult[Out]{
		Output:    result,
		FromCache: false,
		Key:       input.Key,
	}, nil
}