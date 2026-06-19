// Package core — MemoryStorageBackend 内存存储实现 (v0.9.0)

package core

import (
	"context"
	"os"
	"strings"
	"sync"
)

// MemoryStorageBackend 是 StorageBackend 的内存实现。
// 用于测试和开发环境，不持久化数据。
type MemoryStorageBackend struct {
	mu     sync.RWMutex
	data   map[string][]byte
	closed bool
}

// NewMemoryStorageBackend 创建内存存储后端。
func NewMemoryStorageBackend() *MemoryStorageBackend {
	return &MemoryStorageBackend{
		data: make(map[string][]byte),
	}
}

func (m *MemoryStorageBackend) Save(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return os.ErrClosed
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[key] = cp
	return nil
}

func (m *MemoryStorageBackend) Load(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, os.ErrClosed
	}
	data, ok := m.data[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (m *MemoryStorageBackend) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return os.ErrClosed
	}
	delete(m.data, key)
	return nil
}

func (m *MemoryStorageBackend) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, os.ErrClosed
	}
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MemoryStorageBackend) SaveBatch(_ context.Context, entries map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return os.ErrClosed
	}
	for k, v := range entries {
		cp := make([]byte, len(v))
		copy(cp, v)
		m.data[k] = cp
	}
	return nil
}

func (m *MemoryStorageBackend) LoadBatch(_ context.Context, keys []string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, os.ErrClosed
	}
	result := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if data, ok := m.data[k]; ok {
			cp := make([]byte, len(data))
			copy(cp, data)
			result[k] = cp
		}
	}
	return result, nil
}

func (m *MemoryStorageBackend) HealthCheck(_ context.Context) error {
	if m.closed {
		return os.ErrClosed
	}
	return nil
}

func (m *MemoryStorageBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.data = nil
	return nil
}