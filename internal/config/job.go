// Package config provides application configuration management.
package config

// JobConfig holds job queue configuration.
// All fields are configurable via environment variables with mapstructure tags.
type JobConfig struct {
	// WorkerPoolSize is the number of concurrent job workers (default: 5)
	WorkerPoolSize int `mapstructure:"worker_pool_size"`

	// PollerCount is the number of Redis queue pollers (default: 5)
	PollerCount int `mapstructure:"poller_count"`

	// JobTimeout is the max time a job can run before being marked failed (default: 30s)
	JobTimeoutSeconds int `mapstructure:"job_timeout_seconds"`

	// MaxRetries is the default max retry attempts for jobs (default: 3)
	MaxRetries int `mapstructure:"max_retries"`

	// ResultTTLSeconds is how long to keep job results (default: 7 days = 604800)
	ResultTTLSeconds int `mapstructure:"result_ttl_seconds"`

	// StuckJobThresholdSeconds is how long before a processing job is considered stuck (default: 300s = 5min)
	StuckJobThresholdSeconds int `mapstructure:"stuck_job_threshold_seconds"`

	// ReaperIntervalSeconds is how often the reaper runs (default: 60s)
	ReaperIntervalSeconds int `mapstructure:"reaper_interval_seconds"`

	// CallbackTimeoutSeconds is the HTTP timeout for webhook callbacks (default: 5s)
	CallbackTimeoutSeconds int `mapstructure:"callback_timeout_seconds"`

	// QueueKey is the Redis key for the job queue sorted set (default: jobs:queue)
	QueueKey string `mapstructure:"queue_key"`

	// JobDataKeyPrefix is the Redis key prefix for job data hashes (default: jobs:data:)
	JobDataKeyPrefix string `mapstructure:"job_data_key_prefix"`
}

// DefaultJobConfig returns a JobConfig with sensible defaults.
func DefaultJobConfig() JobConfig {
	return JobConfig{
		WorkerPoolSize:            5,
		PollerCount:               5,
		JobTimeoutSeconds:         30,
		MaxRetries:                3,
		ResultTTLSeconds:          604800, // 7 days
		StuckJobThresholdSeconds:  300,    // 5 minutes
		ReaperIntervalSeconds:      60,     // 1 minute
		CallbackTimeoutSeconds:     5,
		QueueKey:                  "jobs:queue",
		JobDataKeyPrefix:          "jobs:data:",
	}
}