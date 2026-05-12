package response

import "github.com/example/go-api-base/internal/domain"

// ActivityFollowListResponse wraps a paginated list of follow relationships.
type ActivityFollowListResponse struct {
	Follows []domain.ActivityFollowResponse `json:"follows"`
	Total   int64                           `json:"total"`
}

// UnreadCountResponse represents the number of unread activities.
type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

// MarkAllReadResponse represents the result of marking all activities as read.
type MarkAllReadResponse struct {
	MarkedCount int64 `json:"marked_count"`
}

// MarkReadResponse represents the result of marking a single activity as read.
type MarkReadResponse struct {
	ActivityID string `json:"activity_id"`
	ReadAt      string `json:"read_at"`
}