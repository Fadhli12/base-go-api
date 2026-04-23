# Go API Base - Feature Recommendations

**Generated:** 2026-04-23  
**Purpose:** Catalog of features that can be built on this base API  

---

## Overview

This document provides recommendations for new features that can be implemented on top of the existing Go API Base. Features are categorized by priority and complexity.

---

## Existing Features Summary

### Core Infrastructure ✅

| Feature | Status | Notes |
|---------|--------|-------|
| JWT Authentication | ✅ Complete | Access (15m) + Refresh (30d) tokens |
| RBAC with Casbin | ✅ Complete | Roles, permissions, scopes |
| Permission Cache | ✅ Complete | Redis with pub/sub invalidation |
| Audit Logging | ✅ Complete | Before/after JSONB, immutable |
| Rate Limiting | ✅ Complete | Redis-backed sliding window |
| Health Checks | ✅ Complete | `/healthz`, `/readyz` |
| Graceful Shutdown | ✅ Complete | 30-second timeout |
| Swagger Docs | ✅ Complete | Auto-generated OpenAPI |

### Domain Entities ✅

| Entity | Status | Pattern |
|--------|--------|---------|
| User | ✅ Complete | Soft delete, UUID |
| Role | ✅ Complete | System roles protected |
| Permission | ✅ Complete | Resource:action:scope model |
| News | ✅ Complete | Status workflow (draft→published→archived) |
| Media | ✅ Complete | Polymorphic, conversions, multiple disks |
| Invoice | ✅ Complete | Domain module example with ownership |
| AuditLog | ✅ Complete | Immutable, JSONB before/after |

### Infrastructure ✅

| Component | Options | Notes |
|-----------|---------|-------|
| Storage | Local, S3, MinIO | Configurable via env |
| Cache | Redis, Memory, None | Driver pattern |
| Database | PostgreSQL | GORM, SQL migrations |
| Image Processing | Thumbnails, Previews | Configurable quality/dimensions |

---

## Feature Recommendations

### Priority 1: Essential Business Features

#### 1.1 Team/Organization Support

**Complexity:** Medium  
**Effort:** 2-3 days  
**Dependencies:** None (extends existing RBAC)

**Description:**
Multi-tenant support allowing users to belong to organizations with organization-scoped resources.

**Entities to Add:**
```go
// Organization entity
type Organization struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Name        string         `gorm:"size:255;not null"`
    Slug        string         `gorm:"uniqueIndex;size:100"`
    OwnerID     uuid.UUID      `gorm:"type:uuid;not null"`
    Settings    datatypes.JSON `gorm:"type:jsonb"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// Organization membership
type OrganizationMember struct {
    OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey"`
    UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
    Role           string    `gorm:"size:50;not null"` // owner, admin, member
    JoinedAt       time.Time `gorm:"autoCreateTime"`
}
```

**Permission Model:**
```
organization:view - View organization details
organization:manage - Manage organization settings
organization:invite - Invite new members
organization:remove - Remove members
```

**Implementation Notes:**
- Use existing Casbin RBAC with domain support
- Add `X-Organization-ID` header for tenant context
- Scope all resources to organization
- Migration: Add organization foreign key to existing entities

---

#### 1.2 API Key Authentication

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** None

**Description:**
Service-to-service authentication and API access for external integrations.

**Entities to Add:**
```go
type APIKey struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    Name        string         `gorm:"size:255;not null"`
    KeyHash     string         `gorm:"size:255;not null;uniqueIndex"`
    Prefix      string         `gorm:"size:8;not null"` // Visible prefix for identification
    Scopes      datatypes.JSON `gorm:"type:jsonb"` // ["read:invoices", "write:news"]
    ExpiresAt   *time.Time
    LastUsedAt  *time.Time
    CreatedAt   time.Time
    RevokedAt   *time.Time     `gorm:"index"`
}
```

**Middleware Pattern:**
```go
func APIKeyAuth(apiKeyService *APIKeyService) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := c.Request().Header.Get("X-API-Key")
            if key == "" {
                return next(c) // Fall through to JWT
            }
            
            apiKey, err := apiKeyService.Validate(c.Request().Context(), key)
            if err != nil {
                return echo.NewHTTPError(401, "Invalid API key")
            }
            
            // Set API key context
            c.Set("api_key", apiKey)
            c.Set("scopes", apiKey.Scopes)
            return next(c)
        }
    }
}
```

**Implementation Notes:**
- API keys have scope limitations (not full permissions)
- Rate limit separately from user sessions
- Log all API key usage in audit trail
- Allow key rotation without revocation

---

#### 1.3 Email Service

**Complexity:** Low-Medium  
**Effort:** 1-2 days  
**Dependencies:** External SMTP service

**Description:**
Transactional email sending with templates for notifications, password reset, invitations.

**Configuration:**
```go
type EmailConfig struct {
    Driver      string // "smtp", "ses", "sendgrid", "mailgun"
    SMTPHost    string
    SMTPPort    int
    SMTPUser    string
    SMTPPass    string
    FromAddress string
    FromName    string
}
```

**Service Interface:**
```go
type EmailService interface {
    Send(ctx context.Context, to, subject, template string, data map[string]interface{}) error
    SendBatch(ctx context.Context, recipients []string, subject, template string, data map[string]interface{}) error
    Queue(ctx context.Context, to, subject, template string, data map[string]interface{}) error
}
```

**Templates:**
```
templates/emails/
├── welcome.html
├── password-reset.html
├── invitation.html
├── invoice-created.html
└── news-published.html
```

**Implementation Notes:**
- Multiple backend support (SMTP, AWS SES, SendGrid)
- Template rendering with html/template
- Queue for async sending (Redis-backed)
- Retry logic with exponential backoff
- Track email status (sent, delivered, bounced)

---

#### 1.4 Notification System

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Email service (optional)

**Description:**
In-app notifications for user activities, mentions, alerts with optional email/webhook delivery.

**Entities:**
```go
type Notification struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    Type        string         `gorm:"size:50;not null"` // info, warning, error, success
    Title       string         `gorm:"size:255;not null"`
    Message     string         `gorm:"type:text"`
    ActionURL   string         `gorm:"size:500"` // Optional link
    ReadAt      *time.Time     `gorm:"index"`
    CreatedAt   time.Time
}

type NotificationPreference struct {
    UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
    NotificationType string   `gorm:"size:50;primaryKey"`
    EmailEnabled   bool      `gorm:"default:false"`
    PushEnabled    bool      `gorm:"default:true"` // Future: push notifications
}
```

**Service Pattern:**
```go
func (s *NotificationService) Notify(ctx context.Context, userID uuid.UUID, notif Notification) error {
    // 1. Store notification
    if err := s.repo.Create(ctx, &notif); err != nil {
        return err
    }
    
    // 2. Check preferences
    prefs, err := s.prefsRepo.Get(ctx, userID, notif.Type)
    if err == nil && prefs.EmailEnabled {
        // Queue email asynchronously
        s.emailQueue <- EmailJob{UserID: userID, Notification: notif}
    }
    
    // 3. Real-time delivery via WebSocket (future)
    s.wsHub.Broadcast <- WSMessage{UserID: userID, Data: notif}
    
    return nil
}
```

**Notification Types:**
- `mention` - User mentioned in content
- `assignment` - User assigned to task
- `system` - System announcements
- `invoice.created` - Invoice events
- `news.published` - News publication

---

#### 1.5 Webhook System

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** None

**Description:**
Outbound webhooks for external system integrations with retry and delivery tracking.

**Entities:**
```go
type Webhook struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    OrganizationID uuid.UUID   `gorm:"type:uuid;index"`
    Name        string         `gorm:"size:255;not null"`
    URL         string         `gorm:"size:500;not null"`
    Secret      string         `gorm:"size:255"` // HMAC signature
    Events      datatypes.JSON `gorm:"type:jsonb"` // ["invoice.created", "user.registered"]
    Active      bool           `gorm:"default:true"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"`
}

type WebhookDelivery struct {
    ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
    WebhookID     uuid.UUID `gorm:"type:uuid;not null;index"`
    Event         string    `gorm:"size:100;not null"`
    Payload       datatypes.JSON
    ResponseCode  int
    ResponseBody  string
    DeliveredAt   time.Time
    Duration      int64 // milliseconds
    Success       bool
    AttemptNumber int
}
```

**Events:**
```go
const (
    EventUserCreated    = "user.created"
    EventUserDeleted    = "user.deleted"
    EventInvoiceCreated = "invoice.created"
    EventInvoicePaid    = "invoice.paid"
    EventNewsPublished  = "news.published"
)
```

**Implementation Notes:**
- HMAC-SHA256 signature for security
- Retry logic: 3 attempts with exponential backoff
- Delivery log retention: 90 days
- Rate limit per webhook (100/minute default)
- Event filtering at subscription level

---

### Priority 2: Enhanced Features

#### 2.1 Two-Factor Authentication (2FA)

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Email service (for recovery)

**Description:**
TOTP-based two-factor authentication with backup codes.

**Entities:**
```go
type UserTwoFactor struct {
    ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
    UserID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
    Secret       string    `gorm:"size:255;not null"` // Encrypted TOTP secret
    Enabled      bool      `gorm:"default:false"`
    BackupCodes  datatypes.JSON `gorm:"type:jsonb"` // Hashed backup codes
    VerifiedAt   *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Flow:**
1. User enables 2FA → Generate secret
2. User verifies with authenticator app
3. Generate backup codes (10 codes)
4. Login requires code verification
5. Recovery via backup codes

---

#### 2.2 File Versioning

**Complexity:** Medium-High  
**Effort:** 2-3 days  
**Dependencies:** Media entity

**Description:**
Version history for uploaded files with diff tracking.

**Entities:**
```go
type MediaVersion struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
    MediaID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    Version      int            `gorm:"not null"`
    Filename     string         `gorm:"size:255;not null"`
    Path         string         `gorm:"size:2000;not null"`
    Size         int64          `gorm:"not null"`
    Checksum     string         `gorm:"size:64"` // SHA-256
    UploadedByID uuid.UUID      `gorm:"type:uuid;not null"`
    CreatedAt    time.Time
}

// Add to Media entity
type Media struct {
    // ... existing fields
    CurrentVersion int              `gorm:"default:1"`
    Versions       []*MediaVersion  `gorm:"foreignKey:MediaID"`
}
```

---

#### 2.3 Search & Filtering

**Complexity:** Medium-High  
**Effort:** 3-4 days  
**Dependencies:** None (PostgreSQL full-text)

**Description:**
Advanced search capabilities across entities with filters, sorting, and pagination.

**Features:**
- Full-text search (PostgreSQL `tsvector`)
- Faceted filtering
- Saved searches
- Search suggestions/autocomplete

**Implementation:**
```go
// Add search vectors to entities
type News struct {
    // ... existing fields
    SearchVector pgtype.Text `gorm:"type:tsvector" json:"-"`
}

// Search service
type SearchService struct {
    db *gorm.DB
}

func (s *SearchService) SearchNews(ctx context.Context, query SearchQuery) (*SearchResult, error) {
    var results []News
    
    tx := s.db.WithContext(ctx).Model(&News{})
    
    // Full-text search
    if query.Text != "" {
        tx = tx.Where("search_vector @@ plainto_tsquery('english', ?)", query.Text)
    }
    
    // Filters
    if query.Status != "" {
        tx = tx.Where("status = ?", query.Status)
    }
    if query.AuthorID != "" {
        tx = tx.Where("author_id = ?", query.AuthorID)
    }
    
    // Date range
    if !query.FromDate.IsZero() {
        tx = tx.Where("created_at >= ?", query.FromDate)
    }
    
    // Pagination
    var total int64
    tx.Count(&total)
    tx.Limit(query.Limit).Offset(query.Offset).Find(&results)
    
    return &SearchResult{
        Results: results,
        Total:   total,
        Page:    query.Page,
       PerPage: query.Limit,
    }, nil
}
```

**Migration:**
```sql
-- Add search vector column
ALTER TABLE news ADD COLUMN search_vector tsvector;

-- Create index
CREATE INDEX idx_news_search ON news USING GIN(search_vector);

-- Trigger to update vector
CREATE OR REPLACE FUNCTION news_search_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER news_search_trigger
BEFORE INSERT OR UPDATE ON news
FOR EACH ROW EXECUTE FUNCTION news_search_update();
```

---

#### 2.4 Comment System

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Notification system (optional)

**Description:**
Threaded comments on entities (news, invoices, etc.) with mentions and reactions.

**Entities:**
```go
type Comment struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
    ParentID     *uuid.UUID     `gorm:"type:uuid;index"` // For threading
    AuthorID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    CommentableType string      `gorm:"size:50;not null;index"` // "news", "invoice"
    CommentableID   uuid.UUID   `gorm:"type:uuid;not null;index"`
    Content      string         `gorm:"type:text;not null"`
    EditedAt     *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    gorm.DeletedAt `gorm:"index"`
}

type CommentReaction struct {
    ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
    CommentID   uuid.UUID `gorm:"type:uuid;not null;index"`
    UserID      uuid.UUID `gorm:"type:uuid;not null"`
    Reaction    string    `gorm:"size:20;not null"` // emoji or type
    CreatedAt   time.Time `gorm:"autoCreateTime"`
    UniqueConstraint: "idx_comment_reaction_user" // one reaction per user per comment
}
```

---

#### 2.5 Tagging System

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** None

**Description:**
Flexible tagging system for entities with tag autocomplete.

**Entities:**
```go
type Tag struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Name      string         `gorm:"size:100;not null;uniqueIndex"`
    Slug      string         `gorm:"size:100;not null;uniqueIndex"`
    Color     string         `gorm:"size:7"` // hex color
    UsageCount int           `gorm:"default:0"`
    CreatedAt time.Time

type EntityTag struct {
    ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
    EntityType string    `gorm:"size:50;not null"` // "news", "invoice", "media"
    EntityID   uuid.UUID `gorm:"type:uuid;not null"`
    TagID      uuid.UUID `gorm:"type:uuid;not null"`
    CreatedAt  time.Time `gorm:"autoCreateTime"`
    
    // Composite index
    // idx_entity_tag: (entity_type, entity_id, tag_id) UNIQUE
}

// Usage: Find all tags for a news article
func (r *TagRepository) FindByEntity(ctx context.Context, entityType string, entityID uuid.UUID) ([]Tag, error)
```

---

#### 2.6 Activity Feed / Timeline

**Complexity:** Medium  
**Effort:** 2 days  
**Dependencies:** Notification system

**Description:**
Chronological activity stream for users showing actions across the system.

**Entities:**
```go
type Activity struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    ActorID     uuid.UUID      `gorm:"type:uuid;not null;index"`
    ActionType  string         `gorm:"size:50;not null"` // "created", "updated", "deleted"
    ResourceType string        `gorm:"size:50;not null;index"` // "invoice", "news"
    ResourceID  string         `gorm:"size:100;index"`
    Title       string         `gorm:"size:255;not null"`
    Description string         `gorm:"type:text"`
    Metadata    datatypes.JSON `gorm:"type:jsonb"`
    CreatedAt   time.Time      `gorm:"autoCreateTime;index"`
}

// Predefined activities
const (
    ActivityInvoiceCreated   = "invoice.created"
    ActivityInvoicePaid      = "invoice.paid"
    ActivityNewsPublished    = "news.published"
    ActivityUserJoined       = "user.joined"
)
```

---

### Priority 3: Advanced Features

#### 3.1 Real-time Communication (WebSocket)

**Complexity:** High  
**Effort:** 3-4 days  
**Dependencies:** Redis for pub/sub

**Description:**
Real-time updates via WebSocket for live collaboration, notifications, and presence.

**Features:**
- Room-based broadcasting (organization, project)
- Presence indicators (online/offline)
- Typing indicators
- Real-time notifications

**Implementation:**
```go
type WebSocketHub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
    rooms      map[string]map[*Client]bool
}

type Client struct {
    hub    *WebSocketHub
    conn   *websocket.Conn
    send   chan []byte
    userID uuid.UUID
    rooms  map[string]bool
}

// Redis pub/sub for multi-instance
func (h *WebSocketHub) Subscribe(ctx context.Context, room string) {
    pubsub := h.redis.Subscribe(ctx, "ws:"+room)
    for msg := range pubsub.Channel() {
        h.BroadcastToRoom(room, msg.Payload)
    }
}
```

---

#### 3.2 Background Job Queue

**Complexity:** Medium-High  
**Effort:** 2-3 days  
**Dependencies:** Redis

**Description:**
Asynchronous job processing with retry, scheduling, and monitoring.

**Entities:**
```go
type Job struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Type        string         `gorm:"size:50;not null;index"`
    Payload     datatypes.JSON `gorm:"type:jsonb"`
    Status      string         `gorm:"size:20;not null;default:'pending';index"` // pending, running, completed, failed
    Priority    int            `gorm:"default:0"`
    Attempts    int            `gorm:"default:0"`
    MaxAttempts int            `gorm:"default:3"`
    Error       string         `gorm:"type:text"`
    Result      datatypes.JSON `gorm:"type:jsonb"`
    RunAt       *time.Time     `gorm:"index"` // Scheduled time
    StartedAt   *time.Time
    CompletedAt *time.Time
    CreatedAt   time.Time
}

type JobWorker struct {
    db       *gorm.DB
    handlers map[string]JobHandler
    queue    chan *Job
    workers  int
}

type JobHandler func(ctx context.Context, payload json.RawMessage) error
```

**Use Cases:**
- Email sending
- Report generation
- Data export
- Media processing
- Webhook delivery
- Data cleanup

---

#### 3.3 Analytics Dashboard

**Complexity:** High  
**Effort:** 4-5 days  
**Dependencies:** None (built-in PostgreSQL)

**Description:**
Usage analytics, metrics, and dashboard data aggregation.

**Entities:**
```go
type MetricEvent struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    EventType   string         `gorm:"size:100;not null;index"` // "page_view", "api_call", "invoice_created"
    UserID      *uuid.UUID     `gorm:"type:uuid;index"`
    Metadata    datatypes.JSON `gorm:"type:jsonb"`
    Timestamp   time.Time      `gorm:"autoCreateTime;index"`
    
    // Pre-aggregated fields for fast queries
    Date        time.Time      `gorm:"type:date;index"` // For daily aggregation
    Hour        int            `gorm:"index"` // 0-23 for hourly
}

type DashboardMetric struct {
    ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
    MetricType   string    `gorm:"size:50;not null;uniqueIndex"`
    PeriodType   string    `gorm:"size:20;not null"` // "daily", "weekly", "monthly"
    PeriodStart  time.Time `gorm:"not null;index"`
    PeriodEnd    time.Time `gorm:"not null;index"`
    Value        float64   `gorm:"not null"`
    Metadata     datatypes.JSON
    CalculatedAt time.Time
}
```

**Metrics:**
- Daily/weekly/monthly active users
- API endpoint usage
- Resource creation counts
- Error rates
- Response times

---

#### 3.4 Import/Export System

**Complexity:** Medium-High  
**Effort:** 3 days  
**Dependencies:** Background jobs

**Description:**
Bulk data import/export with CSV, JSON, Excel support.

**Entities:**
```go
type ImportJob struct {
    ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
    UserID       uuid.UUID      `gorm:"type:uuid;not null;index"`
    Type         string         `gorm:"size:50;not null"` // "users", "invoices"
    Format       string         `gorm:"size:10;not null"` // "csv", "json", "xlsx"
    FileURL      string         `gorm:"size:500"` // Upload URL
    Status       string         `gorm:"size:20;not null;default:'pending'"`
    Processed    int            `gorm:"default:0"`
    Succeeded    int            `gorm:"default:0"`
    Failed       int            `gorm:"default:0"`
    Errors       datatypes.JSON `gorm:"type:jsonb"` // Row-by-row errors
    CreatedAt    time.Time
    CompletedAt  *time.Time
}

type ExportJob struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
    Type        string         `gorm:"size:50;not null"`
    Format      string         `gorm:"size:10;not null"`
    Filters     datatypes.JSON `gorm:"type:jsonb"` // Query filters
    Status      string         `gorm:"size:20;not null;default:'pending'"`
    FileURL     string         `gorm:"size:500"` // Download URL
    ExpiresAt   *time.Time     // Download expiry
    CreatedAt   time.Time
    CompletedAt *time.Time
}
```

---

#### 3.5 Settings & Configuration

**Complexity:** Low  
**Effort:** 1 day  
**Dependencies:** None

**Description:**
User-level and system-wide settings with type validation.

**Entities:**
```go
type UserSetting struct {
    ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
    UserID    uuid.UUID      `gorm:"type:uuid;uniqueIndex;not null"`
    Theme     string         `gorm:"size:20;default:'system'"` // "light", "dark", "system"
    Language  string         `gorm:"size:10;default:'en'"`
    Timezone  string         `gorm:"size:50;default:'UTC'"`
    NotificationsEnabled bool `gorm:"default:true"`
    EmailDigestEnabled   bool `gorm:"default:false"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

type SystemSetting struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Key         string         `gorm:"size:100;uniqueIndex;not null"`
    Value       datatypes.JSON `gorm:"type:jsonb"`
    Type        string         `gorm:"size:20;not null"` // "string", "number", "boolean", "json"
    Category    string         `gorm:"size:50;index"` // "general", "security", "features"
    Description string         `gorm:"size:500"`
    IsPublic    bool           `gorm:"default:false"` // Expose to client
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

---

#### 3.6 Feature Flags

**Complexity:** Low-Medium  
**Effort:** 1-2 days  
**Dependencies:** None

**Description:**
Runtime feature toggles for gradual rollout and A/B testing.

**Entities:**
```go
type FeatureFlag struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Key         string         `gorm:"size:100;uniqueIndex;not null"`
    Name        string         `gorm:"size:255;not null"`
    Description string         `gorm:"type:text"`
    Enabled     bool           `gorm:"default:false"`
    Rollout     int            `gorm:"default:100"` // Percentage 0-100
    Conditions  datatypes.JSON `gorm:"type:jsonb"` // User segments, org IDs, etc.
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Usage in code
func (s *FeatureFlagService) IsEnabled(ctx context.Context, key string, userID uuid.UUID) bool {
    flag, err := s.repo.GetByKey(ctx, key)
    if err != nil || !flag.Enabled {
        return false
    }
    
    // Check rollout percentage
    if flag.Rollout < 100 {
        hash := md5.Sum([]byte(userID.String() + key))
        if int(hash[0])%100 >= flag.Rollout {
            return false
        }
    }
    
    // Check conditions
    // ...
    
    return true
}
```

---

## Implementation Priority Matrix

| Feature | Priority | Complexity | Effort | Business Value | Recommend |
|---------|----------|------------|--------|----------------|-----------|
| API Key Auth | High | Low | 1d | High | ✅ Start Here |
| Team/Org | High | Medium | 2-3d | High | ✅ High Value |
| Email Service | High | Low-Medium | 1-2d | High | ✅ Foundation |
| Notifications | High | Medium | 2d | High | ✅ High Value |
| Settings | Medium | Low | 1d | Medium | ✅ Quick Win |
| Feature Flags | Medium | Low-Medium | 1-2d | Medium | ✅ Quick Win |
| Webhooks | Medium | Medium | 2d | Medium | ⚡ Integration |
| Search | Medium | Medium-High | 3-4d | Medium | ⚡ UX |
| Comments | Medium | Medium | 2d | Medium | ⚡ Engagement |
| Tags | Medium | Low | 1d | Low-Medium | ⚡ Organization |
| 2FA | Medium | Medium | 2d | High |🔒 Security |
| File Versioning | Low | Medium-High | 2-3d | Low-Medium | 📁 Media Enhancement |
| Activity Feed | Low | Medium | 2d | Medium | 📊 Awareness |
| WebSocket | Low | High | 3-4d | Medium | ⚡ Real-time |
| Job Queue | Low | Medium-High | 2-3d | Medium | 🔄 Infrastructure |
| Analytics | Low | High | 4-5d | Low-Medium | 📊 Insights |
| Import/Export | Low | Medium-High | 3d | Low-Medium | 📦 Data |

---

## Recommended Implementation Order

### Phase 1 (Week 1-2): Foundation
1. **API Key Auth** - Essential for integrations
2. **Email Service** - Foundation for notifications
3. **Settings** - Quick win, enables customization

### Phase 2 (Week 3-4): Team & Security
4. **Team/Organization** - Multi-tenant support
5. **2FA** - Security enhancement
6. **Notifications** - User engagement

### Phase 3 (Week 5-6): Integration
7. **Webhooks** - External integration
8. **Feature Flags** - Gradual rollout capability
9. **Tags** - Content organization

### Phase 4 (Week 7-8): Enhancement
10. **Search** - UX improvement
11. **Comments** - Engagement
12. **Activity Feed** - Awareness

### Phase 5 (Future): Advanced
13. **Job Queue** - Background processing
14. **WebSocket** - Real-time features
15. **Analytics** - Insights
16. **Import/Export** - Data management

---

## Architecture Patterns

All new features should follow the established patterns:

### Repository Pattern
```go
type FeatureRepository interface {
    Create(ctx context.Context, entity *Entity) error
    FindByID(ctx context.Context, id uuid.UUID) (*Entity, error)
    FindAll(ctx context.Context, limit, offset int) ([]Entity, int64, error)
    Update(ctx context.Context, entity *Entity) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

### Service Pattern
```go
type FeatureService struct {
    repo     FeatureRepository
    enforcer *permission.Enforcer
    audit    *service.AuditService
}

func (s *FeatureService) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (*Entity, error) {
    // 1. Validate input
    // 2. Check permissions
    // 3. Create entity
    // 4. Log audit
    // 5. Return response
}
```

### Handler Pattern
```go
type Handler struct {
    service  *Service
    enforcer *permission.Enforcer
}

func (h *Handler) Create(c echo.Context) error {
    // 1. Get user context
    // 2. Parse request
    // 3. Validate
    // 4. Call service
    // 5. Map errors
    // 6. Return response
}
```

---

## Notes

1. **Migrations**: All new features require SQL migration files in `migrations/`
2. **Permissions**: New features should define permission resources in `permission:sync`
3. **Audit**: Mutating operations must use `AuditService`
4. **Testing**: Integration tests with testcontainers required
5. **Documentation**: Swagger annotations for all endpoints
6. **Soft Delete**: Most entities should use `gorm.DeletedAt`

---

## Conclusion

The Go API Base provides a solid foundation for building production applications. The recommended features focus on:

1. **Essential business features** - Teams, API keys, email
2. **Security enhancements** - 2FA, audit trail
3. **Developer experience** - Webhooks, feature flags
4. **User engagement** - Notifications, comments, activity

Each feature follows established patterns in the codebase, ensuring consistency and maintainability.