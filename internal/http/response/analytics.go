package response

// DashboardResponse represents the aggregated metrics for the analytics dashboard.
type DashboardResponse struct {
	Period             string               `json:"period"`
	UserActivity       UserActivityMetrics  `json:"user_activity"`
	ContentMetrics     ContentMetrics       `json:"content_metrics"`
	EngagementMetrics  EngagementMetrics    `json:"engagement_metrics"`
	SystemMetrics      SystemMetrics        `json:"system_metrics"`
}

// UserActivityMetrics represents user-related metrics for a period.
type UserActivityMetrics struct {
	TotalUsers   int64 `json:"total_users"`
	ActiveUsers  int64 `json:"active_users"`
	NewUsers     int64 `json:"new_users"`
	DeletedUsers int64 `json:"deleted_users"`
}

// ContentMetrics represents content-related metrics for a period.
type ContentMetrics struct {
	NewsCreated     int64 `json:"news_created"`
	NewsPublished   int64 `json:"news_published"`
	InvoicesCreated int64 `json:"invoices_created"`
	InvoicesPaid    int64 `json:"invoices_paid"`
}

// EngagementMetrics represents engagement-related metrics for a period.
type EngagementMetrics struct {
	CommentsCreated     int64   `json:"comments_created"`
	WebhookDeliveryRate float64 `json:"webhook_delivery_rate"`
}

// SystemMetrics represents system-related metrics for a period.
type SystemMetrics struct {
	JobsCompleted int64   `json:"jobs_completed"`
	JobsFailed    int64   `json:"jobs_failed"`
	JobSuccessRate float64 `json:"job_success_rate"`
}

// MetricsTimeSeriesResponse represents a time-series of metric data points.
type MetricsTimeSeriesResponse struct {
	Type       string       `json:"type"`
	Period     string       `json:"period"`
	From       string       `json:"from"`
	To         string       `json:"to"`
	DataPoints []DataPoint  `json:"data_points"`
}

// DataPoint represents a single data point in a time-series.
type DataPoint struct {
	Timestamp string `json:"timestamp"`
	Value     int64  `json:"value"`
}

// PreferencesResponse represents dashboard visibility preferences for an organization.
type PreferencesResponse struct {
	OrganizationID   string         `json:"organization_id"`
	MetricCategories map[string]bool `json:"metric_categories"`
}