# Search Request DTOs - Implementation Notes

## Task Completed
Created `internal/http/request/search.go` with:

1. **SearchRequest** - Search query with filters and pagination
   - query (string, optional)
   - filters (map[string]string, optional)
   - page (int, default 1, min 1)
   - page_size (int, default 20, min 1, max 100)
   - sort_by (string, optional)
   - sort_dir (string, optional, oneof=asc desc)

2. **SavedSearchCreateRequest** - Create saved search
   - name (required, max=255)
   - query_text (required)
   - filters (optional)

3. **SavedSearchUpdateRequest** - Update saved search
   - name (optional, max=255)
   - query_text (optional)
   - filters (optional)

## Pattern Followed
- Used go-playground/validator via `validate` package
- All structs have Validate() methods following existing DTO patterns
- No comments/docstrings (project convention from other DTOs like two_factor.go)
- No leading comments on Validate methods (same pattern as two_factor.go)

## Decisions Made
- Used `omitempty` for optional string fields
- Used `omitempty,min=1` for Page to allow defaulting to 1
- Used `omitempty,min=1,max=100` for PageSize to enforce bounds
- Used `omitempty,oneof=asc desc` for SortDir to validate sort direction

## Files Created
- internal/http/request/search.go (36 lines)
