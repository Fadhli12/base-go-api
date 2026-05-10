package domain

type Exportable interface {
	ToExportRecord() map[string]interface{}
}

type Importable interface {
	FromImportRecord(data map[string]interface{}) error
}

const (
	EntityTypeOrganization   = "organizations"
	EntityTypeRole           = "roles"
	EntityTypePermission     = "permissions"
	EntityTypeUser           = "users"
	EntityTypeOrgMember      = "org_members"
	EntityTypeUserRole       = "user_roles"
	EntityTypeUserPermission = "user_permissions"
)

type ConflictStrategy string

const (
	ConflictSkip      ConflictStrategy = "skip"
	ConflictOverwrite ConflictStrategy = "overwrite"
	ConflictFail      ConflictStrategy = "fail"
)

const (
	ExportQueued     = "queued"
	ExportProcessing = "processing"
	ExportCompleted  = "completed"
	ExportFailed     = "failed"
)

const (
	ImportQueued     = "queued"
	ImportValidating = "validating"
	ImportProcessing = "processing"
	ImportCompleted  = "completed"
	ImportFailed     = "failed"
	ImportCancelled  = "cancelled"
)

// ImportOrder is the topological sort order for importing entities.
// Dependencies must be imported before dependent entities.
var ImportOrder = []string{
	EntityTypeOrganization,
	EntityTypeRole,
	EntityTypePermission,
	EntityTypeUser,
	EntityTypeOrgMember,
	EntityTypeUserRole,
	EntityTypeUserPermission,
}

var ExportOrder = []string{
	EntityTypeOrganization,
	EntityTypeRole,
	EntityTypePermission,
	EntityTypeUser,
	EntityTypeOrgMember,
	EntityTypeUserRole,
	EntityTypeUserPermission,
}

// restrictedEntities are ALWAYS blocked from export/import regardless of Casbin policy (MAC).
var restrictedEntities = map[string]bool{
	"api_keys":           true,
	"two_factor":         true,
	"audit_logs":         true,
	"sessions":           true,
	"webhooks":           true,
	"webhook_deliveries": true,
	"import_jobs":        true,
	"export_jobs":        true,
	"import_id_maps":     true,
}

var mvpEntities = map[string]bool{
	EntityTypeOrganization:   true,
	EntityTypeRole:           true,
	EntityTypePermission:     true,
	EntityTypeUser:           true,
	EntityTypeOrgMember:      true,
	EntityTypeUserRole:       true,
	EntityTypeUserPermission: true,
}

type EntityRegistry struct {
	exportable map[string]bool
	importable map[string]bool
}

func NewEntityRegistry() *EntityRegistry {
	r := &EntityRegistry{
		exportable: make(map[string]bool),
		importable: make(map[string]bool),
	}
	for _, entityType := range ImportOrder {
		r.exportable[entityType] = true
		r.importable[entityType] = true
	}
	return r
}

func (r *EntityRegistry) IsExportable(entityType string) bool {
	if restrictedEntities[entityType] {
		return false
	}
	return r.exportable[entityType]
}

func (r *EntityRegistry) IsImportable(entityType string) bool {
	if restrictedEntities[entityType] {
		return false
	}
	return r.importable[entityType]
}

func (r *EntityRegistry) GetExportOrder() []string {
	result := make([]string, 0, len(ExportOrder))
	for _, entityType := range ExportOrder {
		if r.exportable[entityType] {
			result = append(result, entityType)
		}
	}
	return result
}

func (r *EntityRegistry) GetImportOrder() []string {
	result := make([]string, 0, len(ImportOrder))
	for _, entityType := range ImportOrder {
		if r.importable[entityType] {
			result = append(result, entityType)
		}
	}
	return result
}

func (r *EntityRegistry) IsRestricted(entityType string) bool {
	return restrictedEntities[entityType]
}