//go:build windows

package logger

import (
	"context"
	"encoding/json"
	"fmt"
)

// syslogStub is a no-op implementation for Windows
// since syslog is not available on Windows.
type syslogStub struct{}

func (s *syslogStub) Write(b []byte) (int, error) { return len(b), nil }
func (s *syslogStub) Close() error                 { return nil }
func (s *syslogStub) Debug(m string) error         { return nil }
func (s *syslogStub) Info(m string) error          { return nil }
func (s *syslogStub) Warning(m string) error       { return nil }
func (s *syslogStub) Err(m string) error           { return nil }
func (s *syslogStub) Crit(m string) error          { return nil }

// syslogWriter is the interface satisfied by syslogStub on Windows.
type syslogWriter interface {
	Write([]byte) (int, error)
	Close() error
	Debug(string) error
	Info(string) error
	Warning(string) error
	Err(string) error
	Crit(string) error
}

// newSyslogWriter creates a no-op syslog writer for Windows.
func newSyslogWriter(network, address, tag string) (syslogWriter, error) {
	return &syslogStub{}, nil
}

// SyslogWriter writes log entries to a syslog server.
// On Windows, this is a stub implementation since syslog is not available.
//
// Thread-safe (no-op on Windows).
type SyslogWriter struct {
	writer syslogWriter
	format Format
	closed bool
}

// NewSyslogWriter creates a new SyslogWriter.
// On Windows, this returns a stub that does nothing.
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
	return &SyslogWriter{
		writer: &syslogStub{},
		format: ParseFormat(format),
		closed: false,
	}, nil
}

// Write outputs the log entry to syslog.
// On Windows, this is a no-op.
//
// CONTRACT:
//   - Maps log levels to appropriate syslog severity
//   - Returns error if writer is closed
func (s *SyslogWriter) Write(ctx context.Context, entry LogEntry) error {
	if s.closed {
		return fmt.Errorf("syslog writer is closed")
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
	return nil
}

// Ensure SyslogWriter implements Writer interface.
var _ Writer = (*SyslogWriter)(nil)
