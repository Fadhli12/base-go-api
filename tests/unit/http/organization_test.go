package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationMiddleware_ExtractsOrgID(t *testing.T) {
	e := echo.New()

	// Create test handler to verify org ID was set
	var capturedOrgID uuid.UUID
	var ok bool

	handler := func(c echo.Context) error {
		capturedOrgID, ok = middleware.GetOrganizationID(c)
		return c.String(http.StatusOK, "ok")
	}

	// Apply middleware
	e.Use(middleware.ExtractOrganizationID())
	e.GET("/test", handler)

	// Create request with X-Organization-ID header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Organization-ID", "550e8400-e29b-41d4-a716-446655440000")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify handler was called
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify org ID was extracted and set in context
	assert.True(t, ok, "expected organization ID to be present in context")
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", capturedOrgID.String())
}

func TestOrganizationMiddleware_MissingOrgID_Header(t *testing.T) {
	e := echo.New()

	// Create test handler to verify no org ID was set
	var capturedOrgID uuid.UUID
	var ok bool

	handler := func(c echo.Context) error {
		capturedOrgID, ok = middleware.GetOrganizationID(c)
		return c.String(http.StatusOK, "ok")
	}

	// Apply middleware
	e.Use(middleware.ExtractOrganizationID())
	e.GET("/test", handler)

	// Create request WITHOUT X-Organization-ID header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify handler was called
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify no organization ID was set (ok should be false)
	assert.False(t, ok, "expected organization ID to NOT be present in context when header is missing")
	assert.Equal(t, uuid.Nil, capturedOrgID)
}

func TestOrganizationMiddleware_InvalidOrgID(t *testing.T) {
	e := echo.New()

	// Create test handler (should not be called on error)
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}

	// Apply middleware
	e.Use(middleware.ExtractOrganizationID())
	e.GET("/test", handler)

	// Create request with INVALID X-Organization-ID header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Organization-ID", "not-a-valid-uuid")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Verify error response
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_ORG_ID")
	assert.Contains(t, rec.Body.String(), "Invalid organization ID format")
}

func TestOrganizationMiddleware_ValidOrgID(t *testing.T) {
	e := echo.New()

	// Create test handler to verify org ID
	var capturedOrgID uuid.UUID
	var ok bool

	handler := func(c echo.Context) error {
		capturedOrgID, ok = middleware.GetOrganizationID(c)
		return c.String(http.StatusOK, "ok")
	}

	// Apply middleware
	e.Use(middleware.ExtractOrganizationID())
	e.GET("/test", handler)

	// Test with various valid UUID formats
	validUUIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"123e4567-e89b-12d3-a456-426614174000",
		"00000000-0000-0000-0000-000000000000", // nil UUID is valid
	}

	for _, validUUID := range validUUIDs {
		t.Run("valid UUID: "+validUUID, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Organization-ID", validUUID)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, ok, "expected organization ID to be present for UUID: "+validUUID)
			assert.Equal(t, validUUID, capturedOrgID.String())
		})
	}
}

func TestGetOrganizationID(t *testing.T) {
	t.Run("returns org ID when present", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		c.Set("organization_id", orgID)

		retrievedOrgID, ok := middleware.GetOrganizationID(c)

		assert.True(t, ok)
		assert.Equal(t, orgID, retrievedOrgID)
	})

	t.Run("returns false when not present", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		retrievedOrgID, ok := middleware.GetOrganizationID(c)

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, retrievedOrgID)
	})

	t.Run("returns false when wrong type stored", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Store a string instead of uuid.UUID
		c.Set("organization_id", "not-a-uuid")

		retrievedOrgID, ok := middleware.GetOrganizationID(c)

		assert.False(t, ok)
		assert.Equal(t, uuid.Nil, retrievedOrgID)
	})
}

func TestGetOrganizationIDOrGlobal(t *testing.T) {
	t.Run("returns org ID when present", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		c.Set("organization_id", orgID)

		retrievedOrgID := middleware.GetOrganizationIDOrGlobal(c)

		assert.Equal(t, orgID, retrievedOrgID)
		assert.NotEqual(t, uuid.Nil, retrievedOrgID)
	})

	t.Run("returns nil UUID when not present (global context)", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		retrievedOrgID := middleware.GetOrganizationIDOrGlobal(c)

		assert.Equal(t, uuid.Nil, retrievedOrgID, "expected nil UUID for global context")
	})
}

func TestOrganizationMiddleware_EmptyHeader(t *testing.T) {
	e := echo.New()

	var capturedOrgID uuid.UUID
	var ok bool

	handler := func(c echo.Context) error {
		capturedOrgID, ok = middleware.GetOrganizationID(c)
		return c.String(http.StatusOK, "ok")
	}

	e.Use(middleware.ExtractOrganizationID())
	e.GET("/test", handler)

	// Create request with EMPTY X-Organization-ID header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Organization-ID", "")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should continue without error (backward compatible - empty means no org context)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, ok, "expected organization ID to NOT be present for empty header")
	assert.Equal(t, uuid.Nil, capturedOrgID)
}