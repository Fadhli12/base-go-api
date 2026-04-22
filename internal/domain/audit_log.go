package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AuditLog represents a record of user actions for compliance and security auditing.
// It captures who performed an action, what action was taken, on which resource,
// and the state before and after the action.
type AuditLog struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ActorID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"actor_id"`
	Action     string         `gorm:"size:20;not null" json:"action"`      // create, update, delete, login, etc.
	Resource   string         `gorm:"size:50;not null;index" json:"resource"` // user, role, permission, invoice
	ResourceID string         `gorm:"size:100;index" json:"resource_id"`  // ID of affected resource
	Before     datatypes.JSON `gorm:"type:jsonb" json:"before"`           // State before (null for create)
	After      datatypes.JSON `gorm:"type:jsonb" json:"after"`            // State after (null for delete)
	IPAddress  string         `gorm:"size:45" json:"ip_address"`          // IPv4 or IPv6
	UserAgent  string         `gorm:"size:500" json:"user_agent"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName returns the table name for the AuditLog model
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditAction constants define standard audit action types
const (
	AuditActionCreate   = "create"
	AuditActionUpdate    = "update"
	AuditActionDelete    = "delete"
	AuditActionLogin     = "login"
	AuditActionLogout    = "logout"
	AuditActionAssign    = "assign"
	AuditActionRevoke    = "revoke"
	AuditActionGrant     = "grant"
	AuditActionDeny      = "deny"
)

// AuditResource constants define standard resource types
const (
	AuditResourceUser       = "user"
	AuditResourceRole       = "role"
	AuditResourcePermission = "permission"
	AuditResourceAuth       = "auth"
)