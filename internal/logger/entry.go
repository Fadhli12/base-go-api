package logger

import (
	"time"
)

// LogEntry represents a single log entry with all structured fields.
// Used by Writers to output formatted log data.
//
// JSON tags ensure consistent output format across all writers.
type LogEntry struct {
	// Timestamp is when the log was generated.
	Timestamp time.Time `json:"timestamp"`

	// Level is the log level (DEBUG, INFO, WARN, ERROR).
	Level string `json:"level"`

	// Message is the log message.
	Message string `json:"message"`

	// RequestID is the request correlation ID.
	RequestID string `json:"request_id,omitempty"`

	// UserID is the authenticated user ID.
	UserID string `json:"user_id,omitempty"`

	// OrgID is the organization ID for multi-tenant contexts.
	OrgID string `json:"org_id,omitempty"`

	// TraceID is the distributed trace ID.
	TraceID string `json:"trace_id,omitempty"`

	// Fields contains additional structured fields.
	Fields map[string]interface{} `json:"fields,omitempty"`

	// Error contains the error message if an error was logged.
	Error string `json:"error,omitempty"`

	// StackTrace contains the stack trace for error logs (if available).
	StackTrace string `json:"stack_trace,omitempty"`

	// Source contains the source file and line (if AddSource is enabled).
	Source string `json:"source,omitempty"`
}

// NewLogEntry creates a new LogEntry with the given parameters.
// Automatically sets the timestamp to now.
func NewLogEntry(level Level, msg string, fields []Field) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
		Fields:    make(map[string]interface{}, len(fields)),
	}

	for _, f := range fields {
		if f.Key == "error" {
			if err, ok := f.Value.(error); ok && err != nil {
				entry.Error = err.Error()
			} else if s, ok := f.Value.(string); ok {
				entry.Error = s
			} else {
				entry.Error = f.fieldValue()
			}
		} else {
			entry.Fields[f.Key] = f.Value
		}
	}

	return entry
}

// AddContextFields adds context fields to the log entry.
// Used by SlogLogger to inject request_id, user_id, org_id, trace_id.
func (e *LogEntry) AddContextFields(fields []Field) {
	for _, f := range fields {
		switch f.Key {
		case "request_id":
			if s, ok := f.Value.(string); ok {
				e.RequestID = s
			}
		case "user_id":
			if s, ok := f.Value.(string); ok {
				e.UserID = s
			}
		case "org_id":
			if s, ok := f.Value.(string); ok {
				e.OrgID = s
			}
		case "trace_id":
			if s, ok := f.Value.(string); ok {
				e.TraceID = s
			}
		default:
			// Add to Fields map if not already present
			if _, exists := e.Fields[f.Key]; !exists {
				e.Fields[f.Key] = f.Value
			}
		}
	}
}

// SetError sets the error field on the entry.
func (e *LogEntry) SetError(err string) {
	e.Error = err
}

// SetStackTrace sets the stack trace on the entry.
func (e *LogEntry) SetStackTrace(trace string) {
	e.StackTrace = trace
}

// SetSource sets the source location on the entry.
func (e *LogEntry) SetSource(source string) {
	e.Source = source
}
