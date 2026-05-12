package request

// DashboardQuery represents query parameters for the dashboard endpoint.
type DashboardQuery struct {
	Period string `query:"period" validate:"omitempty,oneof=daily weekly monthly"`
}

// Validate validates the dashboard query parameters.
func (q *DashboardQuery) Validate() error {
	if q.Period == "" {
		q.Period = "daily"
	}
	return validate.Struct(q)
}

// MetricsQuery represents query parameters for the metrics time-series endpoint.
type MetricsQuery struct {
	Type   string `query:"type" validate:"required"`
	Period string `query:"period" validate:"required,oneof=hourly daily weekly monthly"`
	From   string `query:"from" validate:"omitempty,datetime=2006-01-02"`
	To     string `query:"to" validate:"omitempty,datetime=2006-01-02"`
}

// Validate validates the metrics query parameters.
func (q *MetricsQuery) Validate() error {
	return validate.Struct(q)
}

// UpdatePreferencesRequest represents the request body for updating dashboard preferences.
type UpdatePreferencesRequest struct {
	MetricCategories map[string]bool `json:"metric_categories" validate:"required"`
}

// Validate validates the update preferences request.
func (r *UpdatePreferencesRequest) Validate() error {
	return validate.Struct(r)
}