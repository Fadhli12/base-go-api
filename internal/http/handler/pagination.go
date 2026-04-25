package handler

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

// Pagination limits to prevent abuse
const (
	// DefaultLimit is the default number of items per page
	DefaultLimit = 20
	// MaxLimit is the maximum allowed limit to prevent excessive queries
	MaxLimit = 100
	// MaxOffset is the maximum allowed offset to prevent deep pagination issues
	MaxOffset = 10000
)

// PaginationParams holds parsed pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// ParsePagination extracts and validates pagination parameters from query params.
// Returns default values if not specified, enforces max limits to prevent abuse.
func ParsePagination(c echo.Context) PaginationParams {
	limit := DefaultLimit
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
			// Enforce max limit
			if limit > MaxLimit {
				limit = MaxLimit
			}
		}
	}

	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := parseIntParam(o); err == nil && parsed >= 0 {
			offset = parsed
			// Enforce max offset to prevent performance issues
			if offset > MaxOffset {
				offset = MaxOffset
			}
		}
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// parseIntParam parses an integer query parameter
func parseIntParam(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}