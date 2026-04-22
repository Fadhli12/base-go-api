# Permission Module - Casbin RBAC

**Layer:** internal/permission | **Purpose:** RBAC enforcement with Redis caching

---

## OVERVIEW

Casbin-based permission system with RBAC-with-domains model. Uses GORM adapter for policy persistence and Redis for permission decision caching with pub/sub invalidation across instances.

## FILES

| File | Purpose |
|------|---------|
| `enforcer.go` | Casbin wrapper, policy management |
| `cache.go` | Redis permission cache layer |
| `invalidator.go` | Pub/sub cache invalidation |

## ENFORCER

### Initialization

```go
enforcer, err := permission.NewEnforcer(db)
// Creates GORM adapter, loads policies from casbin_rule table
```

### RBAC Model (embedded)

```
r = sub, dom, obj, act    # Request: user, domain, resource, action
p = sub, dom, obj, act    # Policy: role, domain, resource, action
g = _, _, _                 # Grouping: user, role, domain

# Matcher: role matches + domain matches + resource matches + action matches
g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

### Key Methods

```go
// Direct check (bypasses cache)
Enforce(sub, dom, obj, act string) (bool, error)

// Cache-first check (preferred)
EnforceWithCache(ctx, sub, dom, obj, act) (bool, error)

// Policy management
AddPolicy(sub, dom, obj, act) error
RemovePolicy(sub, dom, obj, act) error
LoadPolicy() error    // Reload from DB
SavePolicy() error    // Persist to DB

// Role management
AddRoleForUser(userID, role, domain) error
RemoveRoleForUser(userID, role, domain) error
GetRolesForUser(userID, domain) ([]string, error)

// Permission queries
GetImplicitPermissionsForUser(userID, domain) ([][]string, error)
```

## CACHE

### Key Format

```
perm:{sub}:{dom}:{obj}:{act}
```

Values: `1` = allowed, `0` = denied

### Invalidation

```go
// Clear all (SCAN-based, non-blocking)
cache.InvalidateAll(ctx) error

// Clear user-specific
cache.InvalidateForUser(ctx, userID) error

// Clear domain-specific  
cache.InvalidateForDomain(ctx, domain) error
```

### TTL

Default: 5 minutes. Set during initialization.

## INVALIDATOR (Pub/Sub)

### Channel

```
permission:invalidate
```

### Usage

```go
invalidator := permission.NewInvalidator(redisClient)

// Start background listener (run in goroutine)
go invalidator.StartInvalidationListener(ctx, cache, enforcer)

// Broadcast invalidation to all instances
invalidator.BroadcastInvalidation(ctx, cache, enforcer) error

// Local-only invalidation + reload
invalidator.InvalidateAndReload(ctx, cache, enforcer) error
```

## MIDDLEWARE INTEGRATION

```go
// internal/http/middleware/permission.go

// Pre-configured middleware
middleware.RequirePermission(enforcer, "invoices", "view")
middleware.RequireRole(enforcer, "admin")

// Custom config
middleware.Permission(middleware.PermissionConfig{
    Enforcer: enforcer,
    Resource: "invoices",
    Action:   "view",
    domainExtractor: middleware.TenantDomainExtractor,
})
```

## POLICY STORAGE

**Database:** `casbin_rule` table (GORM adapter auto-creates)

| Column | Purpose |
|--------|---------|
| `ptype` | `p` (policy) or `g` (grouping) |
| `v0` | subject (role/user) |
| `v1` | domain |
| `v2` | object (resource) |
| `v3` | action |

**Example rows:**
```
p, admin, default, users, manage
p, viewer, default, invoices, view
g, user-uuid, admin, default
```

## PATTERNS

### Service Layer Check

```go
// Basic permission
allowed, err := enforcer.EnforceWithCache(ctx, userID, "default", "invoices", "view")

// With ownership scope (not in Casbin, handled separately)
if !isAdmin && invoice.UserID != userID {
    return ErrForbidden
}
```

### Permission Naming

```
{resource}:{action}       # e.g., invoices:view
{resource}:{action}:{scope}  # e.g., invoices:view:all vs invoices:view:own
```

## SYNC COMMANDS

```bash
# Seed roles + permissions + sync to Casbin
make seed

# Sync from manifest only
go run ./cmd/api permission:sync

# After sync, invalidate cache
# (currently manual - call BroadcastInvalidation)
```

## TESTING

```go
// tests/integration/permission_enforcement_test.go
suite := NewTestSuite(t)
defer suite.Cleanup()
suite.RunMigrations(t)

// Add policy
enforcer.AddPolicy("admin", "default", "invoices", "manage")

// Test enforcement
allowed, _ := enforcer.Enforce(userID, "default", "invoices", "manage")
assert.True(t, allowed)
```

## NOTES

- **Performance**: Cache-first for hot path. Direct checks for sync ops.
- **Consistency**: Pub/sub invalidation before response sent.
- **Fallback**: If invalidation fails, cache TTL (5m) ensures eventual consistency.
- **Domain**: Default domain is `"default"`. Multi-tenant uses X-Tenant-ID header.