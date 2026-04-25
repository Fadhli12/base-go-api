// Package permission provides Casbin-based permission enforcement.
package permission

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// Enforcer wraps the Casbin enforcer with custom functionality.
type Enforcer struct {
	enforcer *casbin.Enforcer
	adapter  *gormadapter.Adapter
	cache    *Cache
}

// RBACModel returns the Casbin model definition for RBAC with domains.
// Model definition:
//   - r = sub, dom, obj, act (subject, domain, object, action)
//   - p = sub, dom, obj, act (policy definition)
//   - g = _, _, _ (role definition with domains)
//   - e = some(where (p.eft == allow)) (policy effect)
//   - m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act (matchers)
func RBACModel() model.Model {
	m, _ := model.NewModelFromString(`
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
`)
	return m
}

// NewEnforcer creates a new Casbin enforcer with GORM adapter.
// It initializes the database adapter, creates the RBAC model with domains,
// and loads existing policies from the database.
func NewEnforcer(db *gorm.DB) (*Enforcer, error) {
	// Create GORM adapter for Casbin
	// This will create casbin_rule table if it doesn't exist
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create GORM adapter: %w", err)
	}

	// Create RBAC model with domains
	m := RBACModel()

	// Create enforcer with adapter and model
	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// Load policy from database
	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	slog.Info("Casbin enforcer initialized successfully")

	return &Enforcer{
		enforcer: e,
		adapter:  adapter,
	}, nil
}

// Enforce checks if a subject can perform an action on an object in a domain.
// Returns true if the policy allows the action, false otherwise.
func (e *Enforcer) Enforce(sub, dom, obj, act string) (bool, error) {
	result, err := e.enforcer.Enforce(sub, dom, obj, act)
	if err != nil {
		return false, fmt.Errorf("enforcement failed: %w", err)
	}
	return result, nil
}

// LoadPolicy reloads all policies from the database.
// This is useful when policies are changed externally.
func (e *Enforcer) LoadPolicy() error {
	if err := e.enforcer.LoadPolicy(); err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}
	slog.Debug("Policy reloaded successfully")
	return nil
}

// SavePolicy saves all policies to the database.
func (e *Enforcer) SavePolicy() error {
	if err := e.enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}
	slog.Debug("Policy saved successfully")
	return nil
}

// ClearPolicy clears all policies from memory.
func (e *Enforcer) ClearPolicy() {
	e.enforcer.ClearPolicy()
	slog.Debug("Policy cleared from memory")
}

// AddPolicy adds a policy rule for a subject on an object in a domain.
func (e *Enforcer) AddPolicy(sub, dom, obj, act string) error {
	success, err := e.enforcer.AddPolicy(sub, dom, obj, act)
	if err != nil {
		return fmt.Errorf("failed to add policy: %w", err)
	}
	if !success {
		slog.Debug("Policy already exists", "sub", sub, "dom", dom, "obj", obj, "act", act)
	} else {
		slog.Debug("Policy added", "sub", sub, "dom", dom, "obj", obj, "act", act)
	}
	return nil
}

// RemovePolicy removes a policy rule for a subject on an object in a domain.
func (e *Enforcer) RemovePolicy(sub, dom, obj, act string) error {
	success, err := e.enforcer.RemovePolicy(sub, dom, obj, act)
	if err != nil {
		return fmt.Errorf("failed to remove policy: %w", err)
	}
	if !success {
		slog.Debug("Policy did not exist", "sub", sub, "dom", dom, "obj", obj, "act", act)
	} else {
		slog.Debug("Policy removed", "sub", sub, "dom", dom, "obj", obj, "act", act)
	}
	return nil
}

// AddRoleForUser assigns a role to a user in a domain.
func (e *Enforcer) AddRoleForUser(user, role, domain string) error {
	success, err := e.enforcer.AddRoleForUser(user, role, domain)
	if err != nil {
		return fmt.Errorf("failed to add role for user: %w", err)
	}
	if !success {
		slog.Debug("Role already assigned",
			"user", user, "role", role, "domain", domain)
	} else {
		slog.Debug("Role assigned", "user", user, "role", role, "domain", domain)
	}
	return nil
}

// RemoveRoleForUser removes a role from a user in a domain.
func (e *Enforcer) RemoveRoleForUser(user, role, domain string) error {
	success, err := e.enforcer.DeleteRoleForUser(user, role, domain)
	if err != nil {
		return fmt.Errorf("failed to remove role for user: %w", err)
	}
	if !success {
		slog.Debug("Role was not assigned",
			"user", user, "role", role, "domain", domain)
	} else {
		slog.Debug("Role removed", "user", user, "role", role, "domain", domain)
	}
	return nil
}

// GetRolesForUser gets all roles for a user in a domain.
func (e *Enforcer) GetRolesForUser(user, domain string) ([]string, error) {
	roles, err := e.enforcer.GetRolesForUser(user, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles for user: %w", err)
	}
	return roles, nil
}

// GetUsersForRole gets all users that have a role in a domain.
func (e *Enforcer) GetUsersForRole(role, domain string) ([]string, error) {
	users, err := e.enforcer.GetUsersForRole(role, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get users for role: %w", err)
	}
	return users, nil
}

// HasRoleForUser checks if a user has a role in a domain.
func (e *Enforcer) HasRoleForUser(user, role, domain string) (bool, error) {
	hasRole, err := e.enforcer.HasRoleForUser(user, role, domain)
	if err != nil {
		return false, fmt.Errorf("failed to check role for user: %w", err)
	}
	return hasRole, nil
}

// GetPermissionsForUser gets all permissions for a user in a domain.
func (e *Enforcer) GetPermissionsForUser(user, domain string) ([][]string, error) {
	permissions, err := e.enforcer.GetPermissionsForUser(user, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions for user: %w", err)
	}
	return permissions, nil
}

// GetImplicitPermissionsForUser gets all permissions for a user including inherited roles.
func (e *Enforcer) GetImplicitPermissionsForUser(user, domain string) ([][]string, error) {
	permissions, err := e.enforcer.GetImplicitPermissionsForUser(user, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit permissions for user: %w", err)
	}
	return permissions, nil
}

// GetImplicitRolesForUser gets all roles for a user including inherited roles.
func (e *Enforcer) GetImplicitRolesForUser(user, domain string) ([]string, error) {
	roles, err := e.enforcer.GetImplicitRolesForUser(user, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit roles for user: %w", err)
	}
	return roles, nil
}

// RemoveUser removes all roles and permissions for a user.
func (e *Enforcer) RemoveUser(user string) error {
	_, err := e.enforcer.DeleteUser(user)
	if err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}
	slog.Debug("User removed", "user", user)
	return nil
}

// RemoveRole removes a role and all its policies.
func (e *Enforcer) RemoveRole(role string) error {
	_, err := e.enforcer.DeleteRole(role)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	slog.Debug("Role removed", "role", role)
	return nil
}

// Close releases resources held by the enforcer.
func (e *Enforcer) Close() error {
	if e.adapter != nil {
		if err := e.adapter.Close(); err != nil {
			return fmt.Errorf("failed to close adapter: %w", err)
		}
	}
	slog.Debug("Enforcer closed")
	return nil
}

// CacheKey generates a cache key for permission decisions.
// Format: perm:{sub}:{dom}:{obj}:{act}
func CacheKey(sub, dom, obj, act string) string {
	return fmt.Sprintf("perm:%s:%s:%s:%s", sub, dom, obj, act)
}

// ParseCacheKey parses a cache key into its components.
// Returns sub, dom, obj, act or an error if the key is invalid.
func ParseCacheKey(key string) (sub, dom, obj, act string, err error) {
	parts := strings.Split(key, ":")
	if len(parts) != 5 || parts[0] != "perm" {
		return "", "", "", "", fmt.Errorf("invalid cache key format: %s", key)
	}
	return parts[1], parts[2], parts[3], parts[4], nil
}

// SetCache sets the permission cache for the enforcer.
func (e *Enforcer) SetCache(cache *Cache) {
	e.cache = cache
}

// EnforceWithCache checks permission with Redis cache.
// It first checks the cache, and on a cache miss, calls Enforce and caches the result.
func (e *Enforcer) EnforceWithCache(ctx context.Context, sub, dom, obj, act string) (bool, error) {
	// If no cache is configured, just call Enforce directly
	if e.cache == nil {
		return e.Enforce(sub, dom, obj, act)
	}

	// Generate cache key
	key := CacheKey(sub, dom, obj, act)

	// Check cache first
	cachedAllowed, err := e.cache.Get(ctx, key)
	if err != nil {
		// Log the error but don't fail - fall back to direct enforcement
		slog.Warn("Failed to get cached permission, falling back to direct enforcement",
			"key", key, "error", err.Error())
	} else {
		// Cache hit - return cached result
		return cachedAllowed, nil
	}

	// Cache miss - call Enforce
	allowed, err := e.Enforce(sub, dom, obj, act)
	if err != nil {
		return false, err
	}

	// Cache the result (ignore errors, as cache failures shouldn't block enforcement)
	if cacheErr := e.cache.Set(ctx, key, allowed); cacheErr != nil {
		slog.Warn("Failed to cache permission result",
			"key", key, "error", cacheErr.Error())
	}

	return allowed, nil
}

// InvalidateCache invalidates all cached permission decisions.
func (e *Enforcer) InvalidateCache(ctx context.Context) error {
	if e.cache == nil {
		return nil
	}
	return e.cache.InvalidateAll(ctx)
}