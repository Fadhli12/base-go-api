package request

type SearchRequest struct {
	Query    string            `json:"query" validate:"omitempty"`
	Filters  map[string]string `json:"filters" validate:"omitempty"`
	Page     int               `json:"page" validate:"omitempty,min=1"`
	PageSize int              `json:"page_size" validate:"omitempty,min=1,max=100"`
	SortBy   string            `json:"sort_by" validate:"omitempty"`
	SortDir  string            `json:"sort_dir" validate:"omitempty,oneof=asc desc"`
}

type SavedSearchCreateRequest struct {
	Name      string            `json:"name" validate:"required,max=255"`
	QueryText string            `json:"query_text" validate:"required"`
	Filters   map[string]string `json:"filters" validate:"omitempty"`
}

type SavedSearchUpdateRequest struct {
	Name      string            `json:"name" validate:"omitempty,max=255"`
	QueryText string            `json:"query_text" validate:"omitempty"`
	Filters   map[string]string `json:"filters" validate:"omitempty"`
}

func (r *SearchRequest) Validate() error {
	return validate.Struct(r)
}

func (r *SavedSearchCreateRequest) Validate() error {
	return validate.Struct(r)
}

func (r *SavedSearchUpdateRequest) Validate() error {
	return validate.Struct(r)
}