// Package core — HTTP 安全中间件 (v0.9.0)
//
// 提供 HTTP 认证和授权中间件：
//   - JWT 认证中间件 — 从 Authorization header 提取并验证 JWT
//   - API Key 认证中间件 — 从 X-API-Key header 提取并验证 API Key
//   - RBAC 授权中间件 — 检查用户权限
//   - 多认证链 — 支持多种认证方式组合

package core

import (
	"context"
	"net/http"
	"strings"
)

// ============================================================================
// Context Keys
// ============================================================================

type authContextKey string

const (
	ctxKeyJWTClaims  authContextKey = "lc_jwt_claims"
	ctxKeyUserID     authContextKey = "lc_user_id"
	ctxKeyUserRoles  authContextKey = "lc_user_roles"
	ctxKeyAPIKey     authContextKey = "lc_api_key"
)

// JWTClaimsFromContext 从 Context 提取 JWT Claims。
func JWTClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(ctxKeyJWTClaims).(*JWTClaims)
	return claims, ok
}

// UserIDFromContext 从 Context 提取用户 ID。
func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKeyUserID).(string); ok {
		return id
	}
	return ""
}

// ============================================================================
// JWT 认证中间件
// ============================================================================

// JWTAuthMiddleware 从 Authorization header 提取并验证 JWT。
// 验证通过后将 Claims 和 UserID 注入 Context。
func JWTAuthMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 提取 Token
			token := extractBearerToken(r)
			if token == "" {
				writeAuthError(w, "missing or invalid Authorization header")
				return
			}

			// 验证 Token
			claims, err := jwtService.VerifyToken(token)
			if err != nil {
				writeAuthError(w, "invalid token: "+err.Error())
				return
			}

			// 注入 Context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKeyJWTClaims, claims)
			ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
			ctx = context.WithValue(ctx, ctxKeyUserRoles, claims.Roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWTAuthMiddleware 可选的 JWT 认证（不强制要求 Token）。
func OptionalJWTAuthMiddleware(jwtService *JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token != "" {
				claims, err := jwtService.VerifyToken(token)
				if err == nil {
					ctx := r.Context()
					ctx = context.WithValue(ctx, ctxKeyJWTClaims, claims)
					ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
					ctx = context.WithValue(ctx, ctxKeyUserRoles, claims.Roles)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// API Key 认证中间件
// ============================================================================

// APIKeyAuthMiddleware 从 X-API-Key header 提取并验证 API Key。
func APIKeyAuthMiddleware(manager *APIKeyManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				writeAuthError(w, "missing X-API-Key header")
				return
			}

			key, err := manager.ValidateKey(apiKey)
			if err != nil {
				writeAuthError(w, "invalid API key: "+err.Error())
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKeyAPIKey, key)
			ctx = context.WithValue(ctx, ctxKeyUserRoles, key.Roles)
			ctx = context.WithValue(ctx, ctxKeyUserID, "apikey:"+key.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================================================
// 多认证链中间件
// ============================================================================

// AuthChain 尝试多种认证方式，任意一种成功即可。
// 按顺序尝试：JWT → API Key。
func AuthChain(jwtService *JWTService, apiKeyManager *APIKeyManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 尝试 JWT
			if token := extractBearerToken(r); token != "" {
				claims, err := jwtService.VerifyToken(token)
				if err == nil {
					ctx := r.Context()
					ctx = context.WithValue(ctx, ctxKeyJWTClaims, claims)
					ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
					ctx = context.WithValue(ctx, ctxKeyUserRoles, claims.Roles)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// 尝试 API Key
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				key, err := apiKeyManager.ValidateKey(apiKey)
				if err == nil {
					ctx := r.Context()
					ctx = context.WithValue(ctx, ctxKeyAPIKey, key)
					ctx = context.WithValue(ctx, ctxKeyUserRoles, key.Roles)
					ctx = context.WithValue(ctx, ctxKeyUserID, "apikey:"+key.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			writeAuthError(w, "authentication required")
		})
	}
}

// ============================================================================
// RBAC 授权中间件
// ============================================================================

// RequireRole 检查用户是否拥有指定角色。
func RequireRole(engine *RBACEngine, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			roles, _ := req.Context().Value(ctxKeyUserRoles).([]string)
			for _, r := range roles {
				if r == role {
					next.ServeHTTP(w, req)
					return
				}
			}
			writeAuthError(w, "insufficient role: requires "+role)
		})
	}
}

// RequirePermission 检查用户是否拥有指定权限。
func RequirePermission(engine *RBACEngine, perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				writeAuthError(w, "not authenticated")
				return
			}
			if !engine.CheckPermission(userID, perm) {
				writeAuthError(w, "insufficient permission: requires "+string(perm))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

// extractBearerToken 从 Authorization header 提取 Bearer Token。
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}

// writeAuthError 返回认证错误响应。
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="low-entropy-core"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + message + `"}`))
}