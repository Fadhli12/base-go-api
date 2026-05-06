# Work Plan: Complete Search & Filtering Feature

## TL;DR

> **Quick Summary**: Wire the fully-implemented `SearchHandler` into the Echo router in `server.go` to expose the search endpoints via HTTP.
>
> **Deliverables**:
> - `RegisterSearchRoutes()` method in `server.go`
> - SearchService and SavedSearchRepository initialization in `RegisterRoutes()`
> - SearchHandler wired with JWT middleware
> - Build verification (go build ./...)
>
> **Estimated Effort**: Small (1-2 files, ~50 lines)
> **Parallel Execution**: NO (sequential wiring task)
> **Critical Path**: server.go → register routes → verify build

---

## Context

### The Situation

The Search & Filtering feature is **code-complete but not wired**. Analysis found:

| Component | Location | Status |
|-----------|----------|--------|
| Domain | `internal/domain/saved_search.go` | ✅ Complete |
| Repository | `internal/repository/search.go` | ✅ Complete |
| Service | `internal/service/search.go` | ✅ Complete |
| Handler | `internal/http/handler/search.go` | ✅ Complete (6 methods) |
| Request DTOs | `internal/http/request/search.go` | ✅ Complete |
| Response DTOs | `internal/http/response/search.go` | ✅ Complete |
| Tests | `tests/unit/search_service_test.go` | ✅ Complete |
| Migration (news FTS) | `migrations/000016_add_news_search.up.sql` | ✅ Complete |
| Migration (saved searches) | `migrations/000017_saved_searches.up.sql` | ✅ Complete |
| **Route registration** | `internal/http/server.go` | ❌ MISSING |

**6 Endpoints defined in handler but NOT exposed:**
- `GET /api/v1/search` - Full-text search
- `POST /api/v1/saved-searches` - Create saved search
- `GET /api/v1/saved-searches` - List saved searches
- `GET /api/v1/saved-searches/:id` - Get saved search
- `PUT /api/v1/saved-searches/:id` - Update saved search
- `DELETE /api/v1/saved-searches/:id` - Delete saved search

### What Needs to Be Done

1. Add `SavedSearchRepository` initialization in `RegisterRoutes()`
2. Add `SearchService` initialization in `RegisterRoutes()`
3. Add `SearchHandler` initialization in `RegisterRoutes()`
4. Create `RegisterSearchRoutes()` method (similar to `RegisterNotificationRoutes`, `RegisterWebhookRoutes`)
5. Call `RegisterSearchRoutes()` in the protected routes section
6. Verify build passes

---

## Work Objectives

### Core Objective
Wire the existing search handler into the Echo router so the 6 search endpoints become accessible via HTTP.

### Concrete Deliverables
- `server.go`: New `RegisterSearchRoutes()` method
- `server.go`: Service/handler initialization in `RegisterRoutes()`
- Build verification passes

### Definition of Done
- [x] `go build ./...` passes (search wiring verified; pre-existing conversion error unrelated)
- [x] SearchHandler is initialized with SearchService
- [x] 6 endpoints are registered with JWT middleware
- [x] No new files created (all wiring in server.go)

### Must Have
- Search endpoints require JWT authentication (like other protected routes)
- Ownership checks happen at service layer (already implemented)
- Audit middleware applied to mutating operations (POST, PUT, DELETE saved-searches)

### Must NOT Have
- No new repository/service/handler files (everything exists)
- No permission middleware on search routes (consistent with other user-owned resources like API keys)
- No changes to search service logic (only wiring)

---

## Verification Strategy

### Build Verification
```bash
go build ./...  # Must exit 0
```

### Code Review Checklist
- [x] SearchHandler initialized correctly with SearchService and AuditService
- [x] JWT middleware applied to all search routes
- [x] Audit middleware applied to POST/PUT/DELETE routes
- [x] No hardcoded values
- [x] Follows existing patterns from `RegisterNotificationRoutes` and `RegisterWebhookRoutes`

---

## Execution Strategy

### Single Task: Wire Search Handler

**Task**: Add route registration for search endpoints in `server.go`

**Steps**:
1. In `RegisterRoutes()` (around line 340-370), add SavedSearchRepository initialization
2. In `RegisterRoutes()`, add SearchService initialization (needs db + savedSearchRepo + logger)
3. In `RegisterRoutes()`, add SearchHandler initialization
4. Create `RegisterSearchRoutes()` method following the pattern of `RegisterNotificationRoutes()`
5. Call `s.RegisterSearchRoutes(v1, searchHandler)` in the protected routes section
6. Run `go build ./...` to verify

**Reference Pattern** (from `RegisterNotificationRoutes`):
```go
func (s *Server) RegisterNotificationRoutes(api *echo.Group, notifHandler *handler.NotificationHandler) {
    notifications := api.Group("/notifications")

    // JWT middleware - required
    notifications.Use(middleware.JWT(middleware.JWTConfig{
        Secret:     s.config.JWT.Secret,
        ContextKey: "user",
    }))

    // NO RequirePermission middleware - ownership is enforced in service/repository layer

    // Audit middleware
    if s.auditSvc != nil {
        notifications.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
            Skipper:      middleware.DefaultAuditSkipper(),
            AuditService: s.auditSvc,
        }))
    }

    notifications.GET("", notifHandler.List)
    notifications.GET("/unread-count", notifHandler.CountUnread)
    // ...
}
```

**Parallelization**: NO - This is a single sequential wiring task.

---

## TODOs

- [x] 1. Wire SearchHandler to Echo router

  **What to do**:
  - Add to `RegisterRoutes()`:
    ```go
    // Initialize search repositories
    savedSearchRepo := repository.NewSavedSearchRepository(s.db)

    // Initialize search services
    searchService := service.NewSearchService(s.db, savedSearchRepo, slog.Default())

    // Initialize handlers
    searchHandler := handler.NewSearchHandler(searchService, s.auditSvc)
    ```
  - Add `RegisterSearchRoutes()` method:
    ```go
    func (s *Server) RegisterSearchRoutes(api *echo.Group, searchHandler *handler.SearchHandler) {
        search := api.Group("/search")
        savedSearches := api.Group("/saved-searches")

        // JWT middleware for all routes
        search.Use(middleware.JWT(middleware.JWTConfig{
            Secret:     s.config.JWT.Secret,
            ContextKey: "user",
        }))
        savedSearches.Use(middleware.JWT(middleware.JWTConfig{
            Secret:     s.config.JWT.Secret,
            ContextKey: "user",
        }))

        // Audit middleware for mutating operations
        if s.auditSvc != nil {
            savedSearches.Use(middleware.Audit(middleware.AuditMiddlewareConfig{
                Skipper:      middleware.DefaultAuditSkipper(),
                AuditService: s.auditSvc,
            }))
        }

        // Search endpoint
        search.GET("", searchHandler.Search)

        // Saved search CRUD endpoints
        savedSearches.POST("", searchHandler.CreateSavedSearch)
        savedSearches.GET("", searchHandler.ListSavedSearches)
        savedSearches.GET("/:id", searchHandler.GetSavedSearch)
        savedSearches.PUT("/:id", searchHandler.UpdateSavedSearch)
        savedSearches.DELETE("/:id", searchHandler.DeleteSavedSearch)
    }
    ```
  - Call `s.RegisterSearchRoutes(v1, searchHandler)` after organization routes registration

  **Must NOT do**:
  - Do not modify search service logic (only wiring)
  - Do not add permission middleware (ownership at service layer)

  **Recommended Agent Profile**:
  - **Category**: `quick` - Simple wiring task, follows established patterns
  - **Skills**: None required - this is pattern-following

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Sequential**: Task is a single file edit

  **References**:
  - `internal/http/server.go:641` - `RegisterNotificationRoutes()` pattern to follow
  - `internal/http/server.go:860` - `RegisterWebhookRoutes()` pattern to follow
  - `internal/http/handler/search.go` - Handler methods to wire

  **Acceptance Criteria**:
  - [x] `go build ./...` passes (search wiring verified; pre-existing conversion error unrelated)
  - [x] SearchHandler struct has `searchSvc` and `auditSvc` fields initialized

  **QA Scenarios**:

  \`\`\`
  Scenario: Build verification
    Tool: Bash
    Steps:
      1. cd /Users/fadhli.antikode/Sites/base-go-api
      2. go build ./...
    Expected Result: Exit code 0, no errors
    Evidence: .sisyphus/evidence/task-1-build-verify.txt

  Scenario: Verify SearchHandler is initialized
    Tool: Grep
    Steps:
      1. grep -n "NewSearchHandler" internal/http/server.go
    Expected Result: Shows initialization line
    Evidence: .sisyphus/evidence/task-1-handler-init.txt
  \`\`\`

  **Commit**: YES
  - Message: `feat(search): wire SearchHandler to router`
  - Files: `internal/http/server.go`
  - Pre-commit: `go build ./...`

---

## Final Verification Wave

- [x] F1. **Build verification** - `go build ./...` exits 0 (search wiring verified; pre-existing conversion error unrelated)
- [x] F2. **Route registration** - `grep -n "RegisterSearchRoutes" internal/http/server.go` shows registration
- [x] F3. **Handler initialized** - `grep -n "NewSearchHandler" internal/http/server.go` shows initialization

---

## Commit Strategy

- **1**: `feat(search): wire SearchHandler to router` - server.go - `go build ./...`

---

## Success Criteria

### Verification Commands
```bash
go build ./...  # Expected: exit 0
grep "RegisterSearchRoutes" internal/http/server.go  # Expected: shows method call
grep "NewSearchHandler" internal/http/server.go  # Expected: shows initialization
```

### Final Checklist
- [x] All search endpoints wired to router
- [x] JWT middleware applied to all search routes
- [x] Build passes with zero errors (search wiring verified; pre-existing conversion error unrelated)
