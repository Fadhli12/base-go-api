# Quick Start: Media Library Integration

**Status**: Design Gate  
**Date**: 2026-04-22  
**Project**: Go API Base  
**Phase**: 1 (Design)

---

## Overview

This guide shows how to integrate the Media Library system into your Go API. The system provides polymorphic media management (attach files to any domain model) with permission control, async conversions, and storage abstraction.

By the end, you'll be able to:
- Upload files to any domain model (News, User, Invoice, etc.)
- List, download, and delete media with RBAC enforcement
- Auto-generate thumbnails and conversions
- Query media with pagination and filters

---

## Prerequisites

- Go 1.21+
- Postgres 15+ (migrations support uuid extension)
- Redis 6.0+ (for async conversions and permission caching)
- Docker (for local testing with testcontainers)

---

## Quick Setup (5 Steps)

### Step 1: Run Migrations

The media library requires 3 new tables: `media`, `media_conversions`, `media_downloads`.

```bash
# Create migration files (already in migrations/003_media_library.up.sql)
make migrate

# Verify tables created
psql $DATABASE_URL -c "\dt media*"
```

**Migration includes**:
- `media`: Polymorphic registry (17 columns)
- `media_conversions`: Generated variants (5 columns)
- `media_downloads`: Audit log for downloads (5 columns, optional)
- Indexes on model_type, model_id, collection_name, created_at DESC
- Foreign key from uploaded_by_id to users table

### Step 2: Seed Permissions

The media library uses 4 Casbin permissions: `media:upload`, `media:download`, `media:delete`, `media:view`, `media:admin`.

```bash
# Add permissions to database (not auto-migrated; manual seed)
make seed

# Then sync Casbin cache
go run ./cmd/api permission:sync

# Verify in Redis
redis-cli
> KEYS media:*
1) "media:upload"
2) "media:download"
3) "media:delete"
4) "media:view"
5) "media:admin"
```

**Permission model** (in `internal/permission/casbin/rbac_model.conf`):
```ini
[request_definition]
r = user_id, domain, resource, action

[policy_definition]
p = user_id, domain, resource, action

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r1, p1) && r2 == p2 && r3 == p3 && r4 == p4 || r1 == "admin" && r2 == p2
```

**Example permissions to seed**:
```sql
-- Admin: all media operations
INSERT INTO permissions (role_id, domain, resource, action) 
VALUES ((SELECT id FROM roles WHERE name = 'admin'), 'media', 'media', 'upload');

-- Editor: upload & download own media only
INSERT INTO permissions (role_id, domain, resource, action)
VALUES ((SELECT id FROM roles WHERE name = 'editor'), 'news', 'media', 'upload');

-- Viewer: download media only
INSERT INTO permissions (role_id, domain, resource, action)
VALUES ((SELECT id FROM roles WHERE name = 'viewer'), 'media', 'media', 'download');
```

### Step 3: Configure Storage Driver

Choose your storage backend (local, S3, or MinIO).

**Local filesystem** (development):
```bash
# .env
MEDIA_STORAGE_DRIVER=local
MEDIA_STORAGE_PATH=./storage/media
MEDIA_STORAGE_CLEANUP_INTERVAL=86400  # 24 hours
```

**AWS S3** (production):
```bash
# .env
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_S3_BUCKET=myapp-media-prod
MEDIA_STORAGE_S3_REGION=us-east-1
MEDIA_STORAGE_S3_ENDPOINT=s3.amazonaws.com
AWS_ACCESS_KEY_ID=***
AWS_SECRET_ACCESS_KEY=***
MEDIA_STORAGE_CLEANUP_INTERVAL=86400
```

**MinIO** (self-hosted):
```bash
# .env
MEDIA_STORAGE_DRIVER=s3
MEDIA_STORAGE_S3_BUCKET=myapp-media-dev
MEDIA_STORAGE_S3_ENDPOINT=minio.example.com:9000
MEDIA_STORAGE_S3_DISABLE_SSL=false
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=***
```

### Step 4: Initialize Storage Driver in Bootstrap

In `cmd/api/main.go`, wire up the storage driver:

```go
// Initialize storage driver
var storageDriver storage.Driver
switch os.Getenv("MEDIA_STORAGE_DRIVER") {
case "local":
    storageDriver = storage.NewLocalDriver(
        os.Getenv("MEDIA_STORAGE_PATH"),
    )
case "s3":
    cfg, _ := config.LoadDefaultConfig(ctx)
    storageDriver = storage.NewS3Driver(
        s3.NewFromConfig(cfg),
        os.Getenv("MEDIA_STORAGE_S3_BUCKET"),
    )
default:
    return fmt.Errorf("unknown storage driver: %s", os.Getenv("MEDIA_STORAGE_DRIVER"))
}

// Wire to repositories & services
mediaRepo := repository.NewMediaRepository(db)
conversionHandler := conversion.NewImageConversionHandler(storageDriver)
mediaService := service.NewMediaService(
    mediaRepo,
    storageDriver,
    conversionHandler,
    enforcer,
)
```

### Step 5: Register HTTP Routes

In `internal/http/server.go`, add media endpoints:

```go
// POST /api/v1/models/:model_type/:model_id/media
e.POST("/models/:model_type/:model_id/media", handlers.UploadMedia(mediaService, enforcer))

// GET /api/v1/models/:model_type/:model_id/media
e.GET("/models/:model_type/:model_id/media", handlers.ListMedia(mediaService, enforcer))

// GET /api/v1/media/:media_id
e.GET("/media/:media_id", handlers.GetMedia(mediaService, enforcer))

// GET /api/v1/media/:media_id/download
e.GET("/media/:media_id/download", handlers.DownloadMedia(mediaService, enforcer))

// GET /api/v1/media/:media_id/url
e.GET("/media/:media_id/url", handlers.GetSignedURL(mediaService, enforcer))

// DELETE /api/v1/media/:media_id
e.DELETE("/media/:media_id", handlers.DeleteMedia(mediaService, enforcer))

// Admin endpoints
e.GET("/admin/media", handlers.AdminListMedia(mediaService, enforcer))
e.POST("/admin/media/cleanup", handlers.AdminCleanup(mediaService, enforcer))
e.GET("/admin/media/stats", handlers.AdminStats(mediaService, enforcer))
```

---

## Integration Example: News Model

Let's add media support to the existing News model. This becomes the CRUD base for all other features.

### 1. Update News Domain Entity

In `internal/domain/news.go`:

```go
package domain

import (
    "database/sql/driver"
    "encoding/json"
    "time"

    "github.com/google/uuid"
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

type News struct {
    ID          uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Title       string             `gorm:"type:varchar(255);not null;index"`
    Slug        string             `gorm:"type:varchar(255);not null;uniqueIndex"`
    Content     string             `gorm:"type:text"`
    Metadata    datatypes.JSONMap  `gorm:"type:jsonb"`
    
    // Media association
    MediaCount  int                `gorm:"-" json:"media_count"`
    
    // Timestamps & soft delete
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt     `gorm:"index" json:"-"`
}

func (News) TableName() string {
    return "news"
}

// Response DTO (excludes internal fields)
type NewsResponse struct {
    ID        uuid.UUID       `json:"id"`
    Title     string          `json:"title"`
    Slug      string          `json:"slug"`
    Content   string          `json:"content"`
    Metadata  json.RawMessage `json:"metadata"`
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
}

func (n *News) ToResponse() NewsResponse {
    metadata, _ := json.Marshal(n.Metadata)
    return NewsResponse{
        ID:        n.ID,
        Title:     n.Title,
        Slug:      n.Slug,
        Content:   n.Content,
        Metadata:  metadata,
        CreatedAt: n.CreatedAt,
        UpdatedAt: n.UpdatedAt,
    }
}
```

### 2. Create News Repository

In `internal/repository/news.go`:

```go
package repository

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "gorm.io/gorm"
    "myapp/internal/domain"
    "myapp/pkg/errors"
)

type NewsRepository interface {
    Create(ctx context.Context, news *domain.News) error
    FindByID(ctx context.Context, id uuid.UUID) (*domain.News, error)
    FindBySlug(ctx context.Context, slug string) (*domain.News, error)
    List(ctx context.Context, limit int, offset int) ([]domain.News, int64, error)
    Update(ctx context.Context, news *domain.News) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    HardDelete(ctx context.Context, id uuid.UUID) error
}

type newsRepository struct {
    db *gorm.DB
}

func NewNewsRepository(db *gorm.DB) NewsRepository {
    return &newsRepository{db: db}
}

func (r *newsRepository) Create(ctx context.Context, news *domain.News) error {
    return r.db.WithContext(ctx).Create(news).Error
}

func (r *newsRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.News, error) {
    var news domain.News
    result := r.db.WithContext(ctx).Where("id = ?", id).First(&news)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, errors.ErrNotFound
    }
    return &news, result.Error
}

func (r *newsRepository) FindBySlug(ctx context.Context, slug string) (*domain.News, error) {
    var news domain.News
    result := r.db.WithContext(ctx).Where("slug = ?", slug).First(&news)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, errors.ErrNotFound
    }
    return &news, result.Error
}

func (r *newsRepository) List(ctx context.Context, limit int, offset int) ([]domain.News, int64, error) {
    var newsList []domain.News
    var total int64

    result := r.db.WithContext(ctx).
        Order("created_at DESC").
        Limit(limit).
        Offset(offset).
        Find(&newsList)

    if result.Error != nil {
        return nil, 0, result.Error
    }

    r.db.WithContext(ctx).Model(&domain.News{}).Count(&total)
    return newsList, total, nil
}

func (r *newsRepository) Update(ctx context.Context, news *domain.News) error {
    return r.db.WithContext(ctx).Save(news).Error
}

func (r *newsRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
    return r.db.WithContext(ctx).Delete(&domain.News{}, "id = ?", id).Error
}

func (r *newsRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
    return r.db.WithContext(ctx).Unscoped().Delete(&domain.News{}, "id = ?", id).Error
}
```

### 3. Create News Service

In `internal/service/news.go`:

```go
package service

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "myapp/internal/domain"
    "myapp/internal/permission"
    "myapp/internal/repository"
)

type NewsService interface {
    Create(ctx context.Context, userID uuid.UUID, news *domain.News) error
    GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.News, error)
    GetBySlug(ctx context.Context, slug string) (*domain.News, error)
    List(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]domain.News, int64, error)
    Update(ctx context.Context, userID uuid.UUID, news *domain.News) error
    Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type newsService struct {
    newsRepo repository.NewsRepository
    enforcer *permission.Enforcer
}

func NewNewsService(
    newsRepo repository.NewsRepository,
    enforcer *permission.Enforcer,
) NewsService {
    return &newsService{
        newsRepo: newsRepo,
        enforcer: enforcer,
    }
}

func (s *newsService) Create(ctx context.Context, userID uuid.UUID, news *domain.News) error {
    // Permission check
    if allowed, err := s.enforcer.Enforce(userID.String(), "news", "news", "create"); !allowed || err != nil {
        return fmt.Errorf("permission denied: cannot create news")
    }

    return s.newsRepo.Create(ctx, news)
}

func (s *newsService) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.News, error) {
    // Permission check
    if allowed, err := s.enforcer.Enforce(userID.String(), "news", "news", "view"); !allowed || err != nil {
        return nil, fmt.Errorf("permission denied: cannot view news")
    }

    return s.newsRepo.FindByID(ctx, id)
}

func (s *newsService) GetBySlug(ctx context.Context, slug string) (*domain.News, error) {
    return s.newsRepo.FindBySlug(ctx, slug)
}

func (s *newsService) List(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]domain.News, int64, error) {
    // Permission check
    if allowed, err := s.enforcer.Enforce(userID.String(), "news", "news", "view"); !allowed || err != nil {
        return nil, 0, fmt.Errorf("permission denied: cannot list news")
    }

    return s.newsRepo.List(ctx, limit, offset)
}

func (s *newsService) Update(ctx context.Context, userID uuid.UUID, news *domain.News) error {
    // Permission check
    if allowed, err := s.enforcer.Enforce(userID.String(), "news", "news", "update"); !allowed || err != nil {
        return fmt.Errorf("permission denied: cannot update news")
    }

    return s.newsRepo.Update(ctx, news)
}

func (s *newsService) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
    // Permission check
    if allowed, err := s.enforcer.Enforce(userID.String(), "news", "news", "delete"); !allowed || err != nil {
        return fmt.Errorf("permission denied: cannot delete news")
    }

    return s.newsRepo.SoftDelete(ctx, id)
}
```

### 4. Create News HTTP Handler

In `internal/http/handler/news.go`:

```go
package handler

import (
    "net/http"
    "strconv"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "myapp/internal/domain"
    "myapp/internal/http/response"
    "myapp/internal/service"
    "myapp/pkg/errors"
)

type NewsHandler struct {
    newsService service.NewsService
}

func NewNewsHandler(newsService service.NewsService) *NewsHandler {
    return &NewsHandler{newsService: newsService}
}

// POST /api/v1/news
func (h *NewsHandler) Create(c echo.Context) error {
    var req struct {
        Title   string `json:"title" validate:"required,min=3,max=255"`
        Slug    string `json:"slug" validate:"required,min=3,max=255"`
        Content string `json:"content" validate:"required"`
    }

    if err := c.Bind(&req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }

    userID, _ := uuid.Parse(c.Get("user_id").(string))

    news := &domain.News{
        Title:   req.Title,
        Slug:    req.Slug,
        Content: req.Content,
    }

    if err := h.newsService.Create(c.Request().Context(), userID, news); err != nil {
        if errors.Is(err, errors.ErrPermissionDenied) {
            return response.Forbidden(c, "You do not have permission to create news")
        }
        return response.InternalError(c, "Failed to create news")
    }

    return response.Created(c, news.ToResponse())
}

// GET /api/v1/news/:id
func (h *NewsHandler) GetByID(c echo.Context) error {
    id, _ := uuid.Parse(c.Param("id"))
    userID, _ := uuid.Parse(c.Get("user_id").(string))

    news, err := h.newsService.GetByID(c.Request().Context(), userID, id)
    if err != nil {
        if errors.Is(err, errors.ErrNotFound) {
            return response.NotFound(c, "News not found")
        }
        if errors.Is(err, errors.ErrPermissionDenied) {
            return response.Forbidden(c, "Permission denied")
        }
        return response.InternalError(c, "Failed to fetch news")
    }

    return response.OK(c, news.ToResponse())
}

// GET /api/v1/news
func (h *NewsHandler) List(c echo.Context) error {
    limit, _ := strconv.Atoi(c.QueryParam("limit"))
    if limit == 0 || limit > 100 {
        limit = 20
    }

    offset, _ := strconv.Atoi(c.QueryParam("offset"))

    userID, _ := uuid.Parse(c.Get("user_id").(string))

    newsList, total, err := h.newsService.List(c.Request().Context(), userID, limit, offset)
    if err != nil {
        if errors.Is(err, errors.ErrPermissionDenied) {
            return response.Forbidden(c, "Permission denied")
        }
        return response.InternalError(c, "Failed to list news")
    }

    resp := make([]domain.NewsResponse, len(newsList))
    for i, n := range newsList {
        resp[i] = n.ToResponse()
    }

    return response.OKWithPagination(c, resp, total, limit, offset)
}

// PATCH /api/v1/news/:id
func (h *NewsHandler) Update(c echo.Context) error {
    id, _ := uuid.Parse(c.Param("id"))
    userID, _ := uuid.Parse(c.Get("user_id").(string))

    news, err := h.newsService.GetByID(c.Request().Context(), userID, id)
    if err != nil {
        return response.NotFound(c, "News not found")
    }

    var req struct {
        Title   string `json:"title"`
        Content string `json:"content"`
    }

    if err := c.Bind(&req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }

    if req.Title != "" {
        news.Title = req.Title
    }
    if req.Content != "" {
        news.Content = req.Content
    }

    if err := h.newsService.Update(c.Request().Context(), userID, news); err != nil {
        if errors.Is(err, errors.ErrPermissionDenied) {
            return response.Forbidden(c, "Permission denied")
        }
        return response.InternalError(c, "Failed to update news")
    }

    return response.OK(c, news.ToResponse())
}

// DELETE /api/v1/news/:id
func (h *NewsHandler) Delete(c echo.Context) error {
    id, _ := uuid.Parse(c.Param("id"))
    userID, _ := uuid.Parse(c.Get("user_id").(string))

    if err := h.newsService.Delete(c.Request().Context(), userID, id); err != nil {
        if errors.Is(err, errors.ErrNotFound) {
            return response.NotFound(c, "News not found")
        }
        if errors.Is(err, errors.ErrPermissionDenied) {
            return response.Forbidden(c, "Permission denied")
        }
        return response.InternalError(c, "Failed to delete news")
    }

    return c.NoContent(http.StatusNoContent)
}
```

### 5. Upload Media to News

Once News CRUD is registered, users can attach media:

```bash
# Upload image to news article
curl -X POST http://localhost:8080/api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media \
  -H "Authorization: Bearer <jwt_token>" \
  -F "file=@photo.jpg" \
  -F "collection=featured_image" \
  -F "custom_properties={\"alt_text\": \"Article banner\"}"

# List media for news
curl http://localhost:8080/api/v1/models/news/550e8400-e29b-41d4-a716-446655440000/media \
  -H "Authorization: Bearer <jwt_token>"

# Download media with signed URL
curl "http://localhost:8080/api/v1/media/550e8400.../download?sig=abc&expires=1703341200"
```

---

## Testing Strategy

### Unit Tests

No Docker required; use testify/mock for dependencies:

```bash
go test -v ./tests/unit/service/... -race -coverprofile=coverage.txt
```

Example test:

```go
func TestNewsService_Create(t *testing.T) {
    mockRepo := new(mockNewsRepository)
    mockEnforcer := new(mockEnforcer)

    mockEnforcer.On("Enforce", userID.String(), "news", "news", "create").Return(true, nil)
    mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(n *domain.News) bool {
        return n.Title == "Test News"
    })).Return(nil)

    service := service.NewNewsService(mockRepo, mockEnforcer)
    err := service.Create(ctx, userID, &domain.News{Title: "Test News", Slug: "test-news", Content: "..."})

    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
    mockEnforcer.AssertExpectations(t)
}
```

### Integration Tests

Uses testcontainers-go; requires Docker:

```bash
go test -v -tags=integration ./tests/integration/... -timeout 10m
```

Example test:

```go
//go:build integration

func TestNewsHandler_Upload_Integration(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Close(t)

    suite.RunMigrations(t)
    suite.SetupTest(t)

    // Create test news
    news := &domain.News{Title: "Test", Slug: "test", Content: "..."}
    require.NoError(t, suite.NewsRepo.Create(context.Background(), news))

    // Upload file
    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)
    file, _ := os.Open("testdata/photo.jpg")
    defer file.Close()

    part, _ := writer.CreateFormFile("file", "photo.jpg")
    io.Copy(part, file)
    writer.Close()

    req := httptest.NewRequest("POST", "/api/v1/models/news/"+news.ID.String()+"/media", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("Authorization", "Bearer "+suite.TestToken)

    rec := httptest.NewRecorder()
    suite.Server.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusCreated, rec.Code)
}
```

---

## Configuration Reference

| Variable | Type | Default | Purpose |
|----------|------|---------|---------|
| `MEDIA_STORAGE_DRIVER` | string | `local` | Storage backend: `local`, `s3`, `minio` |
| `MEDIA_STORAGE_PATH` | string | `./storage/media` | Local filesystem path (local driver only) |
| `MEDIA_STORAGE_S3_BUCKET` | string | — | S3 bucket name (s3/minio only) |
| `MEDIA_STORAGE_S3_ENDPOINT` | string | `s3.amazonaws.com` | S3 endpoint (minio: custom host) |
| `MEDIA_STORAGE_S3_REGION` | string | `us-east-1` | AWS region (s3 only) |
| `MEDIA_CLEANUP_INTERVAL` | int | `86400` | Auto-cleanup interval (seconds) |
| `MEDIA_SOFT_DELETE_TTL` | int | `2592000` | Soft-delete grace period (30 days, seconds) |
| `MEDIA_CONVERSION_TIMEOUT` | int | `30` | Conversion timeout (seconds) |
| `MEDIA_UPLOAD_MAX_SIZE` | int | `104857600` | Max upload size (100MB, bytes) |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| 403 Permission Denied on upload | Run `make seed && go run ./cmd/api permission:sync` to sync Casbin |
| Media files not generated (conversions) | Check Redis is running; verify `MEDIA_STORAGE_DRIVER` is set |
| 404 on download | Verify media_id exists; check soft-delete status (`SELECT * FROM media WHERE id = '...'`) |
| Upload timeout (>100MB) | Use multipart upload (v2 feature); current limit is 100MB single-part |
| Storage quota exceeded | Disable quotas in config or increase role quota (`storage_quota_bytes` in roles table) |

---

## Next Steps

1. ✅ Phase 0: Research complete
2. ✅ Phase 1: Design & Contracts complete
3. → Phase 2: Implement core repository + storage abstraction
4. → Phase 3-10: Full implementation per plan.md

**Start Phase 2**:
```bash
git checkout -b 002-media-library-phase-2
# Begin implementing internal/repository/media.go, internal/service/media.go, etc.
```

---

## Support

- **Questions**: Check `docs/media-library/` directory
- **Issues**: `make test-integration` for end-to-end verification
- **Performance**: See `docs/media-library/benchmarks.md` for optimization tips
