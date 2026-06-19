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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
// JWT Service
// ============================================================================

// JWTService 管理 JWT 的创建和验证。
type JWTService struct {
	secret    []byte
	issuer    string
	ttl       time.Duration
	refreshTTL time.Duration

	mu        sync.RWMutex
	blacklist map[string]time.Time // jti -> revoked at
}

// JWTConfig JWT 服务配置。
type JWTConfig struct {
	Secret     []byte        // 签名密钥
	Issuer     string        // 签发者
	TTL        time.Duration // Token 有效期
	RefreshTTL time.Duration // Refresh Token 有效期
}

// NewJWTService 创建 JWT 服务。
func NewJWTService(cfg JWTConfig) *JWTService {
	if cfg.TTL <= 0 {
		cfg.TTL = 1 * time.Hour
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 24 * time.Hour
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "low-entropy-core"
	}

	return &JWTService{
		secret:     cfg.Secret,
		issuer:     cfg.Issuer,
		ttl:        cfg.TTL,
		refreshTTL: cfg.RefreshTTL,
		blacklist:  make(map[string]time.Time),
	}
}

// IssueToken 签发新的 JWT。
func (s *JWTService) IssueToken(subject string, roles, permissions []string) (*JWTToken, error) {
	now := time.Now()
	claims := JWTClaims{
		Subject:     subject,
		Issuer:      s.issuer,
		IssuedAt:    now.Unix(),
		ExpiresAt:   now.Add(s.ttl).Unix(),
		JWTID:       generateJWTID(),
		Roles:       roles,
		Permissions: permissions,
	}
	return s.sign(claims)
}

// IssueRefreshToken 签发刷新 Token（更长有效期）。
func (s *JWTService) IssueRefreshToken(subject string) (*JWTToken, error) {
	now := time.Now()
	claims := JWTClaims{
		Subject:   subject,
		Issuer:    s.issuer,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.refreshTTL).Unix(),
		JWTID:     generateJWTID(),
	}
	return s.sign(claims)
}

// VerifyToken 验证 JWT 并返回 Claims。
// 检查签名、过期时间、黑名单。
func (s *JWTService) VerifyToken(rawToken string) (*JWTClaims, error) {
	token, err := ParseJWT(rawToken)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse: %w", err)
	}

	// 验证签名
	if !s.verifySignature(token) {
		return nil, fmt.Errorf("jwt: invalid signature")
	}

	// 验证过期
	if err := token.Claims.Valid(); err != nil {
		return nil, err
	}

	// 检查黑名单
	s.mu.RLock()
	revokedAt, revoked := s.blacklist[token.Claims.JWTID]
	s.mu.RUnlock()
	if revoked {
		return nil, fmt.Errorf("jwt: token revoked at %v", revokedAt)
	}

	return &token.Claims, nil
}

// RefreshToken 使用刷新 Token 签发新的访问 Token。
func (s *JWTService) RefreshToken(refreshToken string) (*JWTToken, error) {
	claims, err := s.VerifyToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("jwt: refresh: %w", err)
	}
	return s.IssueToken(claims.Subject, claims.Roles, claims.Permissions)
}

// RevokeToken 撤销 Token（加入黑名单）。
func (s *JWTService) RevokeToken(jti string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[jti] = time.Now()
}

// CleanupBlacklist 清理过期的黑名单条目。
func (s *JWTService) CleanupBlacklist() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for jti, revokedAt := range s.blacklist {
		if now.Sub(revokedAt) > s.ttl {
			delete(s.blacklist, jti)
		}
	}
}

// sign 对 Claims 签名生成 JWT。
func (s *JWTService) sign(claims JWTClaims) (*JWTToken, error) {
	header := JWTHeader{Algorithm: "HS256", Type: "JWT"}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("jwt: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("jwt: marshal claims: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	claimsB64 := base64URLEncode(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signingInput))
	signature := base64URLEncode(mac.Sum(nil))

	raw := signingInput + "." + signature

	return &JWTToken{
		Raw:       raw,
		Header:    header,
		Claims:    claims,
		Signature: signature,
	}, nil
}

// verifySignature 验证 JWT 签名。
func (s *JWTService) verifySignature(token *JWTToken) bool {
	parts := strings.Split(token.Raw, ".")
	if len(parts) != 3 {
		return false
	}
	signingInput := parts[0] + "." + parts[1]

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signingInput))
	expectedSig := base64URLEncode(mac.Sum(nil))

	return hmac.Equal([]byte(token.Signature), []byte(expectedSig))
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