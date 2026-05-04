//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/request"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// createTestUserForWebhook creates a test user directly in the database for webhook tests.
func createTestUserForWebhook(t *testing.T, db *gorm.DB) *domain.User {
	user := &domain.User{
		Email:        fmt.Sprintf("webhook-test-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "$2a$12$testhash",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

// pointer helpers
func ptrInt(i int) *int     { return &i }
func ptrBool(b bool) *bool  { return &b }
func ptrStr(s string) *string { return &s }

// newWebhookService creates a WebhookService with test dependencies.
func newWebhookService(db *gorm.DB) (*service.WebhookService, logger.Logger) {
	webhookRepo := repository.NewWebhookRepository(db)
	deliveryRepo := repository.NewWebhookDeliveryRepository(db)
	cfg := config.DefaultWebhookConfig()
	testLogger := createWebhookTestLogger()
	return service.NewWebhookService(webhookRepo, deliveryRepo, &cfg, testLogger), testLogger
}

// createWebhookTestLogger creates a logger for webhook testing.
func createWebhookTestLogger() logger.Logger {
	return logger.NewNopLogger()
}

// =============================================================================
// CREATE TESTS
// =============================================================================

// TestWebhookService_Create_ValidData tests creating a webhook with valid data.
func TestWebhookService_Create_ValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	webhookSvc, _ := newWebhookService(suite.DB)

	webhook, err := webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Test Webhook",
		URL:    "https://example.com/webhook",
		Events: []string{"user.created", "invoice.paid"},
		Active: nil,
	})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, webhook.ID)
	assert.True(t, strings.HasPrefix(webhook.Secret, "whsec_"))
	assert.Equal(t, "Test Webhook", webhook.Name)
	assert.Equal(t, "https://example.com/webhook", webhook.URL)

	// Verify events stored as JSON
	var storedEvents []string
	err = json.Unmarshal(webhook.Events, &storedEvents)
	require.NoError(t, err)
	assert.Contains(t, storedEvents, "user.created")

	assert.True(t, webhook.Active) // defaults to true
	assert.Equal(t, 100, webhook.RateLimit)

	// ToCreateResponse includes secret
	createResp := webhook.ToCreateResponse()
	assert.NotEmpty(t, createResp.Secret)
	assert.Equal(t, webhook.Secret, createResp.Secret)

	// ToResponse does NOT include secret
	resp := webhook.ToResponse()
	assert.Equal(t, webhook.Name, resp.Name)
}

// TestWebhookService_Create_InvalidURL tests creating webhooks with invalid URLs.
func TestWebhookService_Create_InvalidURL(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc, _ := newWebhookService(suite.DB)

	tests := []struct {
		name    string
		req     *request.CreateWebhookRequest
		wantErr string
	}{
		{
			name: "HTTP URL when not allowed",
			req: &request.CreateWebhookRequest{
				Name:   "HTTP Webhook",
				URL:    "http://example.com/webhook",
				Events: []string{"user.created"},
			},
			wantErr: "URL must use HTTPS",
		},
		{
			name: "Private IP URL",
			req: &request.CreateWebhookRequest{
				Name:   "Private Webhook",
				URL:    "http://127.0.0.1/webhook",
				Events: []string{"user.created"},
			},
			wantErr: "private",
		},
		{
			name: "Empty name",
			req: &request.CreateWebhookRequest{
				Name:   "",
				URL:    "https://example.com/webhook",
				Events: []string{"user.created"},
			},
			wantErr: "name is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := webhookSvc.Create(ctx, nil, tc.req)
			require.Error(t, err)

			var appErr *apperrors.AppError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
			assert.Contains(t, appErr.Message, tc.wantErr)
		})
	}
}

// =============================================================================
// GET BY ID TEST
// =============================================================================

// TestWebhookService_GetByID tests retrieving a webhook by ID.
func TestWebhookService_GetByID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc, _ := newWebhookService(suite.DB)

	webhook, err := webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Get Test",
		URL:    "https://example.com/webhook",
		Events: []string{"user.created"},
	})
	require.NoError(t, err)

	retrieved, err := webhookSvc.GetByID(ctx, webhook.ID)
	require.NoError(t, err)
	assert.Equal(t, webhook.ID, retrieved.ID)
	assert.Equal(t, webhook.Name, retrieved.Name)
	assert.Equal(t, webhook.URL, retrieved.URL)
}

// =============================================================================
// LIST BY ORG TEST
// =============================================================================

// TestWebhookService_ListByOrg tests listing webhooks scoped by organization.
func TestWebhookService_ListByOrg(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()

	// Create an organization for scoping
	user := createTestUserForWebhook(t, suite.DB)
	org := &domain.Organization{Name: "Test Org", Slug: "test-org-" + uuid.New().String()[:8], OwnerID: user.ID}
	require.NoError(t, suite.DB.WithContext(ctx).Create(org).Error)

	// Fetch the created org to get its ID
	var orgs []domain.Organization
	err := suite.DB.WithContext(ctx).Find(&orgs).Error
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	orgID := orgs[0].ID

	webhookSvc, _ := newWebhookService(suite.DB)

	// Create 2 webhooks with orgID
	_, err = webhookSvc.Create(ctx, &orgID, &request.CreateWebhookRequest{
		Name:   "Org Webhook 1",
		URL:    "https://org1.example.com/webhook",
		Events: []string{"user.created"},
	})
	require.NoError(t, err)

	_, err = webhookSvc.Create(ctx, &orgID, &request.CreateWebhookRequest{
		Name:   "Org Webhook 2",
		URL:    "https://org2.example.com/webhook",
		Events: []string{"invoice.paid"},
	})
	require.NoError(t, err)

	// Create 1 global webhook (no org)
	_, err = webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Global Webhook",
		URL:    "https://global.example.com/webhook",
		Events: []string{"news.published"},
	})
	require.NoError(t, err)

	// List with orgID -> should return 2
	list, total, err := webhookSvc.ListByOrg(ctx, &orgID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	// List with nil orgID -> should return 1
	list2, total2, err := webhookSvc.ListByOrg(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, list2, 1)
	assert.Equal(t, "Global Webhook", list2[0].Name)
}

// =============================================================================
// UPDATE TEST
// =============================================================================

// TestWebhookService_Update tests updating a webhook.
func TestWebhookService_Update(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc, _ := newWebhookService(suite.DB)

	webhook, err := webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Original Name",
		URL:    "https://example.com/webhook",
		Events: []string{"user.created"},
	})
	require.NoError(t, err)

	originalSecret := webhook.Secret

	// Update name only
	updated, err := webhookSvc.Update(ctx, webhook.ID, &request.UpdateWebhookRequest{
		Name: ptrStr("Updated Name"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, originalSecret, updated.Secret) // secret should not change

	// Update URL -> should regenerate secret
	updated2, err := webhookSvc.Update(ctx, webhook.ID, &request.UpdateWebhookRequest{
		URL: ptrStr("https://new-url.example.com/webhook"),
	})
	require.NoError(t, err)
	assert.Equal(t, "https://new-url.example.com/webhook", updated2.URL)
	assert.NotEqual(t, originalSecret, updated2.Secret)
	assert.True(t, strings.HasPrefix(updated2.Secret, "whsec_"))
}

// =============================================================================
// SOFT DELETE TEST
// =============================================================================

// TestWebhookService_SoftDelete tests soft deleting a webhook.
func TestWebhookService_SoftDelete(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc, _ := newWebhookService(suite.DB)

	webhook, err := webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Delete Me",
		URL:    "https://delete.example.com/webhook",
		Events: []string{"user.created"},
	})
	require.NoError(t, err)

	err = webhookSvc.SoftDelete(ctx, webhook.ID)
	require.NoError(t, err)

	_, err = webhookSvc.GetByID(ctx, webhook.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

// =============================================================================
// INVALID EVENTS TEST
// =============================================================================

// TestWebhookService_InvalidEvents tests creating a webhook with invalid events.
func TestWebhookService_InvalidEvents(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	webhookSvc, _ := newWebhookService(suite.DB)

	_, err := webhookSvc.Create(ctx, nil, &request.CreateWebhookRequest{
		Name:   "Bad Events",
		URL:    "https://example.com/webhook",
		Events: []string{"foo.bar"},
	})
	require.Error(t, err)

	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "VALIDATION_ERROR", appErr.Code)
	assert.Contains(t, appErr.Message, "invalid webhook events")
}
