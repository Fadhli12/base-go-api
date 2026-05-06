package response

import "time"

type SearchResponse struct {
	Items     []SearchItem       `json:"items"`
	Total     int64              `json:"total"`
	Page      int                `json:"page"`
	PageSize  int                `json:"page_size"`
	Highlights map[string][]string `json:"highlights,omitempty"`
	Facets    *Facets            `json:"facets,omitempty"`
}

type SearchItem struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Excerpt   string                 `json:"excerpt,omitempty"`
	Status    string                 `json:"status,omitempty"`
	AuthorID  string                 `json:"author_id,omitempty"`
	Rank      float64                `json:"rank,omitempty"`
	CreatedAt *time.Time            `json:"created_at,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Facets struct {
	Status   map[string]int64 `json:"status,omitempty"`
	Type     map[string]int64 `json:"type,omitempty"`
	Author   map[string]int64 `json:"author,omitempty"`
	DateRange map[string]int64 `json:"date_range,omitempty"`
	Custom   map[string]map[string]int64 `json:"custom,omitempty"`
}