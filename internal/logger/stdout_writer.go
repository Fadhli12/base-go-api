package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// StdoutWriter writes log entries to stdout.
// Supports both JSON and text formats.
//
// Thread-safe via mutex.
type StdoutWriter struct {
	mu     sync.Mutex
	format Format
	output io.Writer
	closed bool
}

// Format represents the log output format.
type Format string

const (
	// FormatJSON outputs structured JSON.
	FormatJSON Format = "json"
	// FormatText outputs human-readable text.
	FormatText Format = "text"
)

// ParseFormat converts a string to Format.
// Returns FormatJSON for unknown values.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "text", "txt":
		return FormatText
	case "json":
		return FormatJSON
	default:
		return FormatJSON
	}
}

// String returns the string representation of the format.
func (f Format) String() string {
	return string(f)
}

// NewStdoutWriter creates a new StdoutWriter with the specified format.
//
// Supported formats:
//   - "json": Structured JSON output (default)
//   - "text": Human-readable text output
//
// Usage:
//
//	writer, err := logger.NewStdoutWriter("json")
//	if err != nil {
//	    return err
//	}
//	defer writer.Close()
func NewStdoutWriter(format string) (*StdoutWriter, error) {
	f := ParseFormat(format)

	return &StdoutWriter{
		format: f,
		output: os.Stdout,
		closed: false,
	}, nil
}

// Write outputs the log entry to stdout in the configured format.
//
// CONTRACT:
//   - Goroutine-safe via mutex
//   - JSON format: single line JSON with newline
//   - Text format: human-readable with timestamp, level, message
//   - Returns error if writer is closed
func (s *StdoutWriter) Write(ctx context.Context, entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stdout writer is closed")
	}

	switch s.format {
	case FormatText:
		return s.writeText(entry)
	case FormatJSON:
		return s.writeJSON(entry)
	default:
		return s.writeJSON(entry)
	}
}

// writeJSON outputs entry as JSON.
func (s *StdoutWriter) writeJSON(entry LogEntry) error {
	// Use compact JSON encoding
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Write with newline
	if _, err := s.output.Write(data); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	if _, err := s.output.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// writeText outputs entry as human-readable text.
//
// Format: [TIMESTAMP] LEVEL: message key=value key=value ...
func (s *StdoutWriter) writeText(entry LogEntry) error {
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

	// Stack trace (if present and source is not empty)
	if entry.StackTrace != "" {
		parts = append(parts, fmt.Sprintf("stack_trace=%s", entry.StackTrace))
	}

	// Source (if present)
	if entry.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", entry.Source))
	}

	// Join and write
	line := strings.Join(parts, " ") + "\n"
	if _, err := s.output.Write([]byte(line)); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// Close marks the writer as closed.
//
// No-op for StdoutWriter but included for interface compliance.
// Safe to call multiple times.
func (s *StdoutWriter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// Format returns the current format of the writer.
func (s *StdoutWriter) Format() Format {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.format
}

// SetFormat changes the format dynamically.
//
// Usage:
//
//	writer.SetFormat("text")  // Switch to text format
//	writer.SetFormat("json") // Switch back to JSON
func (s *StdoutWriter) SetFormat(format string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.format = ParseFormat(format)
}

// Ensure StdoutWriter implements Writer interface.
var _ Writer = (*StdoutWriter)(nil)
