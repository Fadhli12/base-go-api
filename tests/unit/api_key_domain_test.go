package unit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestAPIKey_IsRevoked(t *testing.T) {
	now := time.Now()
	t.Run("not revoked when RevokedAt is nil", func(t *testing.T) {
		key := &domain.APIKey{RevokedAt: nil}
		assert.False(t, key.IsRevoked())
	})
	t.Run("revoked when RevokedAt is set", func(t *testing.T) {
		key := &domain.APIKey{RevokedAt: &now}
		assert.True(t, key.IsRevoked())
	})
}

func TestAPIKey_IsExpired(t *testing.T) {
	t.Run("not expired when ExpiresAt is nil", func(t *testing.T) {
		key := &domain.APIKey{ExpiresAt: nil}
		assert.False(t, key.IsExpired())
	})
	t.Run("expired when ExpiresAt is in the past", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		key := &domain.APIKey{ExpiresAt: &past}
		assert.True(t, key.IsExpired())
	})
	t.Run("not expired when ExpiresAt is in the future", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		key := &domain.APIKey{ExpiresAt: &future}
		assert.False(t, key.IsExpired())
	})
}

func TestAPIKey_IsActive(t *testing.T) {
	t.Run("active when not deleted, not revoked, not expired", func(t *testing.T) {
		key := &domain.APIKey{
			DeletedAt: gorm.DeletedAt{},
			RevokedAt: nil,
			ExpiresAt: nil,
		}
		assert.True(t, key.IsActive())
	})
	t.Run("inactive when soft deleted", func(t *testing.T) {
		key := &domain.APIKey{
			DeletedAt: gorm.DeletedAt{Valid: true},
			RevokedAt: nil,
			ExpiresAt: nil,
		}
		assert.False(t, key.IsActive())
	})
	t.Run("inactive when revoked", func(t *testing.T) {
		now := time.Now()
		key := &domain.APIKey{
			DeletedAt: gorm.DeletedAt{},
			RevokedAt: &now,
			ExpiresAt: nil,
		}
		assert.False(t, key.IsActive())
	})
	t.Run("inactive when expired", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		key := &domain.APIKey{
			DeletedAt: gorm.DeletedAt{},
			RevokedAt: nil,
			ExpiresAt: &past,
		}
		assert.False(t, key.IsActive())
	})
}

func TestAPIKey_HasScope(t *testing.T) {
	makeScopes := func(scopes ...string) datatypes.JSON {
		b, _ := json.Marshal(scopes)
		return b
	}

	t.Run("returns false for nil scopes", func(t *testing.T) {
		key := &domain.APIKey{Scopes: nil}
		assert.False(t, key.HasScope("invoices:view"))
	})
	t.Run("returns true for exact match", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:view", "invoices:create")}
		assert.True(t, key.HasScope("invoices:view"))
	})
	t.Run("returns false for non-matching scope", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:view")}
		assert.False(t, key.HasScope("invoices:delete"))
	})
	t.Run("returns true for wildcard scope", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("*")}
		assert.True(t, key.HasScope("invoices:view"))
		assert.True(t, key.HasScope("users:manage"))
	})
	t.Run("returns true for resource wildcard scope", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:*")}
		assert.True(t, key.HasScope("invoices:view"))
		assert.True(t, key.HasScope("invoices:create"))
	})
	t.Run("returns false for different resource with wildcard action", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:*")}
		assert.False(t, key.HasScope("users:view"))
	})
}

func TestAPIKey_HasAnyScope(t *testing.T) {
	makeScopes := func(scopes ...string) datatypes.JSON {
		b, _ := json.Marshal(scopes)
		return b
	}

	t.Run("returns true when any scope matches", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:view", "users:manage")}
		assert.True(t, key.HasAnyScope([]string{"invoices:view", "invoices:delete"}))
	})
	t.Run("returns false when no scope matches", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:view")}
		assert.False(t, key.HasAnyScope([]string{"users:manage"}))
	})
	t.Run("returns false for empty scopes list to check", func(t *testing.T) {
		key := &domain.APIKey{Scopes: makeScopes("invoices:view")}
		assert.False(t, key.HasAnyScope([]string{}))
	})
}

func TestAPIKey_GetScopes(t *testing.T) {
	t.Run("returns empty for nil scopes", func(t *testing.T) {
		key := &domain.APIKey{Scopes: nil}
		scopes := key.GetScopes()
		assert.Nil(t, scopes)
	})
	t.Run("returns scopes from JSON", func(t *testing.T) {
		b, _ := json.Marshal([]string{"invoices:view", "users:manage"})
		key := &domain.APIKey{Scopes: b}
		scopes := key.GetScopes()
		assert.Equal(t, []string{"invoices:view", "users:manage"}, scopes)
	})
}

func TestAPIKey_IsOwnedBy(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	t.Run("returns true for matching user", func(t *testing.T) {
		key := &domain.APIKey{UserID: userID}
		assert.True(t, key.IsOwnedBy(userID))
	})
	t.Run("returns false for different user", func(t *testing.T) {
		key := &domain.APIKey{UserID: userID}
		assert.False(t, key.IsOwnedBy(otherUserID))
	})
}

func TestAPIKey_ToResponse(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	now := time.Now()
	pastExpiry := now.Add(-1 * time.Hour)
	futureExpiry := now.Add(24 * time.Hour)
	lastUsed := now.Add(-5 * time.Minute)

	t.Run("basic response without expiry or last used", func(t *testing.T) {
		key := &domain.APIKey{
			ID:        id,
			UserID:    userID,
			Name:      "test-key",
			Prefix:    "ak_live_abc1",
			Scopes:    datatypes.JSON(`["invoices:view"]`),
			CreatedAt: now,
			UpdatedAt: now,
		}
		resp := key.ToResponse()
		assert.Equal(t, id.String(), resp.ID)
		assert.Equal(t, userID.String(), resp.UserID)
		assert.Equal(t, "test-key", resp.Name)
		assert.Equal(t, "ak_live_abc1", resp.Prefix)
		assert.Equal(t, []string{"invoices:view"}, resp.Scopes)
		assert.Nil(t, resp.ExpiresAt)
		assert.Nil(t, resp.LastUsedAt)
		assert.False(t, resp.IsRevoked)
		assert.False(t, resp.IsExpired)
		assert.True(t, resp.IsActive)
	})
	t.Run("response with expiry and last used", func(t *testing.T) {
		key := &domain.APIKey{
			ID:         id,
			UserID:     userID,
			Name:       "test-key",
			Prefix:     "ak_live_abc1",
			Scopes:     datatypes.JSON(`["invoices:view"]`),
			ExpiresAt:  &futureExpiry,
			LastUsedAt: &lastUsed,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		resp := key.ToResponse()
		require.NotNil(t, resp.ExpiresAt)
		require.NotNil(t, resp.LastUsedAt)
		assert.False(t, resp.IsExpired)
	})
	t.Run("response for expired key", func(t *testing.T) {
		key := &domain.APIKey{
			ID:        id,
			UserID:    userID,
			Name:      "expired-key",
			Prefix:    "ak_live_abc2",
			Scopes:    datatypes.JSON(`["invoices:view"]`),
			ExpiresAt: &pastExpiry,
			CreatedAt: now,
			UpdatedAt: now,
		}
		resp := key.ToResponse()
		assert.True(t, resp.IsExpired)
		assert.False(t, resp.IsActive)
	})
}

func TestAPIKey_TableName(t *testing.T) {
	t.Run("returns correct table name", func(t *testing.T) {
		key := domain.APIKey{}
		assert.Equal(t, "api_keys", key.TableName())
	})
}

func TestAPIKeyResponse_Fields(t *testing.T) {
	t.Run("response has all expected fields", func(t *testing.T) {
		resp := domain.APIKeyResponse{
			ID:         uuid.New().String(),
			UserID:     uuid.New().String(),
			Name:       "test-key",
			Prefix:     "ak_live_abc1",
			Scopes:     []string{"invoices:view"},
			IsRevoked:  false,
			IsExpired:  false,
			IsActive:   true,
			CreatedAt:  time.Now().Format(time.RFC3339),
			UpdatedAt:  time.Now().Format(time.RFC3339),
		}
		assert.NotEmpty(t, resp.ID)
		assert.NotEmpty(t, resp.UserID)
		assert.Equal(t, "test-key", resp.Name)
		assert.Equal(t, "ak_live_abc1", resp.Prefix)
		assert.Equal(t, []string{"invoices:view"}, resp.Scopes)
		assert.False(t, resp.IsRevoked)
		assert.False(t, resp.IsExpired)
		assert.True(t, resp.IsActive)
	})
}