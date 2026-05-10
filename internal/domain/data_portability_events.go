package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventExportCreated  = "export.created"
	EventExportCompleted = "export.completed"
	EventExportFailed    = "export.failed"
	EventImportCreated   = "import.created"
	EventImportCompleted = "import.completed"
	EventImportFailed    = "import.failed"
)

type DataPortabilityEvent struct {
	Type         string
	JobID        uuid.UUID
	OrgID        *uuid.UUID
	EntityTypes  []string
	Status       string
	Timestamp    time.Time
}

func NewExportCreatedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventExportCreated,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "created",
		Timestamp:   time.Now(),
	}
}

func NewExportCompletedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventExportCompleted,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "completed",
		Timestamp:   time.Now(),
	}
}

func NewExportFailedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventExportFailed,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "failed",
		Timestamp:   time.Now(),
	}
}

func NewImportCreatedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventImportCreated,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "created",
		Timestamp:   time.Now(),
	}
}

func NewImportCompletedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventImportCompleted,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "completed",
		Timestamp:   time.Now(),
	}
}

func NewImportFailedEvent(jobID uuid.UUID, orgID *uuid.UUID, entityTypes []string) *DataPortabilityEvent {
	return &DataPortabilityEvent{
		Type:        EventImportFailed,
		JobID:       jobID,
		OrgID:       orgID,
		EntityTypes: entityTypes,
		Status:      "failed",
		Timestamp:   time.Now(),
	}
}

func (e *DataPortabilityEvent) ToWebhookEvent() WebhookEvent {
	return WebhookEvent{
		Type:    e.Type,
		Payload: e,
		OrgID:   e.OrgID,
	}
}