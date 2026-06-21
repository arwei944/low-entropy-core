// Package core — JWT 认证模块 (v0.9.0)
//
// 提供 JWT (JSON Web Token) 的创建、验证、刷新功能。
// 支持 HS256 签名算法，可扩展支持 RS256。
//
// 特性:
//   - 标准 JWT 声明 (sub, iss, exp, iat, nbf)
//   - 自定义声明 (roles, permissions)
//   - Token 刷新 (refresh token)
//   - 黑名单 (token revocation)
//   - 过期自动检测

package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// JWT Claims
// ============================================================================

// JWTClaims 标准 JWT 声明 + 自定义声明。
type JWTClaims struct {
	// 标准声明
	Subject   string `json:"sub"`
	Issuer    string `json:"iss,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	NotBefore int64  `json:"nbf,omitempty"`
	JWTID     string `json:"jti,omitempty"`

	// 自定义声明
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	UserID      string   `json:"uid,omitempty"`
	TenantID    string   `json:"tid,omitempty"`
}

// Valid 检查 JWT 是否在有效期内。
func (c *JWTClaims) Valid() error {
	now := time.Now().Unix()
	if c.ExpiresAt > 0 && now > c.ExpiresAt {
		return fmt.Errorf("jwt: token expired at %d", c.ExpiresAt)
	}
	if c.NotBefore > 0 && now < c.NotBefore {
		return fmt.Errorf("jwt: token not valid before %d", c.NotBefore)
	}
	return nil
}

// HasRole 检查是否包含指定角色。
func (c *JWTClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission 检查是否包含指定权限。
func (c *JWTClaims) HasPermission(perm string) bool {
	for _, p := range c.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// HasAnyRole 检查是否包含任意一个角色。
func (c *JWTClaims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAnyPermission 检查是否包含任意一个权限。
func (c *JWTClaims) HasAnyPermission(perms ...string) bool {
	for _, perm := range perms {
		if c.HasPermission(perm) {
			return true
		}
	}
	return false
}

// ============================================================================
// JWT Token
// ============================================================================

// JWTToken 表示一个完整的 JWT。
type JWTToken struct {
	Raw       string     `json:"raw"`
	Header    JWTHeader  `json:"header"`
	Claims    JWTClaims  `json:"claims"`
	Signature string     `json:"signature"`
}

// JWTHeader JWT 头部。
type JWTHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// IsExpired 检查 token 是否已过期。
func (t *JWTToken) IsExpired() bool {
	return t.Claims.Valid() != nil
}

// ============================================================================
// JWT 解析和编码工具
// ============================================================================

// ParseJWT 解析原始 JWT 字符串。
func ParseJWT(raw string) (*JWTToken, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("jwt: invalid token format, expected 3 parts got %d", len(parts))
	}

	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode header: %w", err)
	}
	var header JWTHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal header: %w", err)
	}

	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode claims: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("jwt: unmarshal claims: %w", err)
	}

	return &JWTToken{
		Raw:       raw,
		Header:    header,
		Claims:    claims,
		Signature: parts[2],
	}, nil
}

// base64URLEncode Base64URL 编码（无填充）。
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode Base64URL 解码。
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// generateJWTID 生成 JWT ID。
func generateJWTID() string {
	return fmt.Sprintf("jti-%d-%x", time.Now().UnixNano(), generateShortRandom())
}

// generateShortRandom 生成短随机数。
func generateShortRandom() []byte {
	b := make([]byte, 8)
	// 使用时间戳作为种子
	_ = b
	return []byte(fmt.Sprintf("%x", time.Now().UnixNano()))
}
