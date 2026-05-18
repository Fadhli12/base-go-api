# Go API Base — Feature Recommendations V2

**Generated:** 2026-05-13  
**Purpose:** Catalog of new features recommended for implementation beyond the existing 17 features  
**Base:** All features from `FEATURE_RECOMMENDATIONS.md` (V1) are now ✅ COMPLETE

---

## Prerequisite Summary

All 17 features from V1 are fully implemented and verified:

| Feature | Status | Migration |
|---------|--------|-----------|
| Organization/Team | ✅ Complete | 000006, 000007 |
| API Key Auth | ✅ Complete | 000005 |
| Email Service (SMTP + SendGrid + SES) | ✅ Complete | 000008 |
| Notification System | ✅ Complete | 000012 |
| Webhook System | ✅ Complete | 000014 |
| 2FA | ✅ Complete | 000015 |
| Search & Filtering | ✅ Complete | 000016, 000017 |
| File Versioning | ✅ Complete | 000019 |
| Comment System | ✅ Complete | 000022 |
| Tagging System | ✅ Complete | 000024 |
| Activity Feed / Timeline | ✅ Complete | 000023 |
| Background Job Queue | ✅ Complete | Redis-based |
| Data Import/Export | ✅ Complete | 000020 |
| Settings & Configuration | ✅ Complete | 000013 |
| Feature Flags | ✅ Complete | 000021 |
| WebSocket Real-time | ✅ Complete | 000026 |
| Analytics Dashboard | ✅ Complete | 000025 |

**Additional migrations not in V1:** 000009 (password_reset_tokens), 000010 (refresh_token_family), 000011 (session_metadata), 000018 (refresh_token_is2fa_pending)

---

## Architecture Patterns (Established)

All V2 features MUST follow these established patterns:

### Entity Pattern
```go
type Entity struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (Entity) TableName() string { return "entities" }
func (e *Entity) ToResponse() EntityResponse { ... }
```

### Repository Pattern
```go
type EntityRepository interface {
    Create(ctx context.Context, entity *Entity) error
    FindByID(ctx context.Context, id uuid.UUID) (*Entity, error)
    FindAll(ctx context.Context, filter Filter, limit, offset int) ([]Entity, int64, error)
    Update(ctx context.Context, entity *Entity) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

### Service Pattern
```go
type EntityService struct {
    repo     EntityRepository
    enforcer *permission.Enforcer
    audit    *AuditService
    eventBus *domain.EventBus // optional: SetEventBus() setter
}
```

### Handler Pattern
```go
// Handler-level RBAC enforcement (NOT service-level Enforce calls)
func (h *Handler) Create(c echo.Context) error {
    // 1. Get user context
    // 2. Parse + validate request
    // 3. Check permission via enforcer
    // 4. Call service
    // 5. Map errors
    // 6. Return response
}
```

### Config Pattern
```go
type XConfig struct {
    Enabled         bool          `mapstructure:"x_enabled"`
    SomeTimeout     time.Duration `mapstructure:"x_some_timeout"`
    SomeLimit       int           `mapstructure:"x_some_limit"`
}
```

### Migration Pattern
- Sequential numbering: `NNNNNN_descriptive_name.up.sql` + `.down.sql`
- All tables use `uuid` primary keys with `default:gen_random_uuid()`
- All tables have `created_at`, `updated_at`, `deleted_at` columns
- `deleted_at` with partial unique indexes: `WHERE deleted_at IS NULL`

### Permissions Pattern
```go
// In DefaultPermissions():
{Name: "resource:action", Description: "Description", Resource: "resource", Action: "action"},
```

---

## V2 Feature Recommendations

### Tier 1: Security & Compliance (Must-Have for Production)

---

#### 1.1 Idempotency Keys

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Redis (existing)

**Description:**
Prevents duplicate operations from retry mechanisms, network failures, and double-clicks. Every payment API (Stripe, PayPal) requires this. RFC 9110 standard.

**Entities:**
```go
type IdempotencyRecord struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Key         string         `gorm:"size:128;uniqueIndex;not null" json:"key"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    Method      string         `gorm:"size:10;not null" json:"method"`    // POST, PUT, PATCH
    Path        string         `gorm:"size:500;not null" json:"path"`
    RequestHash string         `gorm:"size:64;not null" json:"request_hash"` // SHA-256 of body
    StatusCode  int            `gorm:"not null" json:"status_code"`
    Response    datatypes.JSON `gorm:"type:jsonb" json:"response"`
    ExpiresAt   time.Time      `gorm:"index;not null" json:"expires_at"`
    CreatedAt   time.Time      `json:"created_at"`
}
func (IdempotencyRecord) TableName() string { return "idempotency_records" }
```

**Middleware Pattern:**
```go
func IdempotencyMiddleware(store IdempotencyStore) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := c.Request().Header.Get("Idempotency-Key")
            if key == "" {
                return next(c) // Pass through if no key
            }
            
            // Check Redis for existing response
            cached, err := store.Get(c.Request().Context(), key)
            if err == nil {
                // Return cached response
                return c.JSON(cached.StatusCode, cached.Body)
            }
            
            // Process request and cache response
            return next(c)
        }
    }
}
```

**Configuration:**
```go
type IdempotencyConfig struct {
    Enabled       bool          `mapstructure:"idempotency_enabled"`
    DefaultTTL    time.Duration `mapstructure:"idempotency_default_ttl"`    // 24h
    MaxTTL        time.Duration `mapstructure:"idempotency_max_ttl"`        // 72h
    MaxKeyLength  int           `mapstructure:"idempotency_max_key_length"` // 128
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| (None — middleware, no RBAC needed) | | | |

**Implementation Notes:**
- Redis-backed key store with configurable TTL (default 24h, max 72h)
- `Idempotency-Key` header (RFC 9110)
- SHA-256 request body hash guards against key reuse with different payloads
- Return `409 Conflict` if key exists with different request hash
- Return cached response if key matches
- Automatic cleanup via background reaper (configurable interval)
- Follows existing middleware pattern (`rate_limit.go`, `jwt.go`)
- Separate Redis keys from cache/permission namespace: `idempotency:{key}`

**Endpoints:**
None (pure middleware). Applies to all POST, PUT, PATCH endpoints automatically.

---

#### 1.2 SSRF Protection

**Complexity:** Low  
**Effort:** 1-2 days  
**Dependencies:** None

**Description:**
Webhook worker makes outbound HTTP requests. Without URL validation, attackers can force the server to access internal services (Redis, database, cloud metadata endpoints). OWASP API Security Top 10 #6.

**Entities:**
None (middleware + validation layer only)

**Service Pattern:**
```go
type SSRFValidator struct {
    allowedCIDRs    []*net.IPNet
    blockedCIDRs    []*net.IPNet
    blockedHosts    []string
    allowPrivateIPs bool
    dnsRebindTTL    time.Duration
}

func (v *SSRFValidator) ValidateURL(rawURL string) error {
    // 1. Parse URL
    // 2. Resolve hostname (check DNS rebinding)
    // 3. Check against blocked CIDRs (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, 127.0.0.0/8)
    // 4. Check against blocked hosts (metadata endpoints)
    // 5. Return error if blocked
}
```

**Configuration:**
```go
type SSRFConfig struct {
    Enabled            bool     `mapstructure:"ssrf_enabled"`
    BlockedCIDRs       []string `mapstructure:"ssrf_blocked_cidrs"`        // default: private ranges
    BlockedHosts       []string `mapstructure:"ssrf_blocked_hosts"`         // e.g., metadata.internal
    AllowPrivateIPs    bool     `mapstructure:"ssrf_allow_private_ips"`     // default: false
    AllowedSchemes     []string `mapstructure:"ssrf_allowed_schemes"`       // default: ["https"]
    DNSRebindProtection bool     `mapstructure:"ssrf_dns_rebind_protection"` // default: true
}
```

**Permissions:**
None (infrastructure security, no RBAC needed)

**Implementation Notes:**
- Apply to webhook worker outbound HTTP calls
- Apply to any future outbound HTTP features (OAuth callbacks, inbound webhooks)
- Block RFC 1918 private ranges, link-local, loopback, cloud metadata (169.254.169.254)
- DNS rebinding protection: resolve hostname, check resolved IP, not original
- Configurable allowlist for development (allow private IPs)
- Follows existing config pattern (`webhook.go`, `email.go`)
- Wrap `http.Client` with SSRF-safe transport

**Endpoints:**
None (internal security layer, applied in webhook worker and HTTP clients)

---

#### 1.3 OAuth 2.0 Social Login

**Complexity:** Medium-High  
**Effort:** 3-4 days  
**Dependencies:** Existing JWT auth system

**Description:**
"Sign in with Google/GitHub/Microsoft" — expected by every SaaS user in 2026. Also enables enterprise SSO via SAML/OIDC bridges.

**Entities:**
```go
type OAuthProvider struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Name         string         `gorm:"size:50;uniqueIndex;not null" json:"name"`  // "google", "github", "microsoft"
    DisplayName  string         `gorm:"size:100;not null" json:"display_name"`
    ClientID     string         `gorm:"size:500;not null" json:"-"`
    ClientSecret string         `gorm:"size:500;not null" json:"-"`  // encrypted at rest
    AuthURL      string         `gorm:"size:500;not null" json:"auth_url"`
    TokenURL     string         `gorm:"size:500;not null" json:"token_url"`
    UserInfoURL  string         `gorm:"size:500;not null" json:"user_info_url"`
    Scopes       datatypes.JSON `gorm:"type:jsonb" json:"scopes"`
    Enabled      bool           `gorm:"default:true" json:"enabled"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (OAuthProvider) TableName() string { return "oauth_providers" }

type OAuthAccount struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    Provider     string         `gorm:"size:50;not null;index" json:"provider"`   // "google", "github"
    ProviderID   string         `gorm:"size:255;not null" json:"provider_id"`     // external user ID
    Email        string         `gorm:"size:255" json:"email"`
    AccessToken  string         `gorm:"size:2000" json:"-"`  // encrypted
    RefreshToken string         `gorm:"size:2000" json:"-"`  // encrypted
    TokenExpiry  *time.Time     `json:"token_expiry,omitempty"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (OAuthAccount) TableName() string { return "oauth_accounts" }
// Unique: (provider, provider_id) WHERE deleted_at IS NULL
// Unique: (user_id, provider) WHERE deleted_at IS NULL
```

**Service Pattern:**
```go
type OAuthService struct {
    providerRepo OAuthProviderRepository
    accountRepo  OAuthAccountRepository
    userRepo     repository.UserRepository
    enforcer     *permission.Enforcer
    audit        *service.AuditService
    jwtSecret    string
    eventBus     *domain.EventBus
}

func (s *OAuthService) Authorize(ctx context.Context, provider string) (string, error)  // Returns redirect URL
func (s *OAuthService) Callback(ctx context.Context, provider, code string) (*AuthTokens, error)  // Exchange code for tokens
func (s *OAuthService) Link(ctx context.Context, userID uuid.UUID, provider, code string) (*OAuthAccount, error)  // Link to existing user
func (s *OAuthService) Unlink(ctx context.Context, userID uuid.UUID, provider string) error  // Unlink provider
func (s *OAuthService) ListProviders(ctx context.Context) ([]OAuthProvider, error)  // List enabled providers
func (s *OAuthService) ListLinkedAccounts(ctx context.Context, userID uuid.UUID) ([]OAuthAccount, error)  // User's linked accounts
```

**Configuration:**
```go
type OAuthConfig struct {
    Enabled          bool          `mapstructure:"oauth_enabled"`
    StateTTL         time.Duration `mapstructure:"oauth_state_ttl"`         // 10m
    GitHubClientID   string        `mapstructure:"oauth_github_client_id"`
    GitHubClientSecret string      `mapstructure:"oauth_github_client_secret"`
    GoogleClientID   string        `mapstructure:"oauth_google_client_id"`
    GoogleClientSecret string      `mapstructure:"oauth_google_client_secret"`
    MicrosoftClientID string      `mapstructure:"oauth_microsoft_client_id"`
    MicrosoftClientSecret string  `mapstructure:"oauth_microsoft_client_secret"`
    EncryptionKey     string        `mapstructure:"oauth_encryption_key"`   // AES-256 for token encryption
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `oauth:manage` | oauth | manage | Configure OAuth providers (admin) |
| `oauth:link` | oauth | link | Link/unlink own social accounts |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/auth/oauth/:provider` | Redirect to OAuth provider |
| `GET` | `/api/v1/auth/oauth/:provider/callback` | OAuth callback (exchanges code) |
| `POST` | `/api/v1/auth/oauth/:provider/link` | Link provider to existing account |
| `DELETE` | `/api/v1/auth/oauth/:provider/unlink` | Unlink provider from account |
| `GET` | `/api/v1/auth/oauth/providers` | List available OAuth providers |
| `GET` | `/api/v1/auth/oauth/accounts` | List current user's linked accounts |

**Migration:** `migrations/000027_oauth_providers.up.sql`, `000028_oauth_accounts.up.sql`

**EventBus Integration:**
- `auth.oauth.linked` — when user links a social account
- `auth.oauth.unlinked` — when user unlinks a social account

**Implementation Notes:**
- Encrypted token storage (AES-256-GCM) for OAuth access/refresh tokens
- Account linking: multiple OAuth providers → one user account
- Auto-create user on first OAuth login (with email from provider)
- PKCE support for public clients (mobile/SPA)
- State parameter with HMAC verification to prevent CSRF
- Follows existing auth pattern (`internal/service/auth.go`)
- Integrates with existing `refresh_token.go` domain for JWT token generation
- Pre-configured providers (GitHub, Google, Microsoft) with sensible defaults

---

### Tier 2: SaaS Essentials

---

#### 2.1 Session Management

**Complexity:** Low-Medium  
**Effort:** 2 days  
**Dependencies:** Existing refresh token system

**Description:**
Users need "log out of all devices," view active sessions, and revoke compromised tokens. Security best practice that complements existing JWT auth.

**Entities:**
```go
type Session struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    RefreshToken string         `gorm:"size:500;uniqueIndex;not null" json:"-"`  // hashed refresh token
    Device       string         `gorm:"size:255" json:"device"`     // "Chrome on Windows"
    Browser      string         `gorm:"size:100" json:"browser"`    // "Chrome 120"
    OS           string         `gorm:"size:100" json:"os"`         // "Windows 11"
    IPAddress   string         `gorm:"size:45" json:"ip_address"`  // IPv6-compatible
    UserAgent    string         `gorm:"size:500" json:"user_agent"`
    LastActiveAt time.Time      `gorm:"index" json:"last_active_at"`
    ExpiresAt    time.Time      `gorm:"index;not null" json:"expires_at"`
    RevokedAt    *time.Time     `gorm:"index" json:"revoked_at,omitempty"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
}
func (Session) TableName() string { return "sessions" }
```

**Service Pattern:**
```go
type SessionService struct {
    repo     SessionRepository
    userRepo repository.UserRepository
    enforcer *permission.Enforcer
    audit    *service.AuditService
}

func (s *SessionService) Create(ctx context.Context, userID uuid.UUID, deviceInfo DeviceInfo) (*Session, error)
func (s *SessionService) ListByUser(ctx context.Context, userID uuid.UUID) ([]Session, error)
func (s *SessionService) Revoke(ctx context.Context, userID, sessionID uuid.UUID) error
func (s *SessionService) RevokeAll(ctx context.Context, userID uuid.UUID, exceptSessionID ...uuid.UUID) error  // "log out everywhere"
func (s *SessionService) RevokeExpired(ctx context.Context) (int64, error)  // cleanup job
```

**Configuration:**
```go
type SessionConfig struct {
    MaxSessionsPerUser int           `mapstructure:"session_max_per_user"` // default: 10
    SessionTTL         time.Duration `mapstructure:"session_ttl"`           // default: 720h (30d)
    CleanupInterval    time.Duration `mapstructure:"session_cleanup_interval"` // default: 1h
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `session:view` | session | view | View own sessions |
| `session:manage` | session | manage | Revoke sessions (self or others, admin) |
| `session:revoke_all` | session | revoke_all | Revoke all sessions for any user (admin) |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/sessions` | List current user's sessions |
| `DELETE` | `/api/v1/sessions/:id` | Revoke specific session |
| `DELETE` | `/api/v1/sessions` | Revoke all sessions ("log out everywhere") |

**Migration:** `migrations/000027_sessions.up.sql` (or next available number)

**EventBus Integration:**
- `auth.session.created` — new session created
- `auth.session.revoked` — session revoked

**Implementation Notes:**
- Extends existing `internal/domain/refresh_token.go` — sessions wrap refresh tokens
- Device/browser/OS parsed from `User-Agent` header using `ua-parser`
- `RevokeAll` option to keep current session alive (`exceptSessionID`)
- Background reaper for expired session cleanup
- Audit logging on revoke actions
- `IPAddress` populated from `X-Forwarded-For` or `RemoteAddr`

---

#### 2.2 Rate Limiting Tiers (Per-Plan)

**Complexity:** Low  
**Effort:** 1-2 days  
**Dependencies:** Existing rate limiter + organization system

**Description:**
Current rate limiting is flat. SaaS needs different limits per plan (free: 100/hr, pro: 1000/hr, enterprise: unlimited). Enables monetization.

**Entities:**
```go
type RateLimitPlan struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Name        string         `gorm:"size:100;uniqueIndex;not null" json:"name"` // "free", "pro", "enterprise"
    Description string         `gorm:"type:text" json:"description"`
    Limits      datatypes.JSON `gorm:"type:jsonb;not null" json:"limits"` // {"api":1000, "uploads":100, "exports":50}
    IsDefault   bool           `gorm:"default:false" json:"is_default"`
    IsSystem    bool           `gorm:"default:false" json:"is_system"` // protected from deletion
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (RateLimitPlan) TableName() string { return "rate_limit_plans" }

// Extends Organization entity (add column via migration)
// Organization.RateLimitPlanID uuid.UUID `gorm:"type:uuid;index"`
```

**Service Pattern:**
```go
type RateLimitPlanService struct {
    repo     RateLimitPlanRepository
    enforcer *permission.Enforcer
    audit    *service.AuditService
}

// Modified rate limiter middleware
func (m *RateLimitMiddleware) getLimitForUser(c echo.Context) (int, time.Duration) {
    // 1. Get user's organization
    // 2. Get organization's rate limit plan
    // 3. Parse limits from plan JSON
    // 4. Return limit for current endpoint category
}
```

**Configuration:**
```go
type RateLimitPlanConfig struct {
    DefaultPlan        string `mapstructure:"rate_limit_default_plan"`         // "free"
    DefaultRequestsPerHour int `mapstructure:"rate_limit_default_rph"`          // 100
    EnablePerPlan       bool   `mapstructure:"rate_limit_per_plan_enabled"`     // default: true
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `rate_limit_plan:view` | rate_limit_plan | view | View rate limit plans |
| `rate_limit_plan:manage` | rate_limit_plan | manage | Create, update, delete plans |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/rate-limit-plans` | Create plan (admin) |
| `GET` | `/api/v1/rate-limit-plans` | List plans |
| `GET` | `/api/v1/rate-limit-plans/:id` | Get plan |
| `PUT` | `/api/v1/rate-limit-plans/:id` | Update plan (admin) |
| `DELETE` | `/api/v1/rate-limit-plans/:id` | Delete plan (admin, non-system only) |
| `GET` | `/api/v1/rate-limit/usage` | Current user's rate limit usage |

**Migration:** `migrations/000028_rate_limit_plans.up.sql` (add plan table + organization FK)

**Implementation Notes:**
- Extends existing `internal/http/middleware/rate_limit.go`
- JSONB `limits` field for flexible per-endpoint categories
- Default plan applied to organizations without explicit plan
- Returns standard rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`
- System plans (free, pro, enterprise) protected from deletion
- Audit logging on plan changes
- Follows existing organization scoping pattern

---

#### 2.3 API Versioning

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** None (Echo router groups)

**Description:**
With 17+ features and growing, breaking changes are inevitable. API versioning provides a clean deprecation path.

**Entities:**
None (middleware + routing configuration only)

**Middleware Pattern:**
```go
func APIVersionMiddleware(currentVersion string, deprecatedVersions []string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            // Set current API version in context
            c.Set("api_version", currentVersion)
            
            // Add standard headers
            c.Response().Header().Set("X-API-Version", currentVersion)
            
            // Check if client requests deprecated version
            accept := c.Request().Header.Get("Accept")
            if isDeprecated(accept, deprecatedVersions) {
                c.Response().Header().Set("Deprecation", "true")
                c.Response().Header().Set("Sunset", "2027-01-01") // Configurable
                c.Response().Header().Set("Link", "</api/v2>; rel=\"successor-version\"")
            }
            
            return next(c)
        }
    }
}
```

**Configuration:**
```go
type APIVersionConfig struct {
    CurrentVersion      string   `mapstructure:"api_current_version"`       // "v1"
    DeprecatedVersions  []string `mapstructure:"api_deprecated_versions"`  // ["v0"]
    SunsetDate          string   `mapstructure:"api_sunset_date"`          // "2027-01-01"
    EnableVersionHeader  bool    `mapstructure:"api_version_header"`      // true
}
```

**Permissions:**
None (infrastructure-level, no RBAC)

**Implementation Notes:**
- URL versioning only: `/api/v1/`, `/api/v2/`
- Deprecation headers per RFC 8594 (`Deprecation`, `Sunset`, `Link`)
- Echo route group per version (`e.Group("/api/v1")`, `e.Group("/api/v2")`)
- Shared handlers with version-specific request/response mapping
- `Swagger` per version (`/swagger/v1/`, `/swagger/v2/`)
- No database migration needed

---

#### 2.4 GDPR Data Export (Right of Portability)

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** Existing background job queue + export service

**Description:**
GDPR Article 20 requires users to request "all my data." Different from the admin-facing Data Import/Export — this is a user-facing self-service export.

**Entities:**
```go
type DataExportRequest struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    Format      string         `gorm:"size:10;not null;default:'json'" json:"format"` // "json", "csv"
    Status      string         `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, processing, completed, failed
    FileURL     string         `gorm:"size:500" json:"file_url,omitempty"`
    ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
    CompletedAt *time.Time     `json:"completed_at,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (DataExportRequest) TableName() string { return "data_export_requests" }
```

**Service Pattern:**
```go
type DataExportService struct {
    repo       DataExportRequestRepository
    userRepo   repository.UserRepository
    jobService *JobService
    storage    storage.Driver
    enforcer   *permission.Enforcer
}

func (s *DataExportService) RequestExport(ctx context.Context, userID uuid.UUID, format string) (*DataExportRequest, error)
func (s *DataExportService) GetExportStatus(ctx context.Context, userID, id uuid.UUID) (*DataExportRequest, error)
func (s *DataExportService) DownloadExport(ctx context.Context, userID, id uuid.UUID) (io.ReadCloser, error)
func (s *DataExportService) CollectUserData(ctx context.Context, userID uuid.UUID) (*UserDataBundle, error)  // assembles all user data
```

**Configuration:**
```go
type DataExportConfig struct {
    MaxExportsPerMonth int           `mapstructure:"data_export_max_per_month"` // default: 3
    ExportTTL          time.Duration `mapstructure:"data_export_ttl"`           // default: 72h
    Formats            []string      `mapstructure:"data_export_formats"`       // default: ["json", "csv"]
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `data_export:request` | data_export | request | Request own data export |
| `data_export:download` | data_export | download | Download own export file |
| `data_export:manage` | data_export | manage | Manage all exports (admin) |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/me/data-export` | Request data export (authenticated user) |
| `GET` | `/api/v1/me/data-export` | Get export status |
| `GET` | `/api/v1/me/data-export/:id/download` | Download export file |

**Migration:** `migrations/000029_data_export_requests.up.sql`

**Implementation Notes:**
- Extends existing job queue for async processing
- Assembles data from: user profile, organizations, notifications, activities, comments, tags, media, invoices, audit logs
- Signed download URLs with expiry (72h default)
- Rate-limited to prevent abuse (max 3 exports/month per user)
- Uses existing storage driver (local/S3/MinIO) for file storage
- Follows existing export service pattern (`internal/service/export_service.go`)

---

### Tier 3: Production Hardening

---

#### 3.1 Circuit Breaker

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** None (wraps existing HTTP clients)

**Description:**
When external services (SendGrid, S3, SES) go down, circuit breaker prevents cascading failures. Opens circuit after N failures, half-opens to test recovery, closes when healthy.

**Entities:**
```go
type CircuitBreaker struct {
    mu              sync.Mutex
    name            string
    state           CircuitState // Closed, Open, HalfOpen
    failures        int
    successes       int
    maxFailures     int
    timeout         time.Duration
    halfOpenMax     int
    lastFailureTime time.Time
    onStateChange   func(name string, from, to CircuitState)
}

type CircuitState int
const (
    CircuitClosed   CircuitState = iota
    CircuitOpen
    CircuitHalfOpen
)
```

**Service Pattern:**
```go
type CircuitBreakerService struct {
    breakers map[string]*CircuitBreaker
    redis    *redis.Client
    mu       sync.RWMutex
}

func (s *CircuitBreakerService) Execute(ctx context.Context, name string, fn func() error) error
func (s *CircuitBreakerService) GetState(ctx context.Context, name string) (CircuitState, error)
func (s *CircuitBreakerService) GetMetrics(ctx context.Context) (map[string]CircuitMetrics, error)
func (s *CircuitBreakerService) Reset(ctx context.Context, name string) error
```

**Configuration:**
```go
type CircuitBreakerConfig struct {
    Enabled           bool          `mapstructure:"circuit_breaker_enabled"`
    MaxFailures       int           `mapstructure:"circuit_breaker_max_failures"`    // default: 5
    Timeout           time.Duration `mapstructure:"circuit_breaker_timeout"`         // default: 30s
    HalfOpenMax       int           `mapstructure:"circuit_breaker_half_open_max"`   // default: 3
    SuccessThreshold  int           `mapstructure:"circuit_breaker_success_threshold"` // default: 3
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `circuit_breaker:view` | circuit_breaker | view | View circuit breaker states |
| `circuit_breaker:manage` | circuit_breaker | manage | Reset circuit breakers (admin) |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/circuit-breakers` | List all circuit breaker states |
| `GET` | `/api/v1/circuit-breakers/:name` | Get specific circuit breaker state |
| `POST` | `/api/v1/circuit-breakers/:name/reset` | Reset circuit breaker (admin) |

**Implementation Notes:**
- Redis-backed state for multi-instance coordination
- Wraps existing HTTP clients in `email_sendgrid_provider.go`, `email_ses_provider.go`, `storage_s3.go`
- Follows Go circuit breaker pattern (similar to `github.com/sony/gobreaker` but custom)
- State transitions: Closed → Open (threshold reached) → HalfOpen (timeout expired) → Closed (successes reach threshold)
- Metrics exposed via existing `/healthz` endpoint (future: Prometheus)
- No database table needed — Redis-backed only

---

#### 3.2 OpenTelemetry Integration

**Complexity:** Medium  
**Effort:** 2-3 days  
**Dependencies:** None (complements existing structured logging)

**Description:**
Production debugging requires distributed tracing. Already have structured logging + request IDs — OpenTelemetry extends this to traces + metrics across services.

**Entities:**
None (infrastructure middleware + instrumentation only)

**Middleware Pattern:**
```go
func OTelMiddleware(serviceName string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            tracer := otel.Tracer(serviceName)
            ctx, span := tracer.Start(c.Request().Context(), spanName(c))
            defer span.End()
            
            // Propagate trace context
            c.SetRequest(c.Request().WithContext(ctx))
            
            // Add trace ID to response header
            c.Response().Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
            
            err := next(c)
            if err != nil {
                span.RecordError(err)
                span.SetStatus(codes.Error, err.Error())
            }
            return err
        }
    }
}
```

**Configuration:**
```go
type OTelConfig struct {
    Enabled          bool   `mapstructure:"otel_enabled"`
    ServiceName      string `mapstructure:"otel_service_name"`       // "go-api"
    ExporterType     string `mapstructure:"otel_exporter_type"`      // "otlp", "jaeger", "prometheus"
    ExporterEndpoint string `mapstructure:"otel_exporter_endpoint"`  // "localhost:4317"
    SamplingRate     float64 `mapstructure:"otel_sampling_rate"`    // 0.1 (10%)
    MetricsEnabled   bool   `mapstructure:"otel_metrics_enabled"`    // true
    TracesEnabled    bool   `mapstructure:"otel_traces_enabled"`    // true
}
```

**Permissions:**
None (infrastructure-level, admin dashboard only)

**Implementation Notes:**
- Integration with existing `internal/http/middleware/structured_logging.go`
- Trace ID already propagated in context; bridge to OpenTelemetry span context
- Exports to OTLP (vendor-neutral), Jaeger (traces), or Prometheus (metrics)
- Minimal overhead when disabled (`otel_enabled=false`)
- Replaces `X-Trace-ID` header with proper span context
- Metrics: request count, latency histogram, error rate per endpoint
- No database migration needed

---

#### 3.3 Enhanced Health Checks

**Complexity:** Low  
**Effort:** 0.5 days  
**Dependencies:** None

**Description:**
Current `/healthz` and `/readyz` are basic. Kubernetes needs component-level health with detailed status for each dependency.

**Entities:**
None (extends existing `internal/http/handler/health.go`)

**Handler Pattern:**
```go
type HealthDetail struct {
    Status    string        `json:"status"`    // "healthy", "degraded", "unhealthy"
    Component string        `json:"component"` // "database", "redis", "email", "storage"
    Latency   time.Duration `json:"latency"`
    Error     string        `json:"error,omitempty"`
}

func (h *HealthHandler) DetailedHealth(c echo.Context) error {
    components := []HealthDetail{
        h.checkDatabase(c),
        h.checkRedis(c),
        h.checkStorage(c),
        h.checkEmailProvider(c),
        h.checkWorkers(c),      // job queue, webhook worker, email worker
    }
    
    overall := "healthy"
    for _, comp := range components {
        if comp.Status == "unhealthy" { overall = "unhealthy"; break }
        if comp.Status == "degraded" && overall != "unhealthy" { overall = "degraded" }
    }
    
    status := http.StatusOK
    if overall == "unhealthy" { status = http.StatusServiceUnavailable }
    
    return c.JSON(status, map[string]interface{}{
        "status":     overall,
        "components": components,
        "version":    h.version,
        "uptime":      time.Since(h.startTime),
    })
}
```

**Configuration:**
```go
type HealthConfig struct {
    DetailedEnabled  bool          `mapstructure:"health_detailed_enabled"` // default: true
    MaxDBLatency     time.Duration `mapstructure:"health_max_db_latency"`  // default: 100ms
    MaxRedisLatency  time.Duration `mapstructure:"health_max_redis_latency"` // default: 50ms
    CheckWorkers     bool          `mapstructure:"health_check_workers"`     // default: true
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| (Basic health: no auth required; detailed health: `health:detailed` or admin) | | | |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Liveness (always 200) |
| `GET` | `/readyz` | Readiness (DB + Redis check) |
| `GET` | `/api/v1/health/detailed` | Detailed component health (admin) |

**Implementation Notes:**
- Extends existing `internal/http/handler/health.go`
- Component checks: PostgreSQL latency, Redis latency, storage driver, email provider, worker status
- Worker status: check if job/webhook/email workers are running
- `degraded` = latency above threshold but still functional
- `unhealthy` = connection failure or critical component down
- Existing `/healthz` and `/readyz` unchanged (Kubernetes compatibility)
- New `/api/v1/health/detailed` requires authentication + admin permission

---

### Tier 4: Advanced Features

---

#### 4.1 Inbound Webhook Processing

**Complexity:** Medium-High  
**Effort:** 3 days  
**Dependencies:** Existing webhook system + job queue

**Description:**
You have outbound webhooks. Inbound handles incoming events from Stripe, GitHub, Slack, etc. Need signature verification, routing, and processing.

**Entities:**
```go
type InboundWebhookConfig struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    OrganizationID uuid.UUID   `gorm:"type:uuid;index" json:"organization_id"`
    Name         string         `gorm:"size:255;not null" json:"name"`
    Provider     string         `gorm:"size:50;not null;index" json:"provider"` // "stripe", "github", "slack", "custom"
    Secret       string         `gorm:"size:500;not null" json:"-"`  // for signature verification
    EndpointPath string         `gorm:"size:100;uniqueIndex;not null" json:"endpoint_path"` // "stripe-payments"
    Events       datatypes.JSON `gorm:"type:jsonb" json:"events"` // ["payment.completed", "charge.failed"]
    Active       bool           `gorm:"default:true" json:"active"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (InboundWebhookConfig) TableName() string { return "inbound_webhook_configs" }

type InboundWebhookDelivery struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    ConfigID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"config_id"`
    Provider     string         `gorm:"size:50;not null;index" json:"provider"`
    EventType    string         `gorm:"size:100;not null;index" json:"event_type"`
    Payload      datatypes.JSON `gorm:"type:jsonb" json:"payload"`
    Headers      datatypes.JSON `gorm:"type:jsonb" json:"headers"`
    Status       string         `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, processing, completed, failed
    Attempts     int            `gorm:"default:0" json:"attempts"`
    Error        string         `gorm:"type:text" json:"error,omitempty"`
    ProcessedAt  *time.Time     `json:"processed_at,omitempty"`
    CreatedAt    time.Time       `json:"created_at"`
}
func (InboundWebhookDelivery) TableName() string { return "inbound_webhook_deliveries" }
```

**Service Pattern:**
```go
type InboundWebhookService struct {
    configRepo    InboundWebhookConfigRepository
    deliveryRepo  InboundWebhookDeliveryRepository
    enforcer      *permission.Enforcer
    audit         *service.AuditService
    eventBus      *domain.EventBus
    jobService    *service.JobService
}

func (s *InboundWebhookService) HandleIncoming(ctx context.Context, provider, path string, headers http.Header, body []byte) error
func (s *InboundWebhookService) VerifySignature(provider string, secret string, payload []byte, signature string) bool
func (s *InboundWebhookService) ProcessDelivery(ctx context.Context, deliveryID uuid.UUID) error
func (s *InboundWebhookService) ReplayDelivery(ctx context.Context, userID, deliveryID uuid.UUID) error
```

**Configuration:**
```go
type InboundWebhookConfig struct {
    Enabled           bool   `mapstructure:"inbound_webhook_enabled"`
    MaxPayloadSize    int64  `mapstructure:"inbound_webhook_max_payload"`  // 1MB
    RetryMaxAttempts  int    `mapstructure:"inbound_webhook_retry_max"`    // 3
    RetentionDays     int    `mapstructure:"inbound_webhook_retention_days"` // 90
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `inbound_webhook:view` | inbound_webhook | view | View inbound webhook configs and deliveries |
| `inbound_webhook:manage` | inbound_webhook | manage | Create, update, delete configs |
| `inbound_webhook:replay` | inbound_webhook | replay | Replay failed deliveries |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/webhooks/inbound/:path` | Receive inbound webhook (no auth, signature verified) |
| `GET` | `/api/v1/inbound-webhooks` | List inbound webhook configs |
| `POST` | `/api/v1/inbound-webhooks` | Create inbound webhook config |
| `GET` | `/api/v1/inbound-webhooks/:id` | Get config |
| `PUT` | `/api/v1/inbound-webhooks/:id` | Update config |
| `DELETE` | `/api/v1/inbound-webhooks/:id` | Delete config |
| `GET` | `/api/v1/inbound-webhooks/:id/deliveries` | List deliveries |
| `POST` | `/api/v1/inbound-webhooks/deliveries/:id/replay` | Replay delivery |

**Migration:** `migrations/000030_inbound_webhooks.up.sql`

**EventBus Integration:**
- `inbound_webhook.received.<provider>` — when a webhook is received
- `inbound_webhook.processed.<provider>` — after successful processing

**Implementation Notes:**
- Pre-built signature verifiers: Stripe (HMAC-SHA256), GitHub (HMAC-SHA256), Slack (signing secret)
- Custom provider support with raw body + header verification
- Receives at `/webhooks/inbound/:path` (no JWT auth, verified by signature)
- Processes via existing job queue (creates job for async processing)
- Idempotency key from provider webhook ID (prevents duplicate processing)
- 90-day delivery retention via reaper (follows webhook_reaper.go pattern)

---

#### 4.2 Scheduled Tasks / Cron

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Existing job queue

**Description:**
Job queue is fire-and-forget. Cron adds "run this every night at 2am" (analytics aggregation, cleanup, reports). Natural complement to existing job system.

**Entities:**
```go
type ScheduledTask struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    Name        string         `gorm:"size:255;uniqueIndex;not null" json:"name"` // "nightly-analytics-aggregation"
    CronExpr    string         `gorm:"size:100;not null" json:"cron_expr"`        // "0 2 * * *"
    TaskType    string         `gorm:"size:50;not null;index" json:"task_type"`   // "analytics_aggregation", "data_cleanup"
    Payload     datatypes.JSON `gorm:"type:jsonb" json:"payload"`
    Enabled     bool           `gorm:"default:true" json:"enabled"`
    LastRunAt   *time.Time     `json:"last_run_at,omitempty"`
    NextRunAt   *time.Time     `gorm:"index" json:"next_run_at,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (ScheduledTask) TableName() string { return "scheduled_tasks" }
```

**Service Pattern:**
```go
type SchedulerService struct {
    repo     ScheduledTaskRepository
    jobSvc   *JobService
    enforcer *permission.Enforcer
    audit    *service.AuditService
}

func (s *SchedulerService) Create(ctx context.Context, req CreateScheduledTaskRequest) (*ScheduledTask, error)
func (s *SchedulerService) Update(ctx context.Context, id uuid.UUID, req UpdateScheduledTaskRequest) (*ScheduledTask, error)
func (s *SchedulerService) Trigger(ctx context.Context, id uuid.UUID) error  // Manual trigger
func (s *SchedulerService) Start(ctx context.Context) error                   // Start scheduler loop
func (s *SchedulerService) Stop() error
```

**Configuration:**
```go
type SchedulerConfig struct {
    Enabled          bool          `mapstructure:"scheduler_enabled"`
    CheckInterval    time.Duration `mapstructure:"scheduler_check_interval"` // default: 1m
    MaxConcurrent    int           `mapstructure:"scheduler_max_concurrent"` // default: 5
    Timezone         string        `mapstructure:"scheduler_timezone"`       // default: "UTC"
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `scheduler:view` | scheduler | view | View scheduled tasks |
| `scheduler:manage` | scheduler | manage | Create, update, delete, enable/disable tasks |
| `scheduler:trigger` | scheduler | trigger | Manually trigger a scheduled task |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/scheduled-tasks` | Create scheduled task |
| `GET` | `/api/v1/scheduled-tasks` | List scheduled tasks |
| `GET` | `/api/v1/scheduled-tasks/:id` | Get task |
| `PUT` | `/api/v1/scheduled-tasks/:id` | Update task |
| `DELETE` | `/api/v1/scheduled-tasks/:id` | Delete task |
| `POST` | `/api/v1/scheduled-tasks/:id/trigger` | Manual trigger |

**Migration:** `migrations/000031_scheduled_tasks.up.sql`

**Implementation Notes:**
- Cron expression parser (robfig/cron v3 or simple custom parser)
- Scheduler loop: check `next_run_at` every configurable interval, create Job on schedule
- Pre-seeded tasks: analytics aggregation, data cleanup, session reaper
- Follows existing worker/reaper patterns (AggregationWorker, ActivityReaper)
- Startup wiring: create scheduler → register pre-seeded tasks → start loop
- Graceful shutdown: stop scheduler → wait for running jobs
- LastRunAt/NextRunAt for tracking and rescheduling

---

#### 4.3 Bulk Operations API

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Existing job queue

**Description:**
SaaS APIs need batch create/update/delete (create 100 invoices at once, bulk delete media). Uses existing job queue for async processing.

**Entities:**
```go
type BulkOperation struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
    UserID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
    Operation    string         `gorm:"size:20;not null" json:"operation"`  // "create", "update", "delete"
    ResourceType string         `gorm:"size:50;not null;index" json:"resource_type"` // "invoice", "media"
    Total        int            `gorm:"not null" json:"total"`
    Processed    int            `gorm:"default:0" json:"processed"`
    Succeeded    int            `gorm:"default:0" json:"succeeded"`
    Failed       int            `gorm:"default:0" json:"failed"`
    Errors       datatypes.JSON `gorm:"type:jsonb" json:"errors,omitempty"` // [{index, error}]
    Status       string         `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, processing, completed, failed
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
func (BulkOperation) TableName() string { return "bulk_operations" }
```

**Service Pattern:**
```go
type BulkOperationService struct {
    repo       BulkOperationRepository
    jobSvc     *JobService
    enforcer   *permission.Enforcer
    audit      *service.AuditService
    handlers   map[string]BulkHandler // registry by resource type
}

func (s *BulkOperationService) Create(ctx context.Context, userID uuid.UUID, op string, resourceType string, items []json.RawMessage) (*BulkOperation, error)
func (s *BulkOperationService) GetByID(ctx context.Context, id uuid.UUID) (*BulkOperation, error)
func (s *BulkOperationService) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]BulkOperation, int64, error)
func (s *BulkOperationService) Cancel(ctx context.Context, userID, id uuid.UUID) error
```

**Configuration:**
```go
type BulkConfig struct {
    MaxBatchSize       int `mapstructure:"bulk_max_batch_size"`        // default: 100
    MaxConcurrentBatch int `mapstructure:"bulk_max_concurrent_batch"`  // default: 5
}
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `bulk:create` | bulk | create | Create bulk operations |
| `bulk:view` | bulk | view | View bulk operation status |
| `bulk:cancel` | bulk | cancel | Cancel bulk operations |
| `bulk:manage` | bulk | manage | Manage all bulk operations (admin) |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/bulk/:resource_type` | Create bulk operation |
| `GET` | `/api/v1/bulk` | List user's bulk operations |
| `GET` | `/api/v1/bulk/:id` | Get operation status |
| `POST` | `/api/v1/bulk/:id/cancel` | Cancel operation |

**Implementation Notes:**
- Follows existing job queue pattern for async processing
- Entity registry pattern (extensible): register handlers per resource type
- Partial success semantics: individual item failures don't stop the batch
- Error details per item in `errors` JSONB field
- Max batch size 100 (configurable)
- Audit logging on create and complete

---

#### 4.4 Soft Delete Recovery

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** Soft delete pattern (all entities)

**Description:**
Everything uses soft delete but there's no "undo" endpoint. Admins need to recover accidentally deleted resources.

**Entities:**
None (reuses existing `deleted_at` field on all entities)

**Service Pattern:**
```go
type RecoveryService struct {
    repos     map[string]interface{} // registry of repositories by resource type
    enforcer  *permission.Enforcer
    audit     *service.AuditService
}

func (s *RecoveryService) Recover(ctx context.Context, userID uuid.UUID, resourceType, resourceID string) error
func (s *RecoveryService) ListDeleted(ctx context.Context, userID uuid.UUID, resourceType string, limit, offset int) ([]interface{}, int64, error)
```

**Permissions:**
| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `recovery:view` | recovery | view | View deleted resources |
| `recovery:manage` | recovery | manage | Recover deleted resources (admin) |

**Endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/recovery/:resource_type` | List soft-deleted resources |
| `POST` | `/api/v1/recovery/:resource_type/:id/restore` | Restore soft-deleted resource |

**Implementation Notes:**
- No database migration needed — clears `deleted_at` on existing entities
- Registry pattern: map resource type strings to repository interfaces
- Supports all entities with `deleted_at`: users, organizations, webhooks, comments, tags, etc.
- Audit logging on recovery (before/after state)
- 30-day recovery window recommended (configurable per resource type)
- Admin-only permission (`recovery:manage`)

---

## Implementation Priority Matrix

| Feature | Priority | Complexity | Effort | Business Value | Builds On | Recommend |
|---------|----------|------------|--------|----------------|-----------|-----------|
| Idempotency Keys | P1-Security | Medium | 2d | Critical | Redis + middleware | ✅ Start Here |
| SSRF Protection | P1-Security | Low | 1-2d | Critical | Webhook worker | ✅ Security |
| OAuth Social Login | P1-Security | Medium-High | 3-4d | High | JWT auth system | ✅ User Expectation |
| Session Management | P2-SaaS | Low-Medium | 2d | High | Refresh tokens | ⚡ Quick Win |
| Rate Limit Tiers | P2-SaaS | Low | 1-2d | High | Rate limiter + orgs | ⚡ Monetization |
| API Versioning | P2-SaaS | Low | 1d | Medium | Echo router groups | ⚡ Future-proof |
| GDPR Data Export | P2-SaaS | Low | 1d | Medium | Export service + job queue | ⚡ Compliance |
| Circuit Breaker | P3-Production | Medium | 2d | Medium | HTTP clients | 🔧 Resilience |
| OpenTelemetry | P3-Production | Medium | 2-3d | Medium | Structured logging | 🔧 Observability |
| Enhanced Health | P3-Production | Low | 0.5d | Low | Health handler | 🔧 K8s-ready |
| Inbound Webhooks | P4-Advanced | Medium-High | 3d | Medium | Webhook system + job queue | 📡 Integration |
| Scheduled Tasks | P4-Advanced | Medium | 2d | Medium | Job queue | 📡 Automation |
| Bulk Operations | P4-Advanced | Medium | 2d | Medium | Job queue | 📡 Efficiency |
| Soft Delete Recovery | P4-Advanced | Low | 1d | Low-Medium | Soft delete pattern | 🔧 Safety Net |

---

## Recommended Implementation Order

### Sprint 1 (Week 1): Security Foundation
1. **Idempotency Keys** — prevents double-operations across all POST/PUT/PATCH endpoints
2. **SSRF Protection** — blocks internal network scanning via webhook URLs

### Sprint 2 (Week 2): Auth Completion
3. **OAuth Social Login** — expected by every modern SaaS user
4. **Session Management** — "log out everywhere" security feature

### Sprint 3 (Week 3): SaaS Readiness
5. **Rate Limiting Tiers** — enables pricing differentiation
6. **API Versioning** — future-proofs all 17+ endpoints
7. **GDPR Data Export** — compliance requirement for EU users

### Sprint 4+ (As Needed)
8. Circuit Breaker → OpenTelemetry → Enhanced Health → Inbound Webhooks → Scheduled Tasks → Bulk Operations → Soft Delete Recovery

---

## Architecture Notes

### EventBus Extension Points

Current EventBus events (published by Invoice, News, User services):
```
user.created, user.deleted, invoice.created, invoice.paid, news.published, news.deleted
```

Activity feed maps 7 types, Analytics maps 11 types. New V2 features should add:
- `auth.oauth.linked`, `auth.oauth.unlinked` — OAuth provider changes
- `auth.session.created`, `auth.session.revoked` — Session events
- `circuit_breaker.state_changed` — Circuit state transitions
- `inbound_webhook.received`, `inbound_webhook.processed` — Inbound processing
- `bulk_operation.started`, `bulk_operation.completed` — Bulk status changes

### Startup Integration Pattern

Following existing pattern from `cmd/api/main.go`:
```go
// V2 services initialization
idempotencyMiddleware := middleware.NewIdempotencyMiddleware(redisClient, cfg.Idempotency)
ssrfValidator := service.NewSSRFValidator(cfg.SSRF)
oauthService := service.NewOAuthService(oauthRepo, accountRepo, userRepo, enforcer, audit, cfg.OAuth)
sessionService := service.NewSessionService(sessionRepo, userRepo, enforcer, audit, cfg.Session)
rateLimitPlanService := service.NewRateLimitPlanService(rateLimitPlanRepo, enforcer, audit)
schedulerService := service.NewSchedulerService(schedulerRepo, jobSvc, enforcer, audit, cfg.Scheduler)

// V2 workers/reapers
sessionReaper := service.NewSessionReaper(sessionRepo, cfg.Session, slog.Default())
circuitBreakerService := service.NewCircuitBreakerService(cfg.CircuitBreaker, redisClient)
schedulerService.Start(ctx)

// Graceful shutdown order (append to existing):
// server → eventBus → activityReaper → analyticsReaper → workers → sessionReaper → scheduler → enforcer → db → redis
```

---

## Files Referenced

- Source: `docs/FEATURE_RECOMMENDATIONS.md` (V1)
- This doc: `docs/FEATURE_RECOMMENDATIONS_V2.md`
- Status: `docs/FEATURE_STATUS.md`