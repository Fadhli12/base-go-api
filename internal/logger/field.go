package logger

import (
	"fmt"
	"log/slog"
	"time"
)

// Field represents a structured logging field.
//
// CONTRACT:
//   - Immutable once created
//   - Key must be non-empty (enforced by implementations)
//   - Value must be JSON-serializable
//   - Safe for concurrent use
//   - Stack-allocated (not heap) for performance
//
// Usage:
//
//	field := logger.String("user_id", "abc-123")
//	logger.Info(ctx, "user action", field)
type Field struct {
	Key   string
	Value interface{}
}

// ToSlogAttr converts Field to slog.Attr for use with slog.Handler.
// Handles all standard types with appropriate slog.Kind.
// Complex types are converted via fmt.Sprint for slog.KindAny.
func (f Field) ToSlogAttr() slog.Attr {
	// Handle nil interface values first
	if f.Value == nil {
		return slog.String(f.Key, "<nil>")
	}

	switch v := f.Value.(type) {
	case string:
		return slog.String(f.Key, v)
	case int:
		return slog.Int(f.Key, v)
	case int64:
		return slog.Int64(f.Key, v)
	case float64:
		return slog.Float64(f.Key, v)
	case bool:
		return slog.Bool(f.Key, v)
	case time.Duration:
		return slog.String(f.Key, v.String())
	case time.Time:
		return slog.String(f.Key, v.Format(time.RFC3339))
	case error:
		if v == nil {
			return slog.String(f.Key, "<nil>")
		}
		return slog.String(f.Key, v.Error())
	default:
		return slog.Any(f.Key, v)
	}
}

// String creates a string field.
// Safe for all string values including empty strings.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field.
// Uses 32-bit int on 32-bit architectures, 64-bit on 64-bit.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
// Explicit 64-bit width regardless of architecture.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
// IEEE 754 double precision.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a time.Duration field.
// Serialized as human-readable string (e.g., "1s", "5m30s").
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time creates a time.Time field.
// Serialized as RFC3339 formatted string (e.g., "2026-01-15T10:30:00Z").
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Any creates a field with any value.
// Uses reflection for serialization; prefer typed constructors when possible.
// Value must be JSON-serializable.
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field.
// Key is always "error". Nil errors produce field with "<nil>" value.
//
// Usage:
//
//	logger.Error(ctx, "query failed", logger.Err(err))
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// fieldsToSlogAttrs converts a slice of Fields to a slice of slog.Attr.
// Helper function used by SlogLogger implementations.
func fieldsToSlogAttrs(fields []Field) []slog.Attr {
	if len(fields) == 0 {
		return nil
	}

	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	return attrs
}

// mergeFields combines base fields with additional fields.
// Returns a new slice containing all fields from base followed by fields from additional.
// Used by WithFields to create immutable logger instances.
func mergeFields(base, additional []Field) []Field {
	if len(additional) == 0 {
		return base
	}

	// Pre-allocate with exact capacity
	merged := make([]Field, len(base)+len(additional))
	copy(merged, base)
	copy(merged[len(base):], additional)
	return merged
}

// fieldKey returns the key of a field as a string.
// Helper for internal use.
func (f Field) fieldKey() string {
	return f.Key
}

// fieldValue returns the formatted value of a field.
// Helper for testing and debugging.
func (f Field) fieldValue() string {
	switch v := f.Value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case time.Duration:
		return v.String()
	case time.Time:
		return v.Format(time.RFC3339)
	case error:
		if v == nil {
			return "<nil>"
		}
		return v.Error()
	default:
		return fmt.Sprintf("%v", v)
	}
}
