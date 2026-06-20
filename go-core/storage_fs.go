//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 文件系统持久化后端 (Phase 1, Task 1.1)
//
// StorageBackend 定义了持久化存储的统一接口。
// FileStorageBackend 是文件系统实现，支持目录隔离和原子写入。
//
// 设计原则：
//   - 所有操作通过 context 支持超时和取消
//   - Save 使用原子写入（先写临时文件，再 rename）
//   - 目录结构：{baseDir}/{prefix}/{key}
//   - 线程安全：使用 sync.RWMutex 保护索引
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// FileStorageBackend — 文件系统实现
// ──────────────────────────────────────────────

// FileStorageBackend 是 StorageBackend 的文件系统实现。
// 每个 key 映射为一个文件：{baseDir}/{key}
// 支持原子写入：先写 .tmp 文件，再 rename。
type FileStorageBackend struct {
	mu      sync.RWMutex
	baseDir string
	closed  bool
}

// NewFileStorageBackend 创建文件系统存储后端。
// baseDir 必须是存在且可写的目录（不存在则自动创建）。
func NewFileStorageBackend(baseDir string) (*FileStorageBackend, error) {
	// 确保目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("storage: create base dir %s: %w", baseDir, err)
	}
	return &FileStorageBackend{
		baseDir: baseDir,
	}, nil
}

// filePath 将 key 转换为文件路径。
func (fs *FileStorageBackend) filePath(key string) string {
	// 安全检查：防止路径穿越
	cleanKey := filepath.Clean(key)
	if strings.HasPrefix(cleanKey, "..") {
		cleanKey = strings.TrimPrefix(cleanKey, "..")
		cleanKey = strings.TrimPrefix(cleanKey, string(filepath.Separator))
	}
	return filepath.Join(fs.baseDir, cleanKey)
}

// Save 原子写入数据到指定 key。
// 先写入 .tmp 文件，成功后再 rename 为正式文件。
func (fs *FileStorageBackend) Save(ctx context.Context, key string, data []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fs.mu.RLock()
	if fs.closed {
		fs.mu.RUnlock()
		return fmt.Errorf("storage: backend is closed")
	}
	fs.mu.RUnlock()

	filePath := fs.filePath(key)

	// 确保父目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("storage: create dir %s: %w", dir, err)
	}

	// 原子写入：先写临时文件
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("storage: write tmp %s: %w", tmpPath, err)
	}

	// 原子 rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath) // 清理临时文件
		return fmt.Errorf("storage: rename %s -> %s: %w", tmpPath, filePath, err)
	}

	return nil
}

// Load 从指定 key 加载数据。
func (fs *FileStorageBackend) Load(ctx context.Context, key string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	fs.mu.RLock()
	if fs.closed {
		fs.mu.RUnlock()
		return nil, fmt.Errorf("storage: backend is closed")
	}
	fs.mu.RUnlock()

	filePath := fs.filePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("storage: read %s: %w", filePath, err)
	}
	return data, nil
}

// Delete 删除指定 key 的数据。
func (fs *FileStorageBackend) Delete(ctx context.Context, key string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fs.mu.RLock()
	if fs.closed {
		fs.mu.RUnlock()
		return fmt.Errorf("storage: backend is closed")
	}
	fs.mu.RUnlock()

	filePath := fs.filePath(key)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // 幂等删除：不存在的文件视为成功
		}
		return fmt.Errorf("storage: delete %s: %w", filePath, err)
	}
	return nil
}

// List 列出指定前缀下的所有 key。
// 遍历文件系统，返回相对路径。
func (fs *FileStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	fs.mu.RLock()
	if fs.closed {
		fs.mu.RUnlock()
		return nil, fmt.Errorf("storage: backend is closed")
	}
	fs.mu.RUnlock()

	searchDir := fs.filePath(prefix)
	// 如果 prefix 不包含路径分隔符，搜索整个 baseDir
	if !strings.Contains(prefix, "/") {
		searchDir = fs.baseDir
	}

	var keys []string
	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 转换为相对路径（key），统一使用 / 分隔符
		relPath, err := filepath.Rel(fs.baseDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		// 过滤前缀
		if prefix == "" || strings.HasPrefix(relPath, prefix) {
			keys = append(keys, relPath)
		}
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: list %s: %w", prefix, err)
	}

	return keys, nil
}

// Close 关闭后端（标记为已关闭，阻止后续操作）。
func (fs *FileStorageBackend) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.closed = true
	return nil
}

// ──────────────────────────────────────────────
// SECTION 2.5: 批量操作和健康检查 (v0.9.0)
// ──────────────────────────────────────────────

// SaveBatch 批量保存数据。不保证原子性。
func (fs *FileStorageBackend) SaveBatch(ctx context.Context, entries map[string][]byte) error {
	for key, data := range entries {
		if err := fs.Save(ctx, key, data); err != nil {
			return fmt.Errorf("storage: save batch %s: %w", key, err)
		}
	}
	return nil
}

// LoadBatch 批量加载数据。
func (fs *FileStorageBackend) LoadBatch(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		data, err := fs.Load(ctx, key)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("storage: load batch %s: %w", key, err)
		}
		result[key] = data
	}
	return result, nil
}

// HealthCheck 检查文件系统后端健康状态。
func (fs *FileStorageBackend) HealthCheck(ctx context.Context) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if fs.closed {
		return fmt.Errorf("storage: backend is closed")
	}
	// 检查 baseDir 是否存在且可访问
	info, err := os.Stat(fs.baseDir)
	if err != nil {
		return fmt.Errorf("storage: base dir inaccessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("storage: base dir is not a directory")
	}
	return nil
}

// BaseDir 返回基础目录路径。
func (fs *FileStorageBackend) BaseDir() string {
	return fs.baseDir
}

// ──────────────────────────────────────────────
// SECTION 3: JSON 辅助方法
// ──────────────────────────────────────────────

// SaveJSON 将对象序列化为 JSON 并保存。
func SaveJSON(ctx context.Context, backend StorageBackend, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("storage: marshal %s: %w", key, err)
	}
	return backend.Save(ctx, key, data)
}

// LoadJSON 从 key 加载 JSON 并反序列化到 value。
func LoadJSON(ctx context.Context, backend StorageBackend, key string, value interface{}) error {
	data, err := backend.Load(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("storage: unmarshal %s: %w", key, err)
	}
	return nil
}

// ──────────────────────────────────────────────
// SECTION 4: 存储统计
// ──────────────────────────────────────────────

// StorageStat 存储统计信息。
type StorageStat struct {
	TotalKeys  int   `json:"total_keys"`
	TotalBytes int64 `json:"total_bytes"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Stat 返回存储统计信息。
func (fs *FileStorageBackend) Stat(ctx context.Context) (*StorageStat, error) {
	keys, err := fs.List(ctx, "")
	if err != nil {
		return nil, err
	}

	stat := &StorageStat{TotalKeys: len(keys), UpdatedAt: time.Now()}
	for _, key := range keys {
		info, err := os.Stat(fs.filePath(key))
		if err != nil {
			continue
		}
		stat.TotalBytes += info.Size()
	}
	return stat, nil
}