//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/module/invoice"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInvoiceService_Create_ValidData tests creating an invoice with valid data
func TestInvoiceService_Create_ValidData(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	inv, err := invoiceService.Create(ctx, userID, "INV-001", "Acme Corp", 100.50, "Test invoice", nil)
	require.NoError(t, err, "Should create invoice successfully")
	assert.NotEmpty(t, inv.ID, "Invoice ID should be generated")
	assert.Equal(t, userID, inv.UserID, "UserID should match")
	assert.Equal(t, "INV-001", inv.Number, "Number should match")
	assert.Equal(t, "Acme Corp", inv.Customer, "Customer should match")
	assert.Equal(t, 100.50, inv.Amount, "Amount should match")
	assert.Equal(t, invoice.InvoiceStatusDraft, inv.Status, "Status should be draft")
}

// TestInvoiceService_Create_ValidationError tests creating an invoice with invalid data
func TestInvoiceService_Create_ValidationError(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	tests := []struct {
		name        string
		customer    string
		amount      float64
		expectError string
	}{
		{
			name:        "empty customer",
			customer:    "",
			amount:      100.00,
			expectError: "VALIDATION_ERROR",
		},
		{
			name:        "zero amount",
			customer:    "Acme Corp",
			amount:      0,
			expectError: "VALIDATION_ERROR",
		},
		{
			name:        "negative amount",
			customer:    "Acme Corp",
			amount:      -50.00,
			expectError: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := invoiceService.Create(ctx, userID, "INV-TEST", tt.customer, tt.amount, "Test invoice", nil)
			require.Error(t, err, "Should fail validation")
			assert.Equal(t, tt.expectError, apperrors.GetAppError(err).Code, "Error code should match")
		})
	}
}

// TestInvoiceService_GetByID tests retrieving an invoice by ID
func TestInvoiceService_GetByID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create an invoice
	created, err := invoiceService.Create(ctx, userID, "INV-002", "Acme Corp", 200.00, "Test invoice", nil)
	require.NoError(t, err)

	// Get by ID as owner
	found, err := invoiceService.GetByID(ctx, userID, created.ID, false)
	require.NoError(t, err, "Should find invoice by ID")
	assert.Equal(t, created.ID, found.ID, "IDs should match")
	assert.Equal(t, created.Number, found.Number, "Numbers should match")
}

// TestInvoiceService_GetByID_NotFound tests retrieving a non-existent invoice
func TestInvoiceService_GetByID_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	_, err := invoiceService.GetByID(ctx, userID, createTestUUID(), false)
	require.Error(t, err, "Should return error for non-existent invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceService_Update tests updating an invoice
func TestInvoiceService_Update(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create an invoice
	created, err := invoiceService.Create(ctx, userID, "INV-003", "Acme Corp", 300.00, "Original description", nil)
	require.NoError(t, err)

	// Update the invoice
	updated, err := invoiceService.Update(ctx, userID, created.ID, "Beta Corp", 400.00, "Updated description", nil, invoice.InvoiceStatusPending, false)
	require.NoError(t, err, "Should update invoice successfully")
	assert.Equal(t, "Beta Corp", updated.Customer, "Customer should be updated")
	assert.Equal(t, 400.00, updated.Amount, "Amount should be updated")
	assert.Equal(t, "Updated description", updated.Description, "Description should be updated")
	assert.Equal(t, invoice.InvoiceStatusPending, updated.Status, "Status should be updated")
}

// TestInvoiceService_Update_NotFound tests updating a non-existent invoice
func TestInvoiceService_Update_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	_, err := invoiceService.Update(ctx, userID, createTestUUID(), "Beta Corp", 400.00, "Updated", nil, invoice.InvoiceStatusPending, false)
	require.Error(t, err, "Should fail for non-existent invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceService_Delete tests deleting an invoice
func TestInvoiceService_Delete(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create an invoice
	created, err := invoiceService.Create(ctx, userID, "INV-004", "Acme Corp", 500.00, "Test invoice", nil)
	require.NoError(t, err)

	// Delete the invoice
	err = invoiceService.Delete(ctx, userID, created.ID, false)
	require.NoError(t, err, "Should delete invoice successfully")

	// Verify it's soft deleted
	_, err = invoiceRepo.FindByID(ctx, created.ID)
	require.Error(t, err, "Invoice should not be found after soft delete")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceService_Delete_NotFound tests deleting a non-existent invoice
func TestInvoiceService_Delete_NotFound(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	err := invoiceService.Delete(ctx, userID, createTestUUID(), false)
	require.Error(t, err, "Should fail for non-existent invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceService_ListByUser tests listing invoices for a user
func TestInvoiceService_ListByUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	otherUserID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create invoices for user
	_, err := invoiceService.Create(ctx, userID, "INV-005", "Customer A", 100.00, "Invoice 1", nil)
	require.NoError(t, err)
	_, err = invoiceService.Create(ctx, userID, "INV-006", "Customer B", 200.00, "Invoice 2", nil)
	require.NoError(t, err)

	// Create invoice for other user
	_, err = invoiceService.Create(ctx, otherUserID, "INV-007", "Customer C", 300.00, "Other invoice", nil)
	require.NoError(t, err)

	// List invoices for user
	invoices, count, err := invoiceService.ListByUser(ctx, userID, 10, 0)
	require.NoError(t, err, "Should list user invoices")
	assert.Len(t, invoices, 2, "Should have 2 invoices for user")
	assert.Equal(t, int64(2), count, "Count should be 2")

	// Verify other user's invoices not included
	for _, inv := range invoices {
		assert.Equal(t, userID, inv.UserID, "All invoices should belong to user")
	}
}

// TestInvoiceService_ListAll tests listing all invoices (admin)
func TestInvoiceService_ListAll(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	otherUserID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create invoices for multiple users
	_, err := invoiceService.Create(ctx, userID, "INV-008", "Customer A", 100.00, "Invoice 1", nil)
	require.NoError(t, err)
	_, err = invoiceService.Create(ctx, otherUserID, "INV-009", "Customer B", 200.00, "Invoice 2", nil)
	require.NoError(t, err)

	// List all invoices
	invoices, count, err := invoiceService.ListAll(ctx, 10, 0)
	require.NoError(t, err, "Should list all invoices")
	assert.Len(t, invoices, 2, "Should have 2 total invoices")
	assert.Equal(t, int64(2), count, "Count should be 2")
}

// createTestUserInDB creates a test user directly in the database for testing
func createTestUserInDB(t *testing.T, suite *TestSuite) uuid.UUID {
	userID := uuid.New()
	result := suite.DB.Exec(`
		INSERT INTO users (id, email, password_hash) 
		VALUES ($1, $2, 'hash123')
	`, userID, "user-"+userID.String()[:8]+"@example.com")
	require.NoError(t, result.Error, "Should create test user")
	return userID
}

// createTestUUID creates a random UUID for testing
func createTestUUID() uuid.UUID {
	return uuid.New()
}