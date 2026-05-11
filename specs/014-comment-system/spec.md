# Feature Specification: Comment System

**Feature ID:** 014-comment-system
**Priority:** P2
**Complexity:** Medium
**Estimated Effort:** 2 days
**Dependencies:** None (optional: notification system for mention alerts)
**Branch:** 014-comment-system

---

## 1. Overview

Threaded comments on polymorphic entities (news, invoices, etc.) with mentions (@user), soft delete, and audit logging. Enables user engagement through contextual discussions anchored to any resource type.

## 2. Goals

- CRUD for comments with threading (parent_id for replies)
- Polymorphic association: comments attach to any commentable entity (`commentable_type` + `commentable_id`)
- `@mention` parsing and notification trigger (optional, uses existing notification system)
- RBAC enforcement: `comment:create`, `comment:view`, `comment:delete` (own), `comment:delete_any` (admin)
- Audit logging for create/delete operations (Constitution Principle VII)
- Organization-scoped queries (comments belong to org context)
- Soft delete for compliance (Constitution Principle III)
- SQL migrations only — no AutoMigrate (Constitution Principle V)

## 3. Entities

### 3.1 Comment

```go
type Comment struct {
    ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    ParentID         *uuid.UUID     `gorm:"type:uuid;index"`                            // NULL for top-level, UUID for reply
    AuthorID         uuid.UUID      `gorm:"type:uuid;not null;index"`
    OrganizationID   uuid.UUID      `gorm:"type:uuid;not null;index"`
    CommentableType   string         `gorm:"size:50;not null;index"`                     // "news", "invoice", etc.
    CommentableID     uuid.UUID      `gorm:"type:uuid;not null;index"`                   // ID of the parent entity
    Content          string         `gorm:"type:text;not null"`
    MentionedUserIDs  datatypes.JSON `gorm:"type:jsonb"`                                // Parsed @mentions: ["uuid1","uuid2"]
    EditedAt         *time.Time     `gorm:""`
    IsPinned         bool           `gorm:"default:false;not null"`                     // Admin pinning
    CreatedAt        time.Time      `gorm:"autoCreateTime"`
    UpdatedAt        time.Time      `gorm:"autoUpdateTime"`
    DeletedAt        gorm.DeletedAt `gorm:"index"`
}
```

### 3.2 DTOs

```go
type CreateCommentRequest struct {
    Content     string `json:"content" validate:"required,min=1,max=5000"`
    ParentID    string `json:"parent_id"`     // UUID string, optional (for replies)
}

type UpdateCommentRequest struct {
    Content string `json:"content" validate:"required,min=1,max=5000"`
}

type CommentResponse struct {
    ID               uuid.UUID       `json:"id"`
    ParentID         *uuid.UUID       `json:"parent_id,omitempty"`
    AuthorID         uuid.UUID        `json:"author_id"`
    AuthorName       string           `json:"author_name"`
    OrganizationID   uuid.UUID        `json:"organization_id"`
    CommentableType  string           `json:"commentable_type"`
    CommentableID    uuid.UUID        `json:"commentable_id"`
    Content          string           `json:"content"`
    MentionedUserIDs json.RawMessage  `json:"mentioned_user_ids"`
    IsPinned         bool             `json:"is_pinned"`
    EditedAt         *time.Time       `json:"edited_at,omitempty"`
    ReplyCount       int              `json:"reply_count"`
    CreatedAt        time.Time        `json:"created_at"`
    UpdatedAt        time.Time        `json:"updated_at"`
}
```

## 4. Repository Interface

```go
type CommentRepository interface {
    Create(ctx context.Context, comment *domain.Comment) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Comment, error)
    FindByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error)
    FindReplies(ctx context.Context, parentID uuid.UUID, limit, offset int) ([]*domain.Comment, int64, error)
    Update(ctx context.Context, comment *domain.Comment) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    CountByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID) (int64, error)
}
```

## 5. Service

```go
type CommentService interface {
    Create(ctx context.Context, authorID uuid.UUID, commentableType string, commentableID uuid.UUID, req *CreateCommentRequest) (*CommentResponse, error)
    GetByID(ctx context.Context, id uuid.UUID) (*CommentResponse, error)
    ListByCommentable(ctx context.Context, commentableType string, commentableID uuid.UUID, limit, offset int) ([]*CommentResponse, int64, error)
    ListReplies(ctx context.Context, parentID uuid.UUID, limit, offset int) ([]*CommentResponse, int64, error)
    Update(ctx context.Context, id uuid.UUID, authorID uuid.UUID, content string) (*CommentResponse, error)
    Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error  // checks ownership or admin
    Pin(ctx context.Context, id uuid.UUID) (*CommentResponse, error)
    Unpin(ctx context.Context, id uuid.UUID) (*CommentResponse, error)
}
```

### 5.1 Mentions

- On `Create`, parse `@username` patterns from content
- Resolve usernames to user IDs
- Store `mentioned_user_ids` as JSONB array
- **Future enhancement (not in scope):** Trigger notification events for mentioned users

### 5.2 Ownership Check

- User can only `Update` or `Delete` their own comments (unless `comment:delete_any` permission)
- Pinning requires `comment:manage` permission

### 5.3 CommentableType Validation

Service validates `commentableType` against a registered allowlist. Initially:
- `"news"` — News articles
- `"invoice"` — Invoices
- `"media"` — Media files
- Additional types registered via `RegisterCommentableType(type string)`

## 6. Endpoints

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/api/v1/:type/:id/comments` | `comment:create` | Create comment |
| GET | `/api/v1/:type/:id/comments` | `comment:view` | List comments |
| GET | `/api/v1/comments/:id` | `comment:view` | Get comment |
| PUT | `/api/v1/comments/:id` | `comment:create` (own) | Update comment |
| DELETE | `/api/v1/comments/:id` | owner or `comment:delete_any` | Delete comment |
| POST | `/api/v1/comments/:id/pin` | `comment:manage` | Pin comment |
| POST | `/api/v1/comments/:id/unpin` | `comment:manage` | Unpin comment |
| GET | `/api/v1/comments/:id/replies` | `comment:view` | List replies |

**Note:** `:type` in URL is the `commentable_type` (e.g., `/api/v1/news/{id}/comments`).

## 7. Permissions (seeded via `permission:sync`)

| Permission | Resource | Action | Description |
|-----------|----------|--------|-------------|
| `comment:view` | comment | view | View comments |
| `comment:create` | comment | create | Create/edit own comments |
| `comment:delete` | comment | delete | Delete own comments |
| `comment:delete_any` | comment | delete_any | Delete any comment (admin) |
| `comment:manage` | comment | manage | Pin, unpin, manage comments |

## 8. Migrations

- `000022_create_comments.up.sql` — `comments` table with composite index on `(commentable_type, commentable_id)`, foreign key references, `reply_count` function
- `000022_create_comments.down.sql` — Drop table

## 9. Audit Events

| Event | Resource | Before | After |
|-------|----------|--------|-------|
| `create` | `comment` | nil | comment response |
| `update` | `comment` | original content | updated content |
| `delete` | `comment` | comment | nil |
| `pin` | `comment` | unpinned state | pinned state |
| `unpin` | `comment` | pinned state | unpinned state |

## 10. Constraints

- Comment content: 1-5000 characters (validated on create/update)
- ParentID must reference an existing comment on the same commentable entity
- Threading depth: maximum 2 levels (top-level + replies). Replies to replies are flattened to parent.
- `commentableType` must be in registered allowlist (returns 422 if not)
- System prevents deleting comments that have replies (soft-delete the parent, children preserved)
- Audit logging on all CUD operations (Constitution Principle VII)
- Soft delete (Constitution Principle III)
- RBAC via Casbin (Constitution Principle II)
- Organization-scoped: comments are queryable within org context

## 11. Files to Create/Modify

| File | Action |
|------|--------|
| `internal/domain/comment.go` | Create — entities + DTOs + commentable registry |
| `internal/repository/comment.go` | Create — interface + GORM impl |
| `internal/service/comment.go` | Create — business logic, RBAC, audit, mention parsing |
| `internal/http/handler/comment.go` | Create — 8 HTTP endpoints |
| `internal/http/request/comment.go` | Create — request DTOs + validation |
| `internal/http/response/comment.go` | Create — response envelope |
| `internal/http/server.go` | Modify — DI wiring + route registration |
| `cmd/api/main.go` | Modify — permission seeding |
| `migrations/000022_create_comments.up.sql` | Create |
| `migrations/000022_create_comments.down.sql` | Create |
| `tests/unit/comment_service_test.go` | Create — unit tests |
| `tests/integration/comment_handler_test.go` | Create — integration tests |