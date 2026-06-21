//go:build lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — 存储辅助函数和统计
//
// 本文件包含 JSON 辅助方法、存储统计和 FileStorageBackend 的扩展功能。
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

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
	TotalKeys  int       `json:"total_keys"`
	TotalBytes int64     `json:"total_bytes"`
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
