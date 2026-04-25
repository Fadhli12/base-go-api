package logger

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds logger configuration from environment variables.
//
// Environment Variables:
//   - LOG_LEVEL: Minimum log level (debug, info, warn, error) - default: info
//   - LOG_FORMAT: Output format (json, text) - default: json
//   - LOG_OUTPUTS: Comma-separated output destinations (stdout, file, syslog) - default: stdout
//   - LOG_FILE_PATH: Log file path - default: /var/log/api.log
//   - LOG_FILE_MAX_SIZE: Max file size in MB before rotation - default: 100
//   - LOG_FILE_MAX_BACKUPS: Max backup files to retain - default: 10
//   - LOG_FILE_MAX_AGE: Max age in days to retain backups - default: 30
//   - LOG_FILE_COMPRESS: Compress rotated files - default: true
//   - LOG_SYSLOG_NETWORK: Network type (tcp, udp) - default: "" (local)
//   - LOG_SYSLOG_ADDRESS: Syslog server address - default: "" (local)
//   - LOG_SYSLOG_TAG: Syslog tag - default: go-api
//   - LOG_ADD_SOURCE: Include source file:line in logs - default: false
//
// Usage:
//
//	config := logger.ConfigFromEnv()
//	log, err := logger.NewLogger(config)
//	if err != nil {
//	    panic(err)
//	}
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string

	// Format is the output format (json, text).
	Format string

	// Outputs is a comma-separated list of output destinations (stdout, file, syslog).
	Outputs string

	// FilePath is the path to the log file when file output is enabled.
	FilePath string

	// FileMaxSize is the maximum size in megabytes before rotation.
	FileMaxSize int

	// FileMaxBackups is the maximum number of backup files to retain.
	FileMaxBackups int

	// FileMaxAge is the maximum age in days to retain backups.
	FileMaxAge int

	// FileCompress determines if rotated files should be gzipped.
	FileCompress bool

	// SyslogNetwork is the network type (tcp, udp) or "" for local.
	SyslogNetwork string

	// SyslogAddress is the syslog server address.
	SyslogAddress string

	// SyslogTag is the tag sent with syslog messages.
	SyslogTag string

	// AddSource includes source file:line in log output.
	AddSource bool
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Level:          "info",
		Format:         "json",
		Outputs:        "stdout",
		FilePath:       "/var/log/api.log",
		FileMaxSize:    100,
		FileMaxBackups: 10,
		FileMaxAge:     30,
		FileCompress:   true,
		SyslogNetwork:  "",
		SyslogAddress:  "",
		SyslogTag:      "go-api",
		AddSource:      false,
	}
}

// ConfigFromEnv creates a Config from environment variables.
// Uses defaults for missing or invalid values.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Level = strings.ToLower(v)
	}

	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Format = strings.ToLower(v)
	}

	if v := os.Getenv("LOG_OUTPUTS"); v != "" {
		cfg.Outputs = v
	}

	if v := os.Getenv("LOG_FILE_PATH"); v != "" {
		cfg.FilePath = v
	}

	if v := os.Getenv("LOG_FILE_MAX_SIZE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.FileMaxSize = i
		}
	}

	if v := os.Getenv("LOG_FILE_MAX_BACKUPS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.FileMaxBackups = i
		}
	}

	if v := os.Getenv("LOG_FILE_MAX_AGE"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			cfg.FileMaxAge = i
		}
	}

	if v := os.Getenv("LOG_FILE_COMPRESS"); v != "" {
		cfg.FileCompress = parseBool(v, true)
	}

	if v := os.Getenv("LOG_SYSLOG_NETWORK"); v != "" {
		cfg.SyslogNetwork = v
	}

	if v := os.Getenv("LOG_SYSLOG_ADDRESS"); v != "" {
		cfg.SyslogAddress = v
	}

	if v := os.Getenv("LOG_SYSLOG_TAG"); v != "" {
		cfg.SyslogTag = v
	}

	if v := os.Getenv("LOG_ADD_SOURCE"); v != "" {
		cfg.AddSource = parseBool(v, false)
	}

	return cfg
}

// Validate checks the configuration for errors.
// Returns error if configuration is invalid.
func (c Config) Validate() error {
	// Validate level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !containsString(validLevels, c.Level) {
		return fmt.Errorf("invalid LOG_LEVEL: %q (must be one of: %s)",
			c.Level, strings.Join(validLevels, ", "))
	}

	// Validate format
	validFormats := []string{"json", "text"}
	if !containsString(validFormats, c.Format) {
		return fmt.Errorf("invalid LOG_FORMAT: %q (must be one of: %s)",
			c.Format, strings.Join(validFormats, ", "))
	}

	// Validate outputs
	validOutputs := []string{"stdout", "file", "syslog"}
	outputs := c.ParseOutputs()
	for _, output := range outputs {
		if !containsString(validOutputs, output) {
			return fmt.Errorf("invalid LOG_OUTPUTS value: %q (must be one of: %s)",
				output, strings.Join(validOutputs, ", "))
			}
	}

	// Validate file path if file output is enabled
	if containsString(outputs, "file") && c.FilePath == "" {
		return fmt.Errorf("LOG_FILE_PATH is required when file output is enabled")
	}

	// Validate syslog settings if syslog output is enabled
	if containsString(outputs, "syslog") {
		if c.SyslogTag == "" {
			return fmt.Errorf("LOG_SYSLOG_TAG is required when syslog output is enabled")
		}
	}

	return nil
}

// ParseLevel converts the level string to Level.
// Returns LevelInfo for unknown values.
func (c Config) ParseLevel() Level {
	return ParseLevel(c.Level)
}

// ParseOutputs splits the outputs string into a slice.
// Handles comma-separated values and trims whitespace.
func (c Config) ParseOutputs() []string {
	if c.Outputs == "" {
		return []string{"stdout"}
	}

	parts := strings.Split(c.Outputs, ",")
	outputs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			outputs = append(outputs, p)
		}
	}

	return outputs
}

// ToFileConfig converts logger config to FileConfig.
func (c Config) ToFileConfig() FileConfig {
	return FileConfig{
		Path:       c.FilePath,
		MaxSize:    c.FileMaxSize,
		MaxBackups: c.FileMaxBackups,
		MaxAge:     c.FileMaxAge,
		Compress:   c.FileCompress,
	}
}

// NewLogger creates a Logger from configuration.
//
// Usage:
//
//	config := logger.ConfigFromEnv()
//	if err := config.Validate(); err != nil {
//	    panic(err)
//	}
//	log, err := logger.NewLogger(config)
//	if err != nil {
//	    panic(err)
//	}
func NewLogger(config Config) (Logger, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create writers based on outputs
	writers, err := createWriters(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create writers: %w", err)
	}

	// Create the appropriate handler based on number of writers
	var writer Writer
	if len(writers) == 0 {
		// Default to stdout if no writers created
		sw, err := NewStdoutWriter(config.Format)
		if err != nil {
			return nil, fmt.Errorf("failed to create default stdout writer: %w", err)
		}
		writer = sw
	} else if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = NewMultiWriter(writers...)
	}

	// Create handler from writer
	handler := NewWriterHandler(writer, config.ParseLevel(), config.AddSource)

	// Create SlogLogger
	logger := NewSlogLogger(handler, config.ParseLevel())

	return logger, nil
}

// NewDefaultLogger creates a Logger with default configuration.
//
// Usage:
//
//	log := logger.NewDefaultLogger()
//	log.Info(ctx, "application started")
func NewDefaultLogger() Logger {
	logger, err := NewLogger(DefaultConfig())
	if err != nil {
		// This should never happen with default config
		panic(fmt.Sprintf("failed to create default logger: %v", err))
	}
	return logger
}

// createWriters creates writers based on configuration outputs.
func createWriters(config Config) ([]Writer, error) {
	outputs := config.ParseOutputs()
	writers := make([]Writer, 0, len(outputs))

	for _, output := range outputs {
		switch output {
		case "stdout":
			w, err := NewStdoutWriter(config.Format)
			if err != nil {
				return nil, fmt.Errorf("failed to create stdout writer: %w", err)
			}
			writers = append(writers, w)

		case "file":
			w, err := NewFileWriter(config.ToFileConfig(), config.Format)
			if err != nil {
				return nil, fmt.Errorf("failed to create file writer: %w", err)
			}
			writers = append(writers, w)

		case "syslog":
			w, err := NewSyslogWriter(config.SyslogNetwork, config.SyslogAddress, config.SyslogTag, config.Format)
			if err != nil {
				return nil, fmt.Errorf("failed to create syslog writer: %w", err)
			}
			writers = append(writers, w)
		}
	}

	return writers, nil
}

// parseBool parses a string as a boolean.
// Returns defaultValue if parsing fails.
func parseBool(s string, defaultValue bool) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
