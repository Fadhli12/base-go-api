package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileWriter writes log entries to a file with automatic rotation.
// Uses lumberjack.Logger for rotation based on size, age, and backup count.
//
// Thread-safe via lumberjack's internal locking.
type FileWriter struct {
	mu     sync.RWMutex
	config FileConfig
	format Format
	logger *lumberjack.Logger
	closed bool
}

// FileConfig holds configuration for file-based logging.
type FileConfig struct {
	// Path is the full path to the log file.
	// Example: "/var/log/api.log"
	Path string

	// MaxSize is the maximum size in megabytes before rotation.
	// Default: 100
	MaxSize int

	// MaxBackups is the maximum number of old log files to retain.
	// Default: 10
	MaxBackups int

	// MaxAge is the maximum number of days to retain old log files.
	// Default: 30
	MaxAge int

	// Compress determines if rotated files should be gzipped.
	// Default: true
	Compress bool
}

// DefaultFileConfig returns sensible defaults for file logging.
func DefaultFileConfig() FileConfig {
	return FileConfig{
		Path:       "/var/log/api.log",
		MaxSize:    100,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   true,
	}
}

// NewFileWriter creates a new FileWriter with the specified configuration.
//
// The file is opened or created automatically. Parent directories are created
// if they don't exist.
//
// Usage:
//
//	config := logger.FileConfig{
//	    Path:       "/var/log/myapp.log",
//	    MaxSize:    100,  // MB
//	    MaxBackups: 10,
//	    MaxAge:     30,   // days
//	    Compress:   true,
//	}
//	writer, err := logger.NewFileWriter(config, "json")
//	if err != nil {
//	    return err
//	}
//	defer writer.Close()
func NewFileWriter(config FileConfig, format string) (*FileWriter, error) {
	// Apply defaults
	if config.Path == "" {
		config.Path = DefaultFileConfig().Path
	}
	if config.MaxSize <= 0 {
		config.MaxSize = DefaultFileConfig().MaxSize
	}
	if config.MaxBackups <= 0 {
		config.MaxBackups = DefaultFileConfig().MaxBackups
	}
	if config.MaxAge <= 0 {
		config.MaxAge = DefaultFileConfig().MaxAge
	}

	// Create parent directories if needed
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", dir, err)
	}

	// Initialize lumberjack logger
	l := &lumberjack.Logger{
		Filename:   config.Path,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	// Try to open/create the file
	if _, err := l.Write([]byte("")); err != nil {
		return nil, fmt.Errorf("failed to initialize log file %s: %w", config.Path, err)
	}

	return &FileWriter{
		config: FileConfig{
			Path:       config.Path,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		},
		format: ParseFormat(format),
		logger: l,
		closed: false,
	}, nil
}

// Write outputs the log entry to the file in the configured format.
//
// CONTRACT:
//   - Goroutine-safe via lumberjack's internal locking
//   - Automatically rotates files when MaxSize is reached
//   - Returns error if writer is closed
func (f *FileWriter) Write(ctx context.Context, entry LogEntry) error {
	f.mu.RLock()
	if f.closed {
		f.mu.RUnlock()
		return fmt.Errorf("file writer is closed")
	}
	f.mu.RUnlock()

	switch f.format {
	case FormatText:
		return f.writeText(entry)
	case FormatJSON:
		return f.writeJSON(entry)
	default:
		return f.writeJSON(entry)
	}
}

// writeJSON outputs entry as JSON with newline.
func (f *FileWriter) writeJSON(entry LogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Append newline
	data = append(data, '\n')

	if _, err := f.logger.Write(data); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}

// writeText outputs entry as human-readable text.
//
// Format: [TIMESTAMP] LEVEL: message key=value key=value ...
func (f *FileWriter) writeText(entry LogEntry) error {
	var parts []string

	// Timestamp
	parts = append(parts, fmt.Sprintf("[%s]", entry.Timestamp.Format("2006-01-02 15:04:05")))

	// Level
	parts = append(parts, fmt.Sprintf("%s:", entry.Level))

	// Message
	parts = append(parts, entry.Message)

	// Context fields (request_id, user_id, org_id, trace_id)
	if entry.RequestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", entry.RequestID))
	}
	if entry.UserID != "" {
		parts = append(parts, fmt.Sprintf("user_id=%s", entry.UserID))
	}
	if entry.OrgID != "" {
		parts = append(parts, fmt.Sprintf("org_id=%s", entry.OrgID))
	}
	if entry.TraceID != "" {
		parts = append(parts, fmt.Sprintf("trace_id=%s", entry.TraceID))
	}

	// Custom fields
	for key, value := range entry.Fields {
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}

	// Error
	if entry.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", entry.Error))
	}

	// Stack trace (if present)
	if entry.StackTrace != "" {
		parts = append(parts, fmt.Sprintf("stack_trace=%s", entry.StackTrace))
	}

	// Source (if present)
	if entry.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", entry.Source))
	}

	// Join and write with newline
	line := strings.Join(parts, " ") + "\n"
	if _, err := f.logger.Write([]byte(line)); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}

// Close closes the file writer and releases resources.
//
// Flushes any buffered data and closes the underlying file.
// Safe to call multiple times.
func (f *FileWriter) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}

	f.closed = true

	// lumberjack.Logger doesn't have Close(), but we can set it to nil
	// The file is closed automatically when the logger is garbage collected
	// or when the program exits
	return f.logger.Close()
}

// Rotate forces an immediate log rotation.
//
// Usage:
//
//	if err := writer.Rotate(); err != nil {
//	    log.Printf("failed to rotate log: %v", err)
//	}
func (f *FileWriter) Rotate() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return fmt.Errorf("file writer is closed")
	}

	return f.logger.Rotate()
}

// Path returns the current log file path.
func (f *FileWriter) Path() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.config.Path
}

// Config returns a copy of the current configuration.
func (f *FileWriter) Config() FileConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.config
}

// WriteString writes a raw string to the log file.
// Primarily for testing and debugging.
func (f *FileWriter) WriteString(s string) error {
	f.mu.RLock()
	if f.closed {
		f.mu.RUnlock()
		return fmt.Errorf("file writer is closed")
	}
	f.mu.RUnlock()

	_, err := f.logger.Write([]byte(s))
	return err
}

// Ensure FileWriter implements Writer interface.
var _ Writer = (*FileWriter)(nil)
