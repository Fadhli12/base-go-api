package domain

import (
	"time"

	"github.com/google/uuid"
)

type ImportIDMap struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	JobID      uuid.UUID `gorm:"type:uuid;not null" json:"job_id"`
	EntityType string    `gorm:"type:varchar(50);not null" json:"entity_type"`
	ExternalID uuid.UUID `gorm:"type:uuid;not null" json:"external_id"`
	InternalID uuid.UUID `gorm:"type:uuid;not null" json:"internal_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ImportIDMap) TableName() string { return "import_id_maps" }