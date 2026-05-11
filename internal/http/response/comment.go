package response

import "encoding/json"

// CommentListResponse represents a paginated list of comments.
type CommentListResponse struct {
	Comments []json.RawMessage `json:"comments"`
	Total    int64              `json:"total"`
}