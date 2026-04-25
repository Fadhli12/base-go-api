package logger

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFieldToSlogAttr(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected slog.Attr
	}{
		{
			name:     "string field",
			field:    Field{Key: "name", Value: "test"},
			expected: slog.String("name", "test"),
		},
		{
			name:     "int field",
			field:    Field{Key: "count", Value: 42},
			expected: slog.Int("count", 42),
		},
		{
			name:     "int64 field",
			field:    Field{Key: "timestamp", Value: int64(1234567890)},
			expected: slog.Int64("timestamp", 1234567890),
		},
		{
			name:     "float64 field",
			field:    Field{Key: "pi", Value: 3.14},
			expected: slog.Float64("pi", 3.14),
		},
		{
			name:     "bool field true",
			field:    Field{Key: "active", Value: true},
			expected: slog.Bool("active", true),
		},
		{
			name:     "bool field false",
			field:    Field{Key: "active", Value: false},
			expected: slog.Bool("active", false),
		},
		{
			name:     "duration field",
			field:    Field{Key: "elapsed", Value: time.Second},
			expected: slog.String("elapsed", "1s"),
		},
		{
			name:     "time field",
			field:    Field{Key: "created", Value: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)},
			expected: slog.String("created", "2026-01-15T10:30:00Z"),
		},
		{
			name:     "error field",
			field:    Field{Key: "error", Value: errors.New("something went wrong")},
			expected: slog.String("error", "something went wrong"),
		},
		{
			name:     "nil error field",
			field:    Field{Key: "error", Value: error(nil)},
			expected: slog.String("error", "<nil>"),
		},
		{
			name:     "any field",
			field:    Field{Key: "data", Value: map[string]int{"a": 1}},
			expected: slog.Any("data", map[string]int{"a": 1}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.field.ToSlogAttr()
			assert.Equal(t, tt.expected.Key, result.Key)
			assert.Equal(t, tt.expected.Value, result.Value)
		})
	}
}

func TestFieldConstructors(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		f := String("key", "value")
		assert.Equal(t, "key", f.Key)
		assert.Equal(t, "value", f.Value)
	})

	t.Run("Int", func(t *testing.T) {
		f := Int("count", 42)
		assert.Equal(t, "count", f.Key)
		assert.Equal(t, 42, f.Value)
	})

	t.Run("Int64", func(t *testing.T) {
		f := Int64("timestamp", int64(1234567890))
		assert.Equal(t, "timestamp", f.Key)
		assert.Equal(t, int64(1234567890), f.Value)
	})

	t.Run("Float64", func(t *testing.T) {
		f := Float64("pi", 3.14159)
		assert.Equal(t, "pi", f.Key)
		assert.Equal(t, 3.14159, f.Value)
	})

	t.Run("Bool", func(t *testing.T) {
		f := Bool("active", true)
		assert.Equal(t, "active", f.Key)
		assert.Equal(t, true, f.Value)
	})

	t.Run("Duration", func(t *testing.T) {
		d := 5 * time.Minute
		f := Duration("elapsed", d)
		assert.Equal(t, "elapsed", f.Key)
		assert.Equal(t, d, f.Value)
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()
		f := Time("created", now)
		assert.Equal(t, "created", f.Key)
		assert.Equal(t, now, f.Value)
	})

	t.Run("Any", func(t *testing.T) {
		data := map[string]int{"a": 1}
		f := Any("data", data)
		assert.Equal(t, "data", f.Key)
		assert.Equal(t, data, f.Value)
	})

	t.Run("Err with error", func(t *testing.T) {
		err := errors.New("test error")
		f := Err(err)
		assert.Equal(t, "error", f.Key)
		assert.Equal(t, err, f.Value)
	})

	t.Run("Err with nil error", func(t *testing.T) {
		f := Err(nil)
		assert.Equal(t, "error", f.Key)
		assert.Nil(t, f.Value)
	})
}

func TestFieldsToSlogAttrs(t *testing.T) {
	t.Run("empty fields", func(t *testing.T) {
		result := fieldsToSlogAttrs([]Field{})
		assert.Nil(t, result)
	})

	t.Run("nil fields", func(t *testing.T) {
		result := fieldsToSlogAttrs(nil)
		assert.Nil(t, result)
	})

	t.Run("multiple fields", func(t *testing.T) {
		fields := []Field{
			String("key1", "value1"),
			Int("key2", 42),
			Bool("key3", true),
		}
		result := fieldsToSlogAttrs(fields)
		assert.Len(t, result, 3)
		assert.Equal(t, "key1", result[0].Key)
		assert.Equal(t, "key2", result[1].Key)
		assert.Equal(t, "key3", result[2].Key)
	})
}

func TestMergeFields(t *testing.T) {
	t.Run("both empty", func(t *testing.T) {
		result := mergeFields(nil, nil)
		assert.Empty(t, result)
	})

	t.Run("base only", func(t *testing.T) {
		base := []Field{String("key1", "value1")}
		result := mergeFields(base, nil)
		assert.Len(t, result, 1)
		assert.Equal(t, "key1", result[0].Key)
	})

	t.Run("additional only", func(t *testing.T) {
		additional := []Field{String("key1", "value1")}
		result := mergeFields(nil, additional)
		assert.Len(t, result, 1)
		assert.Equal(t, "key1", result[0].Key)
	})

	t.Run("both populated", func(t *testing.T) {
		base := []Field{String("key1", "value1")}
		additional := []Field{Int("key2", 42)}
		result := mergeFields(base, additional)
		assert.Len(t, result, 2)
		assert.Equal(t, "key1", result[0].Key)
		assert.Equal(t, "key2", result[1].Key)
	})

	t.Run("preserves order", func(t *testing.T) {
		base := []Field{String("first", "1")}
		additional := []Field{String("second", "2"), String("third", "3")}
		result := mergeFields(base, additional)
		assert.Len(t, result, 3)
		assert.Equal(t, "first", result[0].Key)
		assert.Equal(t, "second", result[1].Key)
		assert.Equal(t, "third", result[2].Key)
	})
}

func TestFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected string
	}{
		{"string", String("k", "v"), "v"},
		{"int", Int("k", 42), "42"},
		{"int64", Int64("k", 64), "64"},
		{"float64", Float64("k", 3.14), "3.14"},
		{"bool true", Bool("k", true), "true"},
		{"bool false", Bool("k", false), "false"},
		{"duration", Duration("k", time.Second), "1s"},
		{"nil error", Err(nil), "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.field.fieldValue()
			assert.Equal(t, tt.expected, result)
		})
	}
}
