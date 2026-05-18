package domain

import (
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ======================================================================
// TableName tests
// ======================================================================

func TestIdempotencyRecord_TableName(t *testing.T) {
	record := IdempotencyRecord{}
	assert.Equal(t, "idempotency_keys", record.TableName())
}

// ======================================================================
// ToResponse tests
// ======================================================================

func TestIdempotencyRecord_ToResponse_NilOrgID(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	record := &IdempotencyRecord{
		ID:                 uuid.New(),
		IdempotencyKey:     "req-abc123",
		UserID:              userID,
		OrganizationID:      nil,
		HTTPMethod:          "POST",
		RequestPath:         "/api/v1/invoices",
		Status:              IdempotencyStatusCompleted,
		ResponseStatusCode:  201,
		ResponseBodySize:    42,
		ExpiresAt:           now.Add(24 * time.Hour),
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	resp := record.ToResponse()

	assert.Equal(t, record.ID, resp.ID)
	assert.Equal(t, "req-abc123", resp.IdempotencyKey)
	assert.Equal(t, userID, resp.UserID)
	assert.Nil(t, resp.OrganizationID)
	assert.Equal(t, "POST", resp.HTTPMethod)
	assert.Equal(t, "/api/v1/invoices", resp.RequestPath)
	assert.Equal(t, IdempotencyStatusCompleted, resp.Status)
	assert.Equal(t, 201, resp.ResponseStatusCode)
	assert.Equal(t, 42, resp.ResponseBodySize)
}

func TestIdempotencyRecord_ToResponse_WithOrgID(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	orgID := uuid.New()
	record := &IdempotencyRecord{
		ID:                 uuid.New(),
		IdempotencyKey:     "req-def456",
		UserID:              userID,
		OrganizationID:      &orgID,
		HTTPMethod:          "PUT",
		RequestPath:         "/api/v1/invoices/123",
		Status:              IdempotencyStatusProcessing,
		ResponseStatusCode:  0,
		ResponseBodySize:    0,
		ExpiresAt:           now.Add(1 * time.Hour),
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	resp := record.ToResponse()

	assert.Equal(t, record.ID, resp.ID)
	assert.Equal(t, "req-def456", resp.IdempotencyKey)
	assert.Equal(t, userID, resp.UserID)
	assert.NotNil(t, resp.OrganizationID)
	assert.Equal(t, orgID.String(), *resp.OrganizationID)
	assert.Equal(t, "PUT", resp.HTTPMethod)
	assert.Equal(t, "/api/v1/invoices/123", resp.RequestPath)
	assert.Equal(t, IdempotencyStatusProcessing, resp.Status)
}

// ======================================================================
// IsExpired tests
// ======================================================================

func TestIdempotencyRecord_IsExpired_True(t *testing.T) {
	record := &IdempotencyRecord{
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired 1 hour ago
	}
	assert.True(t, record.IsExpired())
}

func TestIdempotencyRecord_IsExpired_False(t *testing.T) {
	record := &IdempotencyRecord{
		ExpiresAt: time.Now().Add(24 * time.Hour), // expires in 24 hours
	}
	assert.False(t, record.IsExpired())
}

// ======================================================================
// IsProcessing tests
// ======================================================================

func TestIdempotencyRecord_IsProcessing_True(t *testing.T) {
	record := &IdempotencyRecord{
		Status: IdempotencyStatusProcessing,
	}
	assert.True(t, record.IsProcessing())
}

func TestIdempotencyRecord_IsProcessing_False(t *testing.T) {
	record := &IdempotencyRecord{
		Status: IdempotencyStatusCompleted,
	}
	assert.False(t, record.IsProcessing())
}

// ======================================================================
// IsCompleted tests
// ======================================================================

func TestIdempotencyRecord_IsCompleted_True(t *testing.T) {
	record := &IdempotencyRecord{
		Status: IdempotencyStatusCompleted,
	}
	assert.True(t, record.IsCompleted())
}

func TestIdempotencyRecord_IsCompleted_False(t *testing.T) {
	record := &IdempotencyRecord{
		Status: IdempotencyStatusProcessing,
	}
	assert.False(t, record.IsCompleted())
}

// ======================================================================
// Status constants tests
// ======================================================================

func TestIdempotencyStatusConstants(t *testing.T) {
	assert.Equal(t, "processing", IdempotencyStatusProcessing)
	assert.Equal(t, "completed", IdempotencyStatusCompleted)
	assert.Equal(t, "conflict", IdempotencyStatusConflict)
}

// ======================================================================
// Default config tests
// ======================================================================

func TestDefaultIdempotencyConfig(t *testing.T) {
	cfg := config.DefaultIdempotencyConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 128, cfg.MaxKeyLength)
	assert.Equal(t, `^[a-zA-Z0-9_-]+$`, cfg.KeyPattern)
	assert.Equal(t, 24*time.Hour, cfg.DefaultTTL)
	assert.Equal(t, 1*time.Hour, cfg.MinTTL)
	assert.Equal(t, 72*time.Hour, cfg.MaxTTL)
	assert.Equal(t, 5*time.Minute, cfg.GuardTTL)
	assert.Equal(t, 4096, cfg.MaxCachedResponseSize)
	assert.Equal(t, 60*time.Second, cfg.ReaperInterval)
	assert.Equal(t, 30, cfg.RetentionDays)
	assert.Equal(t, 20, cfg.DefaultPageSize)
	assert.Equal(t, 100, cfg.MaxPageSize)
}