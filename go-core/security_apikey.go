// Package core — API Key 管理模块 (v0.9.0)
//
// 提供 API Key 的生成、验证、撤销功能。
// API Key 用于服务间认证，比 JWT 更轻量。
//
// 特性:
//   - 安全生成: 使用 crypto/rand 生成 32 字节密钥
//   - 哈希存储: 仅存储 SHA-256 哈希，不存储明文
//   - 前缀匹配: 支持通过前缀快速查找
//   - 过期管理: 支持设置过期时间
//   - 撤销: 支持即时撤销

package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// API Key
// ============================================================================

// APIKey 表示一个 API Key。
type APIKey struct {
	ID          string            `json:"id"`
	Prefix      string            `json:"prefix"`       // key 前 8 位（用于查找）
	Hash        string            `json:"-"`            // SHA-256 哈希（不序列化）
	Description string            `json:"description"`
	Roles       []string          `json:"roles"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   time.Time         `json:"expires_at,omitempty"`
	LastUsedAt  time.Time         `json:"last_used_at,omitempty"`
	Revoked     bool              `json:"revoked"`
	RevokedAt   time.Time         `json:"revoked_at,omitempty"`
}

// IsExpired 检查 API Key 是否已过期。
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(k.ExpiresAt)
}

// IsValid 检查 API Key 是否有效。
func (k *APIKey) IsValid() bool {
	return !k.Revoked && !k.IsExpired()
}

// ============================================================================
// API Key Manager
// ============================================================================

// APIKeyManager 管理 API Key 的生命周期。
type APIKeyManager struct {
	mu       sync.RWMutex
	keys     map[string]*APIKey // id -> key
	byPrefix map[string]string  // prefix -> id
}

// NewAPIKeyManager 创建 API Key 管理器。
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys:     make(map[string]*APIKey),
		byPrefix: make(map[string]string),
	}
}

// GenerateKey 生成新的 API Key。
// 返回原始 key（仅此一次可见）和 APIKey 对象。
func (m *APIKeyManager) GenerateKey(description string, roles []string, ttl time.Duration) (string, *APIKey, error) {
	// 生成 32 字节随机密钥
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("apikey: generate: %w", err)
	}
	rawKey := "lc_" + hex.EncodeToString(raw)

	// 计算哈希
	hash := hashAPIKey(rawKey)
	prefix := rawKey[:11] // "lc_" + 8 hex chars

	key := &APIKey{
		ID:          generateAPIKeyID(),
		Prefix:      prefix,
		Hash:        hash,
		Description: description,
		Roles:       roles,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
	}

	m.mu.Lock()
	m.keys[key.ID] = key
	m.byPrefix[prefix] = key.ID
	m.mu.Unlock()

	return rawKey, key, nil
}

// ValidateKey 验证 API Key 并返回对应的 APIKey 对象。
// 如果 key 无效、过期或已撤销，返回错误。
func (m *APIKeyManager) ValidateKey(rawKey string) (*APIKey, error) {
	if len(rawKey) < 11 {
		return nil, fmt.Errorf("apikey: invalid key format")
	}

	prefix := rawKey[:11]
	hash := hashAPIKey(rawKey)

	m.mu.RLock()
	id, ok := m.byPrefix[prefix]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("apikey: unknown key")
	}

	key, ok := m.keys[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("apikey: key not found")
	}

	// 使用恒定时间比较防止时序攻击
	if !hmac.Equal([]byte(key.Hash), []byte(hash)) {
		return nil, fmt.Errorf("apikey: invalid key")
	}

	if !key.IsValid() {
		if key.Revoked {
			return nil, fmt.Errorf("apikey: revoked")
		}
		return nil, fmt.Errorf("apikey: expired")
	}

	// 更新最后使用时间
	m.mu.Lock()
	if k, ok := m.keys[key.ID]; ok {
		k.LastUsedAt = time.Now()
	}
	m.mu.Unlock()

	return key, nil
}

// RevokeKey 撤销 API Key。
func (m *APIKeyManager) RevokeKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[id]
	if !ok {
		return fmt.Errorf("apikey: key %s not found", id)
	}
	key.Revoked = true
	key.RevokedAt = time.Now()
	return nil
}

// GetKey 获取 API Key 信息（不含原始密钥）。
func (m *APIKeyManager) GetKey(id string) (*APIKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, ok := m.keys[id]
	return key, ok
}

// ListKeys 列出所有 API Key。
func (m *APIKeyManager) ListKeys() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		result = append(result, key)
	}
	return result
}

// CleanupExpired 清理已过期的 Key。
func (m *APIKeyManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, key := range m.keys {
		if key.IsExpired() {
			delete(m.byPrefix, key.Prefix)
			delete(m.keys, id)
			count++
		}
	}
	return count
}

// ============================================================================
// 内部工具
// ============================================================================

// hashAPIKey 计算 API Key 的 SHA-256 哈希。
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// generateAPIKeyID 生成 API Key ID。
func generateAPIKeyID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "apikey_" + hex.EncodeToString(b)
}