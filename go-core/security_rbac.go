// Package core — RBAC 基于角色的访问控制 (v0.9.0)
//
// 提供完整的 RBAC 实现：
//   - 角色 (Role) — 权限集合
//   - 用户 (User) — 拥有一组角色
//   - 权限检查 — 支持通配符和层级权限
//   - 角色继承 — 子角色继承父角色所有权限
//
// 权限格式: resource:action (如 "pipeline:read", "pipeline:*", "*")
// 通配符:
//   - "*" 匹配所有权限
//   - "resource:*" 匹配该资源的所有操作

package core

import (
	"fmt"
	"strings"
	"sync"
)

// ============================================================================
// RBAC 核心类型
// ============================================================================

// Permission 权限字符串，格式: "resource:action"。
type Permission string

// Resource 返回权限的资源部分。
func (p Permission) Resource() string {
	parts := strings.SplitN(string(p), ":", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Action 返回权限的操作部分。
func (p Permission) Action() string {
	parts := strings.SplitN(string(p), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Match 检查权限是否匹配。
// 支持通配符: "*" 匹配所有, "resource:*" 匹配该资源所有操作。
func (p Permission) Match(target Permission) bool {
	s := string(p)
	t := string(target)

	// 完全匹配
	if s == t {
		return true
	}

	// 全局通配符
	if s == "*" {
		return true
	}

	// 资源级通配符
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 && parts[1] == "*" {
		targetParts := strings.SplitN(t, ":", 2)
		if len(targetParts) == 2 && parts[0] == targetParts[0] {
			return true
		}
	}

	return false
}

// ============================================================================
// Role
// ============================================================================

// Role 角色定义。
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Parents     []string     `json:"parents,omitempty"` // 父角色名称（继承）
}

// HasPermission 检查角色是否拥有指定权限（含父角色继承）。
func (r *Role) HasPermission(perm Permission, roles map[string]*Role) bool {
	// 直接检查
	for _, p := range r.Permissions {
		if p.Match(perm) {
			return true
		}
	}
	// 检查父角色
	for _, parentName := range r.Parents {
		if parent, ok := roles[parentName]; ok {
			if parent.HasPermission(perm, roles) {
				return true
			}
		}
	}
	return false
}

// ============================================================================
// RBAC 引擎
// ============================================================================

// RBACEngine 基于角色的访问控制引擎。
// 线程安全，支持动态角色和用户管理。
type RBACEngine struct {
	mu    sync.RWMutex
	roles map[string]*Role
	users map[string][]string // userID -> role names
}

// NewRBACEngine 创建 RBAC 引擎。
func NewRBACEngine() *RBACEngine {
	return &RBACEngine{
		roles: make(map[string]*Role),
		users: make(map[string][]string),
	}
}

// AddRole 添加角色。
func (e *RBACEngine) AddRole(role *Role) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.roles[role.Name] = role
}

// RemoveRole 移除角色。
func (e *RBACEngine) RemoveRole(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.roles, name)
}

// GetRole 获取角色定义。
func (e *RBACEngine) GetRole(name string) (*Role, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	role, ok := e.roles[name]
	return role, ok
}

// AssignRole 为用户分配角色。
func (e *RBACEngine) AssignRole(userID, roleName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.roles[roleName]; !ok {
		return fmt.Errorf("rbac: role %s does not exist", roleName)
	}

	roles := e.users[userID]
	// 避免重复
	for _, r := range roles {
		if r == roleName {
			return nil
		}
	}
	e.users[userID] = append(roles, roleName)
	return nil
}

// RevokeRole 撤销用户角色。
func (e *RBACEngine) RevokeRole(userID, roleName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	roles := e.users[userID]
	for i, r := range roles {
		if r == roleName {
			e.users[userID] = append(roles[:i], roles[i+1:]...)
			return
		}
	}
}

// GetUserRoles 获取用户的所有角色。
func (e *RBACEngine) GetUserRoles(userID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	roles := e.users[userID]
	result := make([]string, len(roles))
	copy(result, roles)
	return result
}

// CheckPermission 检查用户是否拥有指定权限。
func (e *RBACEngine) CheckPermission(userID string, perm Permission) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	roleNames := e.users[userID]
	for _, roleName := range roleNames {
		if role, ok := e.roles[roleName]; ok {
			if role.HasPermission(perm, e.roles) {
				return true
			}
		}
	}
	return false
}

// CheckAnyPermission 检查用户是否拥有任意一个权限。
func (e *RBACEngine) CheckAnyPermission(userID string, perms ...Permission) bool {
	for _, perm := range perms {
		if e.CheckPermission(userID, perm) {
			return true
		}
	}
	return false
}

// CheckAllPermissions 检查用户是否拥有所有权限。
func (e *RBACEngine) CheckAllPermissions(userID string, perms ...Permission) bool {
	for _, perm := range perms {
		if !e.CheckPermission(userID, perm) {
			return false
		}
	}
	return true
}

// ============================================================================
// 预定义角色
// ============================================================================

// DefaultRoles 返回预定义的默认角色。
func DefaultRoles() map[string]*Role {
	return map[string]*Role{
		"admin": {
			Name:        "admin",
			Description: "超级管理员，拥有所有权限",
			Permissions: []Permission{"*"},
		},
		"operator": {
			Name:        "operator",
			Description: "运维操作员",
			Permissions: []Permission{
				"pipeline:read", "pipeline:write",
				"agent:read", "agent:manage",
				"observation:read",
				"config:read",
			},
		},
		"developer": {
			Name:        "developer",
			Description: "开发者",
			Permissions: []Permission{
				"pipeline:read", "pipeline:write",
				"agent:read",
				"observation:read",
			},
		},
		"viewer": {
			Name:        "viewer",
			Description: "只读观察者",
			Permissions: []Permission{
				"pipeline:read",
				"observation:read",
			},
		},
		"agent": {
			Name:        "agent",
			Description: "Agent 执行者",
			Permissions: []Permission{
				"agent:execute",
				"agent:report",
			},
		},
	}
}

// NewRBACEngineWithDefaults 创建带默认角色的 RBAC 引擎。
func NewRBACEngineWithDefaults() *RBACEngine {
	engine := NewRBACEngine()
	for _, role := range DefaultRoles() {
		engine.AddRole(role)
	}
	return engine
}