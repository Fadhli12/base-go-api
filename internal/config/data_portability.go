package config

type DataPortabilityConfig struct {
	ExportWorkerConcurrency int `mapstructure:"export_worker_concurrency"`
	ExportRateLimit         int `mapstructure:"export_rate_limit"`
	ImportWorkerConcurrency int `mapstructure:"import_worker_concurrency"`
	ImportRateLimit         int `mapstructure:"import_rate_limit"`
	ImportRateLimitRecords  int `mapstructure:"import_rate_limit_records"`
}

func DefaultDataPortabilityConfig() DataPortabilityConfig {
	return DataPortabilityConfig{
		ExportWorkerConcurrency: 5,
		ExportRateLimit:         10,
		ImportWorkerConcurrency: 5,
		ImportRateLimit:         5,
		ImportRateLimitRecords:  50000,
	}
}