package service

import (
	"context"
)

const (
	ActionExport = "export"
	ActionImport = "import"
)

type DataPortabilityRateLimiter interface {
	Allow(ctx context.Context, userID string, action string) (bool, error)
	AllowRecords(ctx context.Context, orgID string, count int) (bool, error)
}