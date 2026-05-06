# Search Filter Learnings

## Repository Pattern

- Created `internal/repository/search.go` with two interfaces:
  - `SearchRepository` - for full-text search operations
  - `SavedSearchRepository` - for saved search CRUD operations
- Follows context-first pattern (ctx context.Context as first param)
- Uses uuid.UUID for all IDs
- Errors mapped via `errors.WrapInternal()` and `errors.ErrNotFound`
- Soft delete pattern using GORM's `DeletedAt` (via domain model)

## Structures

### SearchParams
```go
type SearchParams struct {
    Query    string
    Filters  map[string]interface{}
    Page     int
    PageSize int
    SortBy   string
    SortDir  string
}
```

### SearchResult[T]
```go
type SearchResult[T any] struct {
    Items      []T
    Total      int64
    Page       int
    PageSize   int
    Highlights map[string][]string
}
```

## SavedSearchRepository Methods
- `Create(ctx, *domain.SavedSearch) error`
- `FindByUserID(ctx, uuid.UUID) ([]domain.SavedSearch, error)`
- `FindByID(ctx, uuid.UUID) (*domain.SavedSearch, error)`
- `Update(ctx, *domain.SavedSearch) error`
- `SoftDelete(ctx, uuid.UUID) error`

## Notes
- Domain model `SavedSearch` uses JSONB for Filters field
- Interface only (no implementation) - actual implementation would use GORM soft delete scopes

## SearchService Implementation

Created `internal/service/search.go` with:
- `SearchService` struct: `db *gorm.DB`, `savedSearchRepo`, `log *slog.Logger`
- `NewSearchService` constructor
- `Search()` method: full-text search with PostgreSQL tsvector/tsquery
  - Truncates query at 500 chars
  - Uses `websearch_to_tsquery()` internally via `buildTSQuery()` which strips non-word chars and appends `:*` for prefix matching
  - Filters: status, author_id, date_from, date_to via `appendFilter()` helper
  - Always excludes soft-deleted rows (`n.deleted_at IS NULL`)
  - Sorting: `ts_rank_cd()` for relevance, or `title`, `created_at`, `updated_at`
  - Highlighting: `ts_headline()` with MaxWords=50, MinWords=20, MaxFragments=3
  - Highlights extracted to `SearchResult.Highlights` map and stripped from rows
  - Structured logging with user_id, query, result_count, page, page_size
- SavedSearch CRUD methods: `CreateSavedSearch`, `ListSavedSearches`, `GetSavedSearch`, `UpdateSavedSearch`, `DeleteSavedSearch`
  - Owner enforcement: compares userID against ss.UserID
  - Max 50 saved searches per user
  - JSONB filters via `domain.NewJSONB()`

### Missing Interface Fix
`SavedSearchRepository` interface was absent from repository/search.go — only the private struct existed. Added the interface definition.
## Search Response DTO Pattern (2026-05-06)

### Created: internal/http/response/search.go

**Structures:**
- `SearchResponse`: items ([]SearchItem), total (int64), page (int), page_size (int), highlights (map[string][]string), facets (*Facets)
- `SearchItem`: id, title, excerpt, status, author_id, rank (float64), created_at (*time.Time), metadata (map[string]interface{})
- `Facets`: status, type, author, date_range, custom - all map[string]int64 for counts

**Pattern followed from envelope.go:**
- No comments on exported types (per project convention)
- JSON struct tags with omitempty for optional fields
- Pointer types for optional fields (CreatedAt, Facets)

**Key decisions:**
- Highlights as map[string][]string (field name → matched snippets)
- Facets uses nested maps for custom aggregations
- Metadata as map[string]interface{} for flexibility

## SearchHandler Implementation (2026-05-06)

### Created: internal/http/handler/search.go

**Endpoints implemented:**
- `GET /api/v1/search` - Full-text search on news articles
- `GET /api/v1/saved-searches` - List user's saved searches
- `POST /api/v1/saved-searches` - Create saved search
- `GET /api/v1/saved-searches/:id` - Get saved search by ID
- `PUT /api/v1/saved-searches/:id` - Update saved search
- `DELETE /api/v1/saved-searches/:id` - Soft delete saved search

**Handler pattern followed:**
- Uses `middleware.GetUserID(c)` to extract authenticated user from JWT
- Uses `middleware.GetLogger(c)` for structured logging
- Uses `response.SuccessWithContext(c, data)` for JSON responses
- Uses `req.Validate()` for request validation
- Uses `apperrors.GetAppError(err)` for error code mapping
- Ownership validation is done in the service layer (GetSavedSearch, UpdateSavedSearch, DeleteSavedSearch all check userID)

**Key imports:**
- `domain` package for `SavedSearchResponse`
- `response` package for `SearchResponse` and `SearchItem`
- `request` package for `SearchRequest`, `SavedSearchCreateRequest`, `SavedSearchUpdateRequest`
- `strconv` for parsing pagination parameters

**Helper functions:**
- `parsePositiveInt(s string) (int, error)` - Parse and validate positive integer from string
- `getStringValue(m map[string]interface{}, key string) string` - Safely extract string from map
- `extractMetadata(m map[string]interface{}) map[string]interface{}` - Extract unknown fields as metadata

### Registered routes in server.go:

**New repository:** `savedSearchRepo := repository.NewSavedSearchRepository(s.db)`
**New service:** `searchService := service.NewSearchService(s.db, savedSearchRepo, slog.Default())`
**New handler:** `searchHandler := handler.NewSearchHandler(searchService)`
**New method:** `RegisterSearchRoutes(v1, searchHandler)`

**Route registration pattern:**
```go
func (s *Server) RegisterSearchRoutes(api *echo.Group, searchHandler *handler.SearchHandler) {
    search := api.Group("/search")
    search.Use(middleware.JWT(...))
    search.GET("", searchHandler.Search)
    search.GET("/saved-searches", searchHandler.ListSavedSearches)
    search.POST("/saved-searches", searchHandler.CreateSavedSearch)
    search.GET("/saved-searches/:id", searchHandler.GetSavedSearch)
    search.PUT("/saved-searches/:id", searchHandler.UpdateSavedSearch)
    search.DELETE("/saved-searches/:id", searchHandler.DeleteSavedSearch)
}
```

**Note:** Search endpoint is `/api/v1/search` (via GET /search) not `/api/v1/saved-searches` which is for saved searches. This matches the task requirements.

## Wire-up in cmd/api/main.go (2026-05-06)

**Added to DefaultPermissions():**
```go
{Name: "search:read", Description: "Perform searches across resources", Resource: "search", Action: "read"},
{Name: "saved_search:create", Description: "Create saved searches", Resource: "saved_search", Action: "create"},
{Name: "saved_search:read", Description: "View saved searches", Resource: "saved_search", Action: "read"},
{Name: "saved_search:update", Description: "Update saved searches", Resource: "saved_search", Action: "update"},
{Name: "saved_search:delete", Description: "Delete saved searches", Resource: "saved_search", Action: "delete"},
```

**Admin role updated to include:** `"search:read", "saved_search:create", "saved_search:read", "saved_search:update", "saved_search:delete"`

**Note:** SearchService, SearchHandler, and RegisterSearchRoutes were already wired in `internal/http/server.go` (lines 374, 404, 481, 623). The wire-up in main.go was only for adding permissions to DefaultPermissions() function.
