// Package config provides application configuration management.
package config

// SwaggerConfig holds Swagger/OpenAPI documentation configuration.
type SwaggerConfig struct {
	Enabled bool   `mapstructure:"enabled"` // enable/disable swagger UI
	Path    string `mapstructure:"path"`    // URL path for swagger UI
}

// DefaultSwaggerConfig returns a SwaggerConfig with sensible defaults.
func DefaultSwaggerConfig() SwaggerConfig {
	return SwaggerConfig{
		Enabled: true,
		Path:    "/swagger",
	}
}