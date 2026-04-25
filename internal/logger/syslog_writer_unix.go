//go:build !windows

package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/syslog"
)

// syslogWriter is the interface satisfied by *syslog.Writer.
type syslogWriter interface {
	Write([]byte) (int, error)
	Close() error
	Debug(string) error
	Info(string) error
	Warning(string) error
	Err(string) error
	Crit(string) error
}

// newSyslogWriter creates a syslog writer appropriate for the platform.
func newSyslogWriter(network, address, tag string) (syslogWriter, error) {
	// Try to dial remote syslog
	if network != "" || address != "" {
		w, err := syslog.Dial(network, address, syslog.LOG_INFO, tag)
		if err == nil {
			return w, nil
		}
	}

	// Fall back to local syslog
	return syslog.New(syslog.LOG_INFO, tag)
}

// SyslogWriter writes log entries to a syslog server.
//
// Thread-safe via syslog.Writer's internal locking.
type SyslogWriter struct {
	writer syslogWriter
	format Format
	closed bool
}

// NewSyslogWriter creates a new SyslogWriter.
//
// Parameters:
//   - network: "tcp", "udp", or "" for local syslog
//   - address: server address (e.g., "localhost:514") or "" for local
//   - tag: syslog tag (usually application name)
//   - format: "json" or "text"
//
// Usage:
//
//	writer, err := logger.NewSyslogWriter("tcp", "logs.example.com:514", "myapp", "json")
//	if err != nil {
//	    return err
//	}
//	defer writer.Close()
func NewSyslogWriter(network, address, tag, format string) (*SyslogWriter, error) {
	// Connect to syslog (platform-specific)
	w, err := newSyslogWriter(network, address, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog: %w", err)
	}

	return &SyslogWriter{
		writer: w,
		format: ParseFormat(format),
		closed: false,
	}, nil
}

// Write outputs the log entry to syslog.
//
// CONTRACT:
//   - Maps log levels to appropriate syslog severity
//   - Returns error if writer is closed
func (s *SyslogWriter) Write(ctx context.Context, entry LogEntry) error {
	if s.closed {
		return fmt.Errorf("syslog writer is closed")
	}

	// Format the message
	var msg string
	var err error
	switch s.format {
	case FormatText:
		msg = s.formatText(entry)
	case FormatJSON:
		msg, err = s.formatJSON(entry)
		if err != nil {
			return err
		}
	default:
		msg, err = s.formatJSON(entry)
		if err != nil {
			return err
		}
	}

	// Write with appropriate severity
	switch entry.Level {
	case "DEBUG":
		err = s.writer.Debug(msg)
	case "INFO":
		err = s.writer.Info(msg)
	case "WARN":
		err = s.writer.Warning(msg)
	case "ERROR":
		err = s.writer.Err(msg)
	default:
		err = s.writer.Info(msg)
	}

	if err != nil {
		return fmt.Errorf("failed to write to syslog: %w", err)
	}

	return nil
}

// formatJSON returns JSON representation of the entry.
func (s *SyslogWriter) formatJSON(entry LogEntry) (string, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entry: %w", err)
	}
	return string(data), nil
}

// formatText returns text representation of the entry.
func (s *SyslogWriter) formatText(entry LogEntry) string {
	return fmt.Sprintf("[%s] %s: %s", entry.Level, entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Message)
}

// Close closes the syslog connection.
//
// Safe to call multiple times.
func (s *SyslogWriter) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	return s.writer.Close()
}

// Ensure SyslogWriter implements Writer interface.
var _ Writer = (*SyslogWriter)(nil)
