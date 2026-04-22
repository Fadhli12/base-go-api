//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/example/go-api-base/internal/module/invoice"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInvoiceScope_UserCanAccessOwnInvoice tests that users can access their own invoices
func TestInvoiceScope_UserCanAccessOwnInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	created, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-001", "Customer A", 100.00, "Test invoice", nil)
	require.NoError(t, err, "Should create invoice")

	// User A can access their own invoice
	found, err := invoiceService.GetByID(ctx, userAID, created.ID, false)
	require.NoError(t, err, "User A should access own invoice")
	assert.Equal(t, created.ID, found.ID, "Should get correct invoice")
}

// TestInvoiceScope_UserCannotAccessOtherUserInvoice tests that regular users cannot access other users' invoices
func TestInvoiceScope_UserCannotAccessOtherUserInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-002", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B tries to access User A's invoice (without admin privileges)
	_, err = invoiceService.GetByID(ctx, userBID, invoiceA.ID, false)
	require.Error(t, err, "User B should not access User A's invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceScope_AdminCanAccessAnyInvoice tests that admins can access any invoice
func TestInvoiceScope_AdminCanAccessAnyInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-003", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B (as admin) can access User A's invoice
	found, err := invoiceService.GetByID(ctx, userBID, invoiceA.ID, true)
	require.NoError(t, err, "Admin should access any invoice")
	assert.Equal(t, invoiceA.ID, found.ID, "Should get correct invoice")
}

// TestInvoiceScope_UserCanUpdateOwnInvoice tests that users can update their own invoices
func TestInvoiceScope_UserCanUpdateOwnInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	created, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-004", "Customer A", 100.00, "Original", nil)
	require.NoError(t, err, "Should create invoice")

	// User A can update their own invoice
	updated, err := invoiceService.Update(ctx, userAID, created.ID, "Customer B", 200.00, "Updated", nil, invoice.InvoiceStatusPending, false)
	require.NoError(t, err, "User A should update own invoice")
	assert.Equal(t, "Customer B", updated.Customer, "Customer should be updated")
	assert.Equal(t, 200.00, updated.Amount, "Amount should be updated")
}

// TestInvoiceScope_UserCannotUpdateOtherUserInvoice tests that regular users cannot update other users' invoices
func TestInvoiceScope_UserCannotUpdateOtherUserInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-005", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B tries to update User A's invoice
	_, err = invoiceService.Update(ctx, userBID, invoiceA.ID, "Customer B", 200.00, "Updated", nil, invoice.InvoiceStatusPending, false)
	require.Error(t, err, "User B should not update User A's invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")
}

// TestInvoiceScope_AdminCanUpdateAnyInvoice tests that admins can update any invoice
func TestInvoiceScope_AdminCanUpdateAnyInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-006", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B (as admin) can update User A's invoice
	updated, err := invoiceService.Update(ctx, userBID, invoiceA.ID, "Customer B", 200.00, "Updated by admin", nil, invoice.InvoiceStatusPending, true)
	require.NoError(t, err, "Admin should update any invoice")
	assert.Equal(t, "Customer B", updated.Customer, "Customer should be updated")
}

// TestInvoiceScope_UserCanDeleteOwnInvoice tests that users can delete their own invoices
func TestInvoiceScope_UserCanDeleteOwnInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	created, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-007", "Customer A", 100.00, "Test invoice", nil)
	require.NoError(t, err, "Should create invoice")

	// User A can delete their own invoice
	err = invoiceService.Delete(ctx, userAID, created.ID, false)
	require.NoError(t, err, "User A should delete own invoice")

	// Verify soft deletion
	_, err = invoiceRepo.FindByID(ctx, created.ID)
	require.Error(t, err, "Invoice should not be found after deletion")
}

// TestInvoiceScope_UserCannotDeleteOtherUserInvoice tests that regular users cannot delete other users' invoices
func TestInvoiceScope_UserCannotDeleteOtherUserInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-008", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B tries to delete User A's invoice
	err = invoiceService.Delete(ctx, userBID, invoiceA.ID, false)
	require.Error(t, err, "User B should not delete User A's invoice")
	assert.True(t, errors.Is(err, apperrors.ErrNotFound), "Should be ErrNotFound")

	// Verify invoice still exists
	found, err := invoiceRepo.FindByID(ctx, invoiceA.ID)
	require.NoError(t, err, "Invoice should still exist")
	assert.Equal(t, invoiceA.ID, found.ID, "Invoice should not be deleted")
}

// TestInvoiceScope_AdminCanDeleteAnyInvoice tests that admins can delete any invoice
func TestInvoiceScope_AdminCanDeleteAnyInvoice(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-009", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice for User A")

	// User B (as admin) can delete User A's invoice
	err = invoiceService.Delete(ctx, userBID, invoiceA.ID, true)
	require.NoError(t, err, "Admin should delete any invoice")

	// Verify soft deletion
	_, err = invoiceRepo.FindByID(ctx, invoiceA.ID)
	require.Error(t, err, "Invoice should not be found after deletion")
}

// TestInvoiceScope_ListByUser tests that ListByUser returns only the user's invoices
func TestInvoiceScope_ListByUser(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create invoices for User A
	_, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-010", "Customer A1", 100.00, "Invoice 1", nil)
	require.NoError(t, err)
	_, err = invoiceService.Create(ctx, userAID, "INV-SCOPE-011", "Customer A2", 200.00, "Invoice 2", nil)
	require.NoError(t, err)

	// Create invoices for User B
	_, err = invoiceService.Create(ctx, userBID, "INV-SCOPE-012", "Customer B1", 300.00, "Invoice 3", nil)
	require.NoError(t, err)
	_, err = invoiceService.Create(ctx, userBID, "INV-SCOPE-013", "Customer B2", 400.00, "Invoice 4", nil)
	require.NoError(t, err)

	// List invoices for User A
	userAInvoices, userACount, err := invoiceService.ListByUser(ctx, userAID, 10, 0)
	require.NoError(t, err, "Should list User A's invoices")
	assert.Len(t, userAInvoices, 2, "User A should have 2 invoices")
	assert.Equal(t, int64(2), userACount, "Count should be 2")

	// Verify all invoices belong to User A
	for _, inv := range userAInvoices {
		assert.Equal(t, userAID, inv.UserID, "All invoices should belong to User A")
	}

	// List invoices for User B
	userBInvoices, userBCount, err := invoiceService.ListByUser(ctx, userBID, 10, 0)
	require.NoError(t, err, "Should list User B's invoices")
	assert.Len(t, userBInvoices, 2, "User B should have 2 invoices")
	assert.Equal(t, int64(2), userBCount, "Count should be 2")

	// Verify all invoices belong to User B
	for _, inv := range userBInvoices {
		assert.Equal(t, userBID, inv.UserID, "All invoices should belong to User B")
	}
}

// TestInvoiceScope_SoftDelete tests that deleted invoices are not returned in ListByUser
func TestInvoiceScope_SoftDelete(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// Create multiple invoices
	inv1, err := invoiceService.Create(ctx, userID, "INV-SCOPE-014", "Customer A", 100.00, "Invoice 1", nil)
	require.NoError(t, err)
	_, err = invoiceService.Create(ctx, userID, "INV-SCOPE-015", "Customer B", 200.00, "Invoice 2", nil)
	require.NoError(t, err)

	// List - should have 2 invoices
	invoices, count, err := invoiceService.ListByUser(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, invoices, 2, "Should have 2 invoices")
	assert.Equal(t, int64(2), count, "Count should be 2")

	// Delete one invoice
	err = invoiceService.Delete(ctx, userID, inv1.ID, false)
	require.NoError(t, err)

	// List - should have 1 invoice (soft delete)
	invoices, count, err = invoiceService.ListByUser(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, invoices, 1, "Should have 1 invoice after deletion")
	assert.Equal(t, int64(1), count, "Count should be 1")
}

// TestInvoiceScope_OwnershipEnforced tests that ownership is enforced in all operations
func TestInvoiceScope_OwnershipEnforced(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	ctx := context.Background()
	userAID := createTestUserInDB(t, suite)
	userBID := createTestUserInDB(t, suite)
	invoiceRepo := invoice.NewRepository(suite.DB)
	invoiceService := invoice.NewService(invoiceRepo, nil)

	// User A creates an invoice
	invoiceA, err := invoiceService.Create(ctx, userAID, "INV-SCOPE-016", "Customer A", 100.00, "User A's invoice", nil)
	require.NoError(t, err, "Should create invoice")

	// User B cannot GET User A's invoice
	_, err = invoiceService.GetByID(ctx, userBID, invoiceA.ID, false)
	require.Error(t, err, "Should fail - User B cannot access User A's invoice")

	// User B cannot UPDATE User A's invoice
	_, err = invoiceService.Update(ctx, userBID, invoiceA.ID, "New Customer", 200, "Updated", nil, invoice.InvoiceStatusPending, false)
	require.Error(t, err, "Should fail - User B cannot update User A's invoice")

	// User B cannot DELETE User A's invoice
	err = invoiceService.Delete(ctx, userBID, invoiceA.ID, false)
	require.Error(t, err, "Should fail - User B cannot delete User A's invoice")

	// User A CAN access/update/delete their own invoice
	_, err = invoiceService.GetByID(ctx, userAID, invoiceA.ID, false)
	require.NoError(t, err, "User A can access own invoice")

	_, err = invoiceService.Update(ctx, userAID, invoiceA.ID, "Updated Customer", 200, "Updated", nil, invoice.InvoiceStatusPending, false)
	require.NoError(t, err, "User A can update own invoice")

	err = invoiceService.Delete(ctx, userAID, invoiceA.ID, false)
	require.NoError(t, err, "User A can delete own invoice")
}