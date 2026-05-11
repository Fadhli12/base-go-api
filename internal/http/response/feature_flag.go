package response

import "encoding/json"

type FeatureFlagListResponse struct {
	Flags []json.RawMessage `json:"flags"`
	Total int64              `json:"total"`
}

type FeatureFlagEvaluateResponse struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
	Rollout int    `json:"rollout,omitempty"`
}

type BulkEvaluateResponse struct {
	Flags []FeatureFlagEvaluateResponse `json:"flags"`
}