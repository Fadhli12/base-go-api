# Research: Go API Base

**Date**: 2026-04-22 | **Status**: Phase 0 Research

---

## Executive Summary

This document consolidates research findings for implementing the Go API Base hybrid RBAC/permission system, JWT authentication, PostgreSQL soft deletes, and testing infrastructure. All NEEDS CLARIFICATION items from the implementation plan have been resolved.

---

## 1. Casbin + GORM Integration Patterns

### Decision

Use **Casbin v2 with gorm-adapter v3** for policy storage and enforcement. Initialize with PostgreSQL backend and implement cache layer with Redis for performance and pub/sub for multi-instance synchronization.

### Rationale

- **Runtime customization**: Permissions stored as DB records (not hardcoded enums) allow admin CRUD operations without code changes
- **RBAC-with-domains model**: Supports role hierarchy, explicit deny overriding allow, and wildcard resource matching
- **GORM adapter**: Seamless integration with existing PostgreSQL infrastructure
- **Performance**: In-memory policy enforcement (after LoadPolicy) with O(1) complexity

### Alternatives Considered

| Alternative | Rejected Because |
|--------------|------------------|
| In-memory permission checks | Cannot update permissions at runtime; requires code deploy |
| Direct SQL queries for permission checks | Complex query composition; N+1 problem; no wildcards support |
| Casbin file adapter | Manual policy edits; not suitable for multi-instance APIs |
| Casbin without cache | DB query on every permission check → slow for high-traffic APIs |

### Implementation Pattern

```go
package permission

import (
    "github.com/casbin/casbin/v2"
    "github.com/casbin/gorm-adapter/v3"
    "gorm.io/gorm"
)

type Enforcer struct {
    *casbin.Enforcer
    cache PermissionCache
}

func NewEnforcer(db *gorm.DB, cache PermissionCache) (*Enforcer, error) {
    adapter, err := gormadapter.NewAdapterByDB(db)
    if err != nil {
        return nil, err
    }
    
    enforcer, err := casbin.NewEnforcer("config/rbac_model.conf", adapter)
    if err != nil {
        return nil, err
    }
    
    // Load all policies into memory
    if err := enforcer.LoadPolicy(); err != nil {
        return nil, err
    }
    
    return &Enforcer{
        Enforcer: enforcer,
        cache:    cache,
    }, nil
}

// EnforceWithCache checks permission with Redis cache layer
func (e *Enforcer) EnforceWithCache(userID, resource, action string) (bool, error) {
    cacheKey := fmt.Sprintf("perm:%s:%s:%s", userID, resource, action)
    
    // Check cache first
    if cached, found := e.cache.Get(cacheKey); found {
        return cached.(bool), nil
    }
    
    // Enforce via Casbin
    ok, err := e.Enforcer.Enforce(userID, resource, action)
    if err != nil {
        return false, err
    }
    
    // Cache result (30-60s TTL)
    e.cache.Set(cacheKey, ok, 60*time.Second)
    
    return ok, nil
}

// ReloadAndInvalidate reloads policies from DB and broadcasts to other instances
func (e *Enforcer) ReloadAndInvalidate() error {
    if err := e.Enforcer.LoadPolicy(); err != nil {
        return err
    }
    
    // Invalide all cached permissions (will refresh on next request)
    e.cache.InvalidatePattern("perm:*")
    
    // Broadcast invalidation to other API instances via Redis pub/sub
    e.cache.Publish("permission:invalidate", nil)
    
    return nil
}
```

### Policy Reload Mechanics

**Trigger Points**:
1. Role created, updated, deleted
2. Permission attached to or removed from role
3. User assigned to or removed from role
4. User permission override added or removed

**Implementation**:
```go
// In role service
func (s *RoleService) AttachPermission(roleID, permissionID string) error {
    // 1. Insert into role_permissions table
    if err := s.repo.InsertRolePermission(roleID, permissionID); err != nil {
        return err
    }
    
    // 2. Reload Casbin policies from DB
    if err := s.enforcer.LoadPolicy(); err != nil {
        return err
    }
    
    // 3. Invalidate permission cache (Redis)
    if err := s.cache.InvalidateAll(); err != nil {
        return err
    }
    
    // 4. Broadcast invalidation to other instances (Redis pub/sub)
    s.pubsub.Publish("permission:invalidate", map[string]interface{}{
        "role_id": roleID,
        "permission_id": permissionID,
    })
    
    return nil
}
```

## Common Pitfalls

| Pitfall | Mitigation |
|---------|-----------|
| **LoadPolicy not called on startup** | Initialize enforcer with LoadPolicy() in main.go before serving requests |
| **Cache not invalidated after policy change** | Always call LoadPolicy() + cache.InvalidateAll() after permission/role CRUD |
| **N+1 permission check per request** | Cache result in Redis; preload user's roles + permissions on login |
| **Casbin policy drift across instances** | Use Redis pub/sub to broadcast invalidation; each instance subscribes and reloads |
| **keyMatch2 not working** | Ensure resource patterns in DB match keyMatch2 syntax (e.g., `invoice/:id`, `*`) |
| **Performance degradation with >100k policies** | Consider policy sharding by tenant; use Casbin Watcher for distributed invalidation |

---

## 2. JWT Refresh Token Rotation Patterns

### Decision

Use **stateless JWT access tokens (5–15 min expiry) + refresh tokens stored in PostgreSQL with revocation capability**. Rotate refresh tokens on each use.

### Rationale

- **Stateless access tokens**: Fast permission checks without DB lookup
- **Short-lived**: 5–15 minute expiry limits exposure if compromised
- **Refresh tokens in DB**: Immediate revocation possible (mark as revoked)
- **Rotation**: New refresh token issued on every use; old token invalidates

### Alternatives Considered

| Alternative | Rejected Because |
|--------------|------------------|
| Pure stateless JWT (no refresh) | Cannot revoke tokens; compromised token valid until expiry |
| Refresh tokens in Redis only | Data loss if Redis crashes; DB required for durability |
| Long-lived access tokens (30+ min) | Security risk; longer exposure window |
| JWT blacklist (Redis) | Requires blacklist to store all revoked tokens; memory overhead |

### Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    JWT Auth Flow                             │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. LOGIN                                                    │
│  Client ──► POST /auth/login                                │
│          {email, password}                                   │
│                                                               │
│          ◄─── access_token (15 min)                         │
│          ◄─── refresh_token (30 days, stored in DB)          │
│                                                               │
│  2. ACCESS PROTECTED RESOURCE                                 │
│  Client ──► GET /invoices                                    │
│          Authorization: Bearer <access_token>                │
│                                                               │
│          ◄─── Response (no DB lookup)                        │
│                                                               │
│  3. ACCESS TOKEN EXPIRED (15 min)                            │
│  Client ──► POST /auth/refresh                               │
│          {refresh_token}                                      │
│                                                               │
│          ┌─ Validate refresh_token hash in DB                │
│          ├─ Check expires_at > NOW()                         │
│          ├─ Check revoked_at IS NULL                          │
│          ├─ Mark old refresh_token as rotated (optional)     │
│          └─ Issue NEW access_token + NEW refresh_token       │
│                                                               │
│          ◄─── new_access_token (15 min)                      │
│          ◄─── new_refresh_token (30 days)                     │
│                                                               │
│  4. LOGOUT                                                    │
│  Client ──► POST /auth/logout                                │
│          Authorization: Bearer <access_token>                 │
│                                                               │
│          ─ Mark ALL user's refresh_tokens as revoked         │
│          ─ Client discards access_token                        │
│                                                               │
│  5. EARLY REVOCATION (optional)                               │
│  If need instant revocation (< 15 min), use Redis blacklist: │
│  - Key: jwt_blacklist:{jti}                                  │
│  - Value: timestamp                                           │
│  - TTL: remaining access_token expiry                         │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

### Implementation Pattern

```go
package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

type TokenService struct {
    db           *gorm.DB
    jwtSecret    []byte
    accessExpiry time.Duration  // 15 minutes
    refreshExpiry time.Duration // 30 days
}

type AccessTokenClaims struct {
    jwt.RegisteredClaims
    UserID string `json:"user_id"`
    Email  string `json:"email"`
}

type RefreshToken struct {
    ID        uuid.UUID `gorm:"type:uuid;primary_key"`
    UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
    TokenHash string    `gorm:"not null;index"`
    ExpiresAt time.Time `gorm:"not null;index"`
    RevokedAt *time.Time
    CreatedAt time.Time
}

// GenerateAccessToken creates short-lived JWT (5–15 min)
func (s *TokenService) GenerateAccessToken(userID, email string) (string, error) {
    claims := AccessTokenClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            ID:        uuid.New().String(),
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "go-api",
        },
        UserID: userID,
        Email:  email,
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}

// GenerateRefreshToken creates long-lived token (30 days), stores hash in DB
func (s *TokenService) GenerateRefreshToken(userID string) (string, error) {
    token := uuid.New().String() // Random UUID as refresh token
    tokenHash := sha256Hash(token)
    
    refreshToken := RefreshToken{
        ID:        uuid.New(),
        UserID:    uuid.MustParse(userID),
        TokenHash: tokenHash,
        ExpiresAt: time.Now().Add(s.refreshExpiry),
        CreatedAt: time.Now(),
    }
    
    if err := s.db.Create(&refreshToken).Error; err != nil {
        return "", err
    }
    
    return token, nil
}

// ValidateRefreshToken checks DB for validity and revocation
func (s *TokenService) ValidateRefreshToken(token string) (*RefreshToken, error) {
    tokenHash := sha256Hash(token)
    
    var refreshToken RefreshToken
    if err := s.db.Where("token_hash = ?", tokenHash).First(&refreshToken).Error; err != nil {
        return nil, ErrInvalidToken
    }
    
    if refreshToken.RevokedAt != nil {
        return nil, ErrTokenRevoked
    }
    
    if refreshToken.ExpiresAt.Before(time.Now()) {
        return nil, ErrTokenExpired
    }
    
    return &refreshToken, nil
}

// RotateRefreshToken issues new tokens and invalidates old one
func (s *TokenService) RotateRefreshToken(oldToken string) (string, string, error) {
    // Validate old refresh token
    rt, err := s.ValidateRefreshToken(oldToken)
    if err != nil {
        return "", "", err
    }
    
    // Mark old token as rotated (optional: soft delete)
    now := time.Now()
    s.db.Model(&rt).Update("revoked_at", &now)
    
    // Generate new access + refresh tokens
    accessToken, err := s.GenerateAccessToken(rt.UserID.String(), "")
    if err != nil {
        return "", "", err
    }
    
    newRefreshToken, err := s.GenerateRefreshToken(rt.UserID.String())
    if err != nil {
        return "", "", err
    }
    
    return accessToken, newRefreshToken, nil
}

// RevokeAllUserTokens marks all user's refresh tokens as revoked (logout)
func (s *TokenService) RevokeAllUserTokens(userID string) error {
    now := time.Now()
    return s.db.Model(&RefreshToken{}).
        Where("user_id = ? AND revoked_at IS NULL", userID).
        Update("revoked_at", &now).Error
}

func sha256Hash(token string) string {
    h := sha256.New()
    h.Write([]byte(token))
    return hex.EncodeToString(h.Sum(nil))
}
```

### Security Best Practices

| Practice | Why |
|---------|-----|
| **Store token hash, not plaintext** | If DB compromised, attacker cannot use token hashes |
| **Access token: 5–15 min expiry** | Limits exposure if compromised |
| **Refresh token: 30 days max** | Balances convenience and security |
| **Rotate refresh token on use** | Prevents replay attacks; stolen token becomes invalid after use |
| **Revoke on logout** | Immediate revocation via DB flag; client discards access token |
| **Use HTTPS only** | Prevents token interception in transit |
| **Short JWT ID (jti)** | For optional blacklist; short-lived (within access token expiry) |

---

## 3. Redis Pub/Sub for Permission Cache Invalidation

### Decision

Use **Redis pub/sub with topic-based channels** for multi-instance cache invalidation. Each API instance subscribes to `permission:invalidate` and clears local permission cache when message received.

### Rationale

- **Sub-second consistency**: Changes propagate to all instances before response sent
- **Minimal latency**: Redis pub/sub is O(1) message delivery
- **Scalable**: Works with any number of API instances
- **Simple**: No distributed coordination required

### Alternatives Considered

| Alternative | Rejected Because |
|--------------|------------------|
| Polling DB for changes (30s interval) | Up to 30s inconsistency; wasteful queries |
| Centralized cache (Redis only, no local) | Network latency on every permission check; Redis becomes bottleneck |
| Database triggers + NOTIFY | PostgreSQL-specific; more complex; less flexible than Redis pub/sub |

### Channel Design

| Channel | Event | Payload |
|--------|-------|---------|
| `permission:invalidate` | Any permission/role change | `{user_id?: string, role_id?: string, permission_id?: string, type: "invalidate"}` |
| `permission:reload` | Casbin policy reload needed | `{type: "reload"}` |

### Implementation Pattern

```go
package permission

import (
    "context"
    "encoding/json"
    "log/slog"
    
    "github.com/redis/go-redis/v9"
)

type CacheInvalidator struct {
    redisClient *redis.Client
    localCache  PermissionCache // In-memory or Redis cache
}

type InvalidationMessage struct {
    Type         string `json:"type"`          // "invalidate" or "reload"
    UserID       string `json:"user_id,omitempty"`
    RoleID       string `json:"role_id,omitempty"`
    PermissionID string `json:"permission_id,omitempty"`
}

// PublishInvalidate broadcasts cache invalidation to all API instances
func (ci *CacheInvalidator) PublishInvalidate(ctx context.Context, message InvalidationMessage) error {
    payload, err := json.Marshal(message)
    if err != nil {
        return err
    }
    
    return ci.redisClient.Publish(ctx, "permission:invalidate", payload).Err()
}

// SubscribeInvalidation listens for invalidation messages and clears cache
func (ci *CacheInvalidator) SubscribeInvalidation(ctx context.Context) error {
    pubsub := ci.redisClient.Subscribe(ctx, "permission:invalidate")
    defer pubsub.Close()
    
    ch := pubsub.Channel()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case msg, ok := <-ch:
            if !ok {
                return nil
            }
            
            var invalidation InvalidationMessage
            if err := json.Unmarshal([]byte(msg.Payload), &invalidation); err != nil {
                slog.Error("failed to unmarshal invalidation message", "error", err)
                continue
            }
            
            // Handle invalidation
            ci.handleInvalidation(ctx, invalidation)
        }
    }
}

// handleInvalidation processes invalidation message
func (ci *CacheInvalidator) handleInvalidation(ctx context.Context, msg InvalidationMessage) {
    switch msg.Type {
    case "invalidate":
        // Invalidate specific user's permissions
        if msg.UserID != "" {
            ci.localCache.InvalidatePattern(fmt.Sprintf("perm:%s:*", msg.UserID))
        }
        
        // Invalidate all users with specific role
        if msg.RoleID != "" {
            // Query DB for users with this role, invalidate each
            // Or invalidate entire cache (conservative approach)
            ci.localCache.InvalidateAll()
        }
        
        // Invalidate all users with specific permission
        if msg.PermissionID != "" {
            ci.localCache.InvalidateAll()
        }
        
    case "reload":
        // Trigger Casbin policy reload
        ci.localCache.InvalidateAll()
    }
}

// StartInvalidationListener runs in background on API startup
func (ci *CacheInvalidator) StartInvalidationListener(ctx context.Context) {
    go func() {
        if err := ci.SubscribeInvalidation(ctx); err != nil {
            slog.Error("invalidation listener failed", "error", err)
        }
    }()
}
```

### Usage in Service Layer

```go
// In role service after attaching permission
func (s *RoleService) AttachPermission(roleID, permissionID string) error {
    // 1. DB update
    if err := s.repo.InsertRolePermission(roleID, permissionID); err != nil {
        return err
    }
    
    // 2. Reload Casbin policies
    if err := s.enforcer.LoadPolicy(); err != nil {
        return err
    }
    
    // 3. Publish invalidation to all instances
    s.invalidator.PublishInvalidate(context.Background(), InvalidationMessage{
        Type:         "invalidate",
        RoleID:       roleID,
        PermissionID: permissionID,
    })
    
    return nil
}
```

### Handling Missed Messages

**Problem**: If instance is down when invalidation published, cache may be stale.

**Mitigations**:
1. **TTL-based fallback**: Permission cache expires in 30–60 seconds; eventual consistency guaranteed
2. **On-demand reload**: If permission check fails, trigger full reload
3. **Health check invalidation**: Periodically (5 min) publish `permission:reload` to sync all instances

### Consistency Guarantees

| Scenario | Behavior |
|---------|----------|
| User granted permission | Invalidation published → all instances refresh within <1s |
| Instance down during grant | On recovery, TTL expires → cache refreshes on next request |
| Network partition | TTL fallback → eventual consistency within 60s |
| High-frequency updates | Pub/sub handles burst; cache invalidates for each change |

---

## 4. PostgreSQL Soft Deletes and Audit Trail Patterns

### Decision

Use **GORM soft delete (`deleted_at` timestamp)** for all user-generated data. Implement **audit logging with before/after JSONB snapshots** for compliance.

### Rationale

- **Audit trail**: Soft deletes preserve historical data
- **Undo capability**: Soft-deleted rows can be restored
- **Compliance**: Regulations often require data retention
- **GORM built-in**: Automatic query scoping with `db.Unscoped()` for raw queries

### Alternatives Considered

| Alternative | Rejected Because |
|--------------|------------------|
| Hard deletes (DELETE FROM) | Audit trail lost; cannot undo |
| Archive tables (move deleted rows to separate table) | Complex; requires copying data; schema drift |
| Audit log only (no soft deletes) | Cannot restore rows; foreign key violations |

### GORM Model Pattern

```go
package domain

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// BaseModel includes common fields for all entities with soft delete
type BaseModel struct {
    ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// User entity with soft delete
type User struct {
    BaseModel
    Email        string `gorm:"uniqueIndex;not null" json:"email"`
    PasswordHash string `gorm:"not null" json:"-"`
}

// Invoice entity with soft delete
type Invoice struct {
    BaseModel
    UserID      uuid.UUID       `gorm:"type:uuid;not null;index" json:"user_id"`
    Amount      float64         `gorm:"not null" json:"amount"`
    Status      string          `gorm:"not null;default:'draft'" json:"status"`
    User        User            `gorm:"foreignKey:UserID" json:"user"`
}

// GORM automatically excludes soft-deleted rows from queries
// db.Find(&users) → only active users (deleted_at IS NULL)
// db.Unscoped().Find(&users) → all users including deleted
```

### Query Scoping

```go
// Get only active users
func (r *UserRepository) FindAll(ctx context.Context) ([]User, error) {
    var users []User
    if err := r.db.WithContext(ctx).Find(&users).Error; err != nil {
        return nil, err
    }
    return users, nil
}

// Get all users including deleted
func (r *UserRepository) FindAllIncludingDeleted(ctx context.Context) ([]User, error) {
    var users []User
    if err := r.db.WithContext(ctx).Unscoped().Find(&users).Error; err != nil {
        return nil, err
    }
    return users, nil
}

// Soft delete a user
func (r *UserRepository) SoftDelete(ctx context.Context, id string) error {
    return r.db.WithContext(ctx).Delete(&User{}, "id = ?", id).Error
}

// Restore a soft-deleted user
func (r *UserRepository) Restore(ctx context.Context, id string) error {
    return r.db.WithContext(ctx).Unscoped().Model(&User{}).Where("id = ?", id).Update("deleted_at", nil).Error
}

// Permanently delete (hard delete)
func (r *UserRepository) HardDelete(ctx context.Context, id string) error {
    return r.db.WithContext(ctx).Unscoped().Delete(&User{}, "id = ?", id).Error
}
```

### Audit Log Table

```go
package domain

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

type AuditLog struct {
    ID          uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
    ActorID     uuid.UUID       `gorm:"type:uuid;not null;index" json:"actor_id"`
    Action      string          `gorm:"not null;index" json:"action"`           // create, update, delete, login, etc.
    Resource    string          `gorm:"not null;index" json:"resource"`         // invoice, user, role, permission
    ResourceID  string          `gorm:"index" json:"resource_id"`               // UUID or natural key
    Before      json.RawMessage `gorm:"type:jsonb" json:"before"`               // State before change
    After       json.RawMessage `gorm:"type:jsonb" json:"after"`               // State after change
    IPAddress   string          `gorm:"type:inet" json:"ip_address"`
    UserAgent   string          `gorm:"type:text" json:"user_agent"`
    CreatedAt   time.Time       `gorm:"not null;index" json:"created_at"`
    
    Actor       User            `gorm:"foreignKey:ActorID" json:"actor"`
}

// Before and After are JSONB snapshots:
// {
//   "id": "550e8400-e29b-41d4-a716-446655440000",
//   "amount": 15000,
//   "status": "draft",
//   "user_id": "..."
// }
```

### Audit Middleware

```go
package middleware

import (
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "gorm.io/gorm"
)

func AuditLogger(db *gorm.DB) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Skip audit for health checks
            if c.Path() == "/healthz" || c.Path() == "/readyz" {
                return next(c)
            }
            
            // Capture before state for updates/deletes
            var before interface{}
            if c.Request().Method == "PUT" || c.Request().Method == "DELETE" {
                // Query DB for current state
                resourceID := c.Param("id")
                before = fetchBeforeState(db, c.Path(), resourceID)
            }
            
            // Execute handler
            err := next(c)
            if err != nil {
                return err
            }
            
            // Capture after state
            var after interface{}
            if c.Request().Method == "POST" || c.Request().Method == "PUT" {
                after = extractAfterState(c)
            }
            
            // Create audit log
            userID := c.Get("user_id").(string)
            
            auditLog := AuditLog{
                ID:         uuid.New(),
                ActorID:    uuid.MustParse(userID),
                Action:     methodToAction(c.Request().Method),
                Resource:   pathToResource(c.Path()),
                ResourceID: c.Param("id"),
                Before:     marshalJSON(before),
                After:      marshalJSON(after),
                IPAddress:  c.RealIP(),
                UserAgent:  c.Request().UserAgent(),
                CreatedAt:  time.Now(),
            }
            
            // Async write to avoid blocking request
            go func() {
                db.Create(&auditLog)
            }()
            
            return nil
        }
    }
}

func methodToAction(method string) string {
    switch method {
    case "POST":
        return "create"
    case "PUT":
        return "update"
    case "DELETE":
        return "delete"
    default:
        return method
    }
}

func marshalJSON(v interface{}) json.RawMessage {
    if v == nil {
        return nil
    }
    data, _ := json.Marshal(v)
    return data
}
```

### Compliance Requirements

| Requirement | Implementation |
|-------------|----------------|
| Immutability | No UPDATE on audit_logs; only INSERT |
| Actor tracking | Every audit_log has actor_id FK |
| Before/after snapshots | JSONB columns capture state delta |
| IP/User-Agent | Stored for forensics |
| Retention | Configure job to rotate logs older than X days |
| Soft delete enforcement | All user-generated tables have deleted_at |

---

## 5. Testcontainers-go Integration Test Setup

### Decision

Use **testcontainers-go** for real PostgreSQL container in integration tests. Each test suite spins up a fresh container, runs migrations, and tears down after tests.

### Rationale

- **Real DB behavior**: No SQLite mock; tests actual PostgreSQL features (JSONB, UUID, unique constraints)
- **Isolation**: Each test gets clean database
- **CI-friendly**: Works in Docker-based CI (GitHub Actions)
- **Fast enough**: Container reuse possible; startup ~3-5 seconds

### Alternatives Considered

| Alternative | Rejected Because |
|--------------|------------------|
| SQLite in-memory | Does not support JSONB, UUID, PostgreSQL-specific features |
| Mock DB (sqlmock) | No integration testing; only unit testing |
| Shared Postgres instance | State leaks between tests; isolation issues |
| Docker Compose in CI | Slower; requires external orchestration |

### Implementation Pattern

```go
package testsuite

import (
    "context"
    "testing"
    "time"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    "gorm.io/gorm"
)

type TestDatabase struct {
    Container testcontainers.Container
    DB        *gorm.DB
    ConnStr   string
}

// SetupTestDatabase spins up a PostgreSQL container for testing
func SetupTestDatabase(t *testing.T) *TestDatabase {
    ctx := context.Background()
    
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_DB":       "testdb",
            "POSTGRES_USER":     "testuser",
            "POSTGRES_PASSWORD": "testpass",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections").
            WithOccurrence(2).
            WithStartupTimeout(time.Minute),
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        t.Fatalf("failed to start container: %v", err)
    }
    
    host, err := container.Host(ctx)
    if err != nil {
        t.Fatalf("failed to get host: %v", err)
    }
    
    port, err := container.MappedPort(ctx, "5432")
    if err != nil {
        t.Fatalf("failed to get port: %v", err)
    }
    
    connStr := fmt.Sprintf(
        "host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable",
        host, port.Port(),
    )
    
    db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to connect to database: %v", err)
    }
    
    return &TestDatabase{
        Container: container,
        DB:        db,
        ConnStr:   connStr,
    }
}

// Teardown stops container
func (td *TestDatabase) Teardown() {
    ctx := context.Background()
    if td.Container != nil {
        td.Container.Terminate(ctx)
    }
}

// RunMigrations applies all SQL migrations
func (td *TestDatabase) RunMigrations(t *testing.T) {
    m, err := migrate.New(
        "file://../../migrations",
        td.ConnStr,
    )
    if err != nil {
        t.Fatalf("failed to create migrate instance: %v", err)
    }
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        t.Fatalf("failed to run migrations: %v", err)
    }
}
```

### Integration Test Example

```go
package integration_test

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    "myapp/internal/domain"
    "myapp/internal/repository"
)

func TestUserRepository_Create(t *testing.T) {
    // Setup
    testDB := SetupTestDatabase(t)
    defer testDB.Teardown()
    
    testDB.RunMigrations(t)
    
    repo := repository.NewUserRepository(testDB.DB)
    
    // Test
    user := &domain.User{
        Email:        "test@example.com",
        PasswordHash: "hashedpassword",
    }
    
    err := repo.Create(context.Background(), user)
    
    // Assert
    assert.NoError(t, err)
    assert.NotEqual(t, uuid.Nil, user.ID)
    
    // Verify soft delete works
    err = repo.SoftDelete(context.Background(), user.ID.String())
    assert.NoError(t, err)
    
    // Query should not find soft-deleted user
    var found domain.User
    err = testDB.DB.First(&found, "id = ?", user.ID).Error
    assert.Error(t, err) // Should not find
    assert.Equal(t, gorm.ErrRecordNotFound, err)
    
    // Unscoped query should find it
    err = testDB.DB.Unscoped().First(&found, "id = ?", user.ID).Error
    assert.NoError(t, err)
    assert.NotNil(t, found.DeletedAt)
}

func TestAuditLog_CaptureState(t *testing.T) {
    testDB := SetupTestDatabase(t)
    defer testDB.Teardown()
    
    testDB.RunMigrations(t)
    
    // Create user
    user := createTestUser(t, testDB.DB)
    
    // Create audit log
    before := map[string]interface{}{"amount": 10000}
    after := map[string]interface{}{"amount": 15000, "status": "paid"}
    
    audit := &domain.AuditLog{
        ID:         uuid.New(),
        ActorID:    user.ID,
        Action:     "update",
        Resource:   "invoice",
        ResourceID: uuid.New().String(),
        Before:     marshalJSON(before),
        After:      marshalJSON(after),
        CreatedAt:  time.Now(),
    }
    
    err := testDB.DB.Create(audit).Error
    assert.NoError(t, err)
    
    // Query
    var found domain.AuditLog
    err = testDB.DB.First(&found, "id = ?", audit.ID).Error
    assert.NoError(t, err)
    
    // Verify JSONB
    var afterData map[string]interface{}
    json.Unmarshal(found.After, &afterData)
    assert.Equal(t, 15000.0, afterData["amount"])
}
```

### Performance Considerations

| Metric | Value |
|--------|-------|
| Container startup | 3-5 seconds |
| Migration run | 0.5-1 second |
| Test execution | Depends on complexity |
| Teardown | <1 second |

**Optimizations**:
- **Reuse container**: Use `testcontainers.ContainerRequest` with `Reuse: true` for multiple tests
- **Parallel tests**: Run independent tests in parallel (go test -parallel)
- **Cache layers**: Docker caches PostgreSQL image locally

### CI/CD Integration (GitHub Actions)

```yaml
name: Tests

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_DB: testdb
          POSTGRES_USER: testuser
          POSTGRES_PASSWORD: testpass
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      redis:
        image: redis:7
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      
      - name: Install dependencies
        run: go mod download
      
      - name: Run unit tests
        run: go test -v ./tests/unit/...
      
      - name: Run integration tests
        run: go test -v -parallel 4 ./tests/integration/...
        env:
          DATABASE_URL: postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable
          REDIS_URL: redis://localhost:6379/0
```

---

## Summary Table

| Area | Decision | Key Pattern |
|------|----------|-------------|
| Authentication | JWT access (15 min) + refresh token in DB (30 days) | Short-lived access, revocable refresh |
| Permission Engine | Casbin RBAC-with-domains + Redis cache + pub/sub invalidation | O(1) enforcement, sub-second consistency |
| Database Soft Deletes | GORM deleted_at timestamp + Unscoped() for raw queries | Audit trail preserved |
| Audit Logging | Before/after JSONB snapshots + async write | Immutable audit_logs table |
| Testing | testcontainers-go for real PostgreSQL integration tests | Isolation, CI-friendly |

---

**Version**: 1.0 | **Created**: 2026-04-22 | **Status**: Phase 0 Complete