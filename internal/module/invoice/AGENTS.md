# Invoice Module - Domain Pattern Template

**Location:** internal/module/invoice | **Purpose:** Example domain module pattern

---

## OVERVIEW

Reference implementation showing domain module structure with permission scope checks. Use as template for new feature modules.

## MODULE STRUCTURE

```
internal/module/invoice/
├── entity.go      # Domain entity + response DTO
├── handler.go     # HTTP layer + permission checks
├── service.go     # Business logic + ownership enforcement
└── repository.go  # Data access interface + implementation
```

## ENTITY PATTERN

```go
type Invoice struct {
    ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`  // Owner FK
    // ... business fields ...
    DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`  // Soft delete
}

func (Invoice) TableName() string { return "invoices" }

type InvoiceResponse struct {
    ID     string `json:"id"`      // UUID as string
    UserID string `json:"user_id"`
    // ... public fields ...
}

func (i *Invoice) ToResponse() InvoiceResponse { ... }
```

## PERMISSION SCOPE PATTERN

### Two-Tier Model

**Tier 1: Route Middleware**
```go
// basic permission check
invoices.Use(middleware.RequirePermission(enforcer, "invoices", "view"))
```

**Tier 2: Handler Scope Check**
```go
func (h *Handler) List(c echo.Context) error {
    isAdmin := h.isAdmin(c)  // checks "invoices:manage"
    
    if isAdmin {
        invoices = h.service.ListAll(ctx, limit, offset)
    } else {
        invoices = h.service.ListByUser(ctx, userID, limit, offset)
    }
}
```

### Scope Matrix

| Operation | User Scope | Admin Scope |
|-----------|-----------|-------------|
| List | Own invoices | All invoices |
| GetByID | Own only | Any |
| Update | Own only | Any |
| Delete | Own only | Any |
| Status | Forbidden | Any |

### Ownership Enforcement (Service)

```go
func (s *Service) GetByID(ctx, userID, id uuid.UUID, isAdmin bool) (*Invoice, error) {
    invoice, err := s.repo.FindByID(ctx, id)
    
    // Security: Return 404 (not 403) for owned-by-others
    if !isAdmin && invoice.UserID != userID {
        return nil, apperrors.ErrNotFound
    }
    return invoice, nil
}
```

## HANDLER PATTERN

```go
type Handler struct {
    service  *Service
    enforcer *permission.Enforcer
}

func (h *Handler) GetByID(c echo.Context) error {
    // 1. Auth check
    userID, _ := middleware.GetUserID(c)
    
    // 2. Parse request
    id, _ := uuid.Parse(c.Param("id"))
    
    // 3. Determine scope
    isAdmin := h.isAdmin(c)
    
    // 4. Delegate to service
    invoice, err := h.service.GetByID(ctx, userID, id, isAdmin)
    
    // 5. Map errors
    if appErr := apperrors.GetAppError(err); appErr != nil {
        return c.JSON(appErr.HTTPStatus, response.ErrorWithContext(c, appErr.Code, appErr.Message))
    }
    
    // 6. Return response DTO
    return c.JSON(http.StatusOK, response.SuccessWithContext(c, invoice.ToResponse()))
}

func (h *Handler) isAdmin(c echo.Context) bool {
    claims, _ := middleware.GetUserClaims(c)
    allowed, _ := h.enforcer.Enforce(claims.UserID, "default", "invoices", "manage")
    return allowed
}
```

## SERVICE PATTERN

```go
type Service struct {
    repo     Repository
    enforcer *permission.Enforcer
}

// Context first, user ID for ownership, entity ID for lookup
func (s *Service) Update(ctx, userID, id uuid.UUID, isAdmin bool, req UpdateRequest) (*Invoice, error) {
    invoice, err := s.repo.FindByID(ctx, id)
    if err != nil { return nil, err }
    
    // Ownership enforcement
    if !isAdmin && invoice.UserID != userID {
        return nil, apperrors.ErrNotFound
    }
    
    // Apply changes
    invoice.Customer = req.Customer
    // ...
    
    return s.repo.Update(ctx, invoice)
}
```

## REPOSITORY PATTERN

```go
type Repository interface {
    Create(ctx context.Context, invoice *Invoice) error
    FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
    FindByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Invoice, error)
    FindAll(ctx context.Context, limit, offset int) ([]Invoice, error)
    Update(ctx context.Context, invoice *Invoice) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
    CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
    CountAll(ctx context.Context) (int64, error)
}

type repository struct { db *gorm.DB }

func (r *repository) FindByID(ctx context.Context, id uuid.UUID) (*Invoice, error) {
    var invoice Invoice
    err := r.db.WithContext(ctx).
        Where("id = ? AND deleted_at IS NULL", id).
        First(&invoice).Error
    
    if err == gorm.ErrRecordNotFound {
        return nil, apperrors.ErrNotFound
    }
    return &invoice, apperrors.WrapInternal(err)
}
```

## REQUEST DTOS

```go
// internal/http/request/invoice.go
type CreateInvoiceRequest struct {
    Customer    string     `json:"customer" validate:"required,max=255"`
    Amount      float64    `json:"amount" validate:"required,gt=0"`
    Description string     `json:"description" validate:"omitempty,max=1000"`
    DueDate     *time.Time `json:"due_date"`
}

func (r *CreateInvoiceRequest) Validate() error {
    return validate.Struct(r)
}

type InvoiceListQuery struct {
    Limit  int `query:"limit" validate:"omitempty,min=1,max=100"`
    Offset int `query:"offset" validate:"omitempty,min=0"`
}

func (q *InvoiceListQuery) GetLimit() int {
    if q.Limit <= 0 { return 20 }
    if q.Limit > 100 { return 100 }
    return q.Limit
}
```

## ROUTE REGISTRATION

```go
// internal/http/server.go
func (s *Server) RegisterInvoiceRoutes(api *echo.Group, h *invoice.Handler) {
    invoices := api.Group("/invoices")
    invoices.Use(middleware.JWT(s.config.JWT))
    invoices.Use(middleware.RequirePermission(s.enforcer, "invoices", "view"))
    invoices.Use(middleware.Audit(s.auditSvc))
    
    invoices.POST("", h.Create)
    invoices.GET("", h.List)
    invoices.GET("/:id", h.GetByID)
    invoices.PUT("/:id", h.Update)
    invoices.DELETE("/:id", h.Delete)
}
```

## TEMPLATE CHECKLIST

New module needs:

- [ ] `entity.go` - Domain entity + ToResponse()
- [ ] `repository.go` - Interface + implementation
- [ ] `service.go` - Business logic + ownership checks
- [ ] `handler.go` - HTTP layer + scope determination
- [ ] `internal/http/request/{module}.go` - DTOs
- [ ] Route registration in `server.go`
- [ ] Migration file for table

## IMPLEMENTATION ORDER

1. Entity (data structure)
2. Repository interface (contract)
3. Repository implementation (queries)
4. Service (logic + permissions)
5. Handler (HTTP)
6. Request DTOs
7. Route registration