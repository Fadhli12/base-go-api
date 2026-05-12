package response

import "encoding/json"

type TagListResponse struct {
	Tags  []json.RawMessage `json:"tags"`
	Total int64             `json:"total"`
}

type EntityTagsResponse struct {
	EntityID   string            `json:"entity_id"`
	EntityType string            `json:"entity_type"`
	Tags       []json.RawMessage `json:"tags"`
}

type BulkAttachResult struct {
	Attached []json.RawMessage `json:"attached"`
	Skipped  []json.RawMessage `json:"skipped"`
	Errors   []json.RawMessage `json:"errors,omitempty"`
}

type BulkDetachResult struct {
	Detached []json.RawMessage `json:"detached"`
	Skipped  []json.RawMessage `json:"skipped"`
	Errors   []json.RawMessage `json:"errors,omitempty"`
}

type AutocompleteResponse struct {
	Tags []json.RawMessage `json:"tags"`
}