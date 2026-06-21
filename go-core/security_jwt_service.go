package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

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
