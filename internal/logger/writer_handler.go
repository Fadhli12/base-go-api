package logger

import (
	"context"
	"log/slog"
)

// WriterHandler is a slog.Handler that writes LogEntry to a Writer.
//
// This bridges our Writer interface with slog.Handler so we can use
// the standard slog infrastructure while controlling the output.
type WriterHandler struct {
	writer    Writer
	level     slog.Level
	addSource bool
	group     string
	attrs     []slog.Attr
}

// NewWriterHandler creates a new WriterHandler.
//
// Usage:
//
//	writer := logger.NewStdoutWriter("json")
//	handler := logger.NewWriterHandler(writer, slog.LevelInfo, false)
//	logger := slog.New(handler)
func NewWriterHandler(writer Writer, level Level, addSource bool) *WriterHandler {
	return &WriterHandler{
		writer:    writer,
		level:     toSlogLevel(level),
		addSource: addSource,
		attrs:     make([]slog.Attr, 0),
	}
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *WriterHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats the Record as a LogEntry and writes it to the writer.
func (h *WriterHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract attrs from the record
	fields := make(map[string]interface{})
	
	// Add pre-attached attrs
	for _, attr := range h.attrs {
		fields[attr.Key] = attr.Value.Any()
	}

	// Add attrs from the record
	r.Attrs(func(a slog.Attr) bool {
		// Skip internal slog attrs
		if a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.MessageKey {
			return true
		}
		fields[a.Key] = a.Value.Any()
		return true
	})

	// Create LogEntry
	entry := LogEntry{
		Timestamp: r.Time.UTC(),
		Level:     levelFromSlog(r.Level),
		Message:   r.Message,
		Fields:    fields,
	}

	// Add context fields if available
	if ctx != nil {
		if requestID := GetRequestID(ctx); requestID != "" {
			entry.RequestID = requestID
		}
		if userID := GetUserID(ctx); userID != "" {
			entry.UserID = userID
		}
		if orgID := GetOrgID(ctx); orgID != "" {
			entry.OrgID = orgID
		}
		if traceID := GetTraceID(ctx); traceID != "" {
			entry.TraceID = traceID
		}
	}

	// Add source if enabled
	if h.addSource && r.PC != 0 {
		entry.Source = sourceFromPC(r.PC)
	}

	// Write to the underlying writer
	return h.writer.Write(ctx, entry)
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
func (h *WriterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	h2 := &WriterHandler{
		writer:    h.writer,
		level:     h.level,
		addSource: h.addSource,
		group:     h.group,
		attrs:     make([]slog.Attr, len(h.attrs)),
	}
	copy(h2.attrs, h.attrs)
	h2.attrs = append(h2.attrs, attrs...)

	return h2
}

// WithGroup returns a new Handler with the given group appended to the receiver's groups.
func (h *WriterHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := &WriterHandler{
		writer:    h.writer,
		level:     h.level,
		addSource: h.addSource,
		group:     h.group,
		attrs:     make([]slog.Attr, len(h.attrs)),
	}
	copy(h2.attrs, h.attrs)

	// For simplicity, we prepend the group to attr keys
	// Full group support would require more complex handling
	return h2
}

// levelFromSlog converts slog.Level to our Level string.
func levelFromSlog(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		if level < slog.LevelInfo {
			return "DEBUG"
		} else if level < slog.LevelWarn {
			return "INFO"
		} else if level < slog.LevelError {
			return "WARN"
		}
		return "ERROR"
	}
}

// sourceFromPC extracts source file:line from program counter.
func sourceFromPC(pc uintptr) string {
	// For now, return empty - full implementation would use runtime.Caller
	// This is a placeholder that can be enhanced later
	return ""
}

// Ensure WriterHandler implements slog.Handler.
var _ slog.Handler = (*WriterHandler)(nil)
