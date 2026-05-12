package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// DashboardMetric represents a pre-aggregated metric stored for dashboard display.
type DashboardMetric struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MetricType     string         `gorm:"size:50;not null"`
	PeriodType     string         `gorm:"size:20;not null;check:period_type IN ('daily','weekly','monthly')"`
	PeriodStart    time.Time      `gorm:"not null"`
	PeriodEnd      time.Time      `gorm:"not null"`
	Value          float64        `gorm:"not null"`
	Metadata       datatypes.JSON `gorm:"type:jsonb"`
	OrganizationID *uuid.UUID    `gorm:"type:uuid"`
	CalculatedAt   time.Time      `gorm:"not null"`
	CreatedAt      time.Time      `gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime"`
}

// TableName returns the table name for the DashboardMetric model.
func (DashboardMetric) TableName() string {
	return "dashboard_metrics"
}

// ToResponse converts DashboardMetric to DashboardMetricResponse.
func (d *DashboardMetric) ToResponse() DashboardMetricResponse {
	var metadata json.RawMessage
	if len(d.Metadata) > 0 {
		metadata = json.RawMessage(d.Metadata)
	}

	var orgIDStr *string
	if d.OrganizationID != nil {
		s := d.OrganizationID.String()
		orgIDStr = &s
	}

	return DashboardMetricResponse{
		ID:             d.ID.String(),
		MetricType:     d.MetricType,
		PeriodType:     d.PeriodType,
		PeriodStart:    d.PeriodStart.Format(time.RFC3339),
		PeriodEnd:      d.PeriodEnd.Format(time.RFC3339),
		Value:          d.Value,
		Metadata:       metadata,
		OrganizationID: orgIDStr,
		CalculatedAt:   d.CalculatedAt.Format(time.RFC3339),
		CreatedAt:      d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      d.UpdatedAt.Format(time.RFC3339),
	}
}

// DashboardMetricResponse represents a dashboard metric in API responses.
type DashboardMetricResponse struct {
	ID             string          `json:"id"`
	MetricType     string          `json:"metric_type"`
	PeriodType     string          `json:"period_type"`
	PeriodStart    string          `json:"period_start"`
	PeriodEnd      string          `json:"period_end"`
	Value          float64         `json:"value"`
	Metadata       json.RawMessage `json:"metadata"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	CalculatedAt   string          `json:"calculated_at"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}