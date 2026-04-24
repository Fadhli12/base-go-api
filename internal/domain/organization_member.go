package domain

import (
	"time"

	"github.com/google/uuid"
)

// OrganizationMember represents membership in an organization
type OrganizationMember struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organization_id"`
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role           string    `gorm:"size:50;not null;default:'member'" json:"role"`
	JoinedAt       time.Time `gorm:"autoCreateTime" json:"joined_at"`

	// Relations
	User         *User         `gorm:"foreignKey:UserID" json:"-"`
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

// TableName returns the table name for the OrganizationMember model
func (OrganizationMember) TableName() string {
	return "organization_members"
}

// OrganizationMemberResponse represents an organization member response
type OrganizationMemberResponse struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	UserName       string    `json:"user_name,omitempty"`
	UserEmail      string    `json:"user_email,omitempty"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
}

// ToResponse converts an OrganizationMember to OrganizationMemberResponse
func (om *OrganizationMember) ToResponse() *OrganizationMemberResponse {
	resp := &OrganizationMemberResponse{
		OrganizationID: om.OrganizationID,
		UserID:         om.UserID,
		Role:           om.Role,
		JoinedAt:       om.JoinedAt,
	}

	if om.User != nil {
		resp.UserName = om.User.Email // Using email as identifier (or Name if available)
	}

	return resp
}

// IsOwner checks if member is organization owner
func (om *OrganizationMember) IsOwner(orgOwnerID uuid.UUID) bool {
	return om.UserID == orgOwnerID
}

// IsAdmin checks if member has admin or owner role
func (om *OrganizationMember) IsAdmin(orgOwnerID uuid.UUID) bool {
	return om.IsOwner(orgOwnerID) || om.Role == "admin"
}

// OrganizationRole constants
const (
	RoleOwner = "owner"
	RoleAdmin = "admin"
	RoleMember = "member"
)

// IsValidRole checks if role is valid
func IsValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	default:
		return false
	}
}