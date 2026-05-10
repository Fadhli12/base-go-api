package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEntityRegistry(t *testing.T) {
	r := NewEntityRegistry()
	assert.NotNil(t, r)
}

func TestIsExportable_MVPEntities(t *testing.T) {
	r := NewEntityRegistry()
	mvpEntityTypes := []string{
		EntityTypeOrganization,
		EntityTypeRole,
		EntityTypePermission,
		EntityTypeUser,
		EntityTypeOrgMember,
		EntityTypeUserRole,
		EntityTypeUserPermission,
	}
	for _, entityType := range mvpEntityTypes {
		assert.True(t, r.IsExportable(entityType), "expected %s to be exportable", entityType)
	}
}

func TestIsExportable_RestrictedEntities(t *testing.T) {
	r := NewEntityRegistry()
	restricted := []string{
		"api_keys", "two_factor", "audit_logs", "sessions",
		"webhooks", "webhook_deliveries", "import_jobs",
		"export_jobs", "import_id_maps",
	}
	for _, entityType := range restricted {
		assert.False(t, r.IsExportable(entityType), "expected %s to be blocked from export", entityType)
	}
}

func TestIsExportable_UnknownEntity(t *testing.T) {
	r := NewEntityRegistry()
	assert.False(t, r.IsExportable("nonexistent"))
}

func TestIsImportable_MVPEntities(t *testing.T) {
	r := NewEntityRegistry()
	mvpEntityTypes := []string{
		EntityTypeOrganization,
		EntityTypeRole,
		EntityTypePermission,
		EntityTypeUser,
		EntityTypeOrgMember,
		EntityTypeUserRole,
		EntityTypeUserPermission,
	}
	for _, entityType := range mvpEntityTypes {
		assert.True(t, r.IsImportable(entityType), "expected %s to be importable", entityType)
	}
}

func TestIsImportable_RestrictedEntities(t *testing.T) {
	r := NewEntityRegistry()
	restricted := []string{
		"api_keys", "two_factor", "audit_logs", "sessions",
		"webhooks", "webhook_deliveries", "import_jobs",
		"export_jobs", "import_id_maps",
	}
	for _, entityType := range restricted {
		assert.False(t, r.IsImportable(entityType), "expected %s to be blocked from import", entityType)
	}
}

func TestIsImportable_UnknownEntity(t *testing.T) {
	r := NewEntityRegistry()
	assert.False(t, r.IsImportable("nonexistent"))
}

func TestGetImportOrder_TopologicalOrder(t *testing.T) {
	r := NewEntityRegistry()
	order := r.GetImportOrder()
	expected := []string{
		EntityTypeOrganization,
		EntityTypeRole,
		EntityTypePermission,
		EntityTypeUser,
		EntityTypeOrgMember,
		EntityTypeUserRole,
		EntityTypeUserPermission,
	}
	assert.Equal(t, expected, order)
}

func TestGetExportOrder(t *testing.T) {
	r := NewEntityRegistry()
	order := r.GetExportOrder()
	expected := []string{
		EntityTypeOrganization,
		EntityTypeRole,
		EntityTypePermission,
		EntityTypeUser,
		EntityTypeOrgMember,
		EntityTypeUserRole,
		EntityTypeUserPermission,
	}
	assert.Equal(t, expected, order)
}

func TestIsRestricted_AlwaysBlocksRegardlessOfOverride(t *testing.T) {
	r := NewEntityRegistry()
	restricted := []string{
		"api_keys", "two_factor", "audit_logs", "sessions",
		"webhooks", "webhook_deliveries", "import_jobs",
		"export_jobs", "import_id_maps",
	}
	for _, entityType := range restricted {
		assert.True(t, r.IsRestricted(entityType), "expected %s to be restricted", entityType)
		assert.False(t, r.IsExportable(entityType), "restricted %s must never be exportable", entityType)
		assert.False(t, r.IsImportable(entityType), "restricted %s must never be importable", entityType)
	}
}

func TestIsRestricted_NonRestrictedEntity(t *testing.T) {
	r := NewEntityRegistry()
	assert.False(t, r.IsRestricted(EntityTypeUser))
	assert.False(t, r.IsRestricted("nonexistent"))
}