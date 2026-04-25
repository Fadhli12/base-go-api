package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{"Debug", LevelDebug, "DEBUG"},
		{"Info", LevelInfo, "INFO"},
		{"Warn", LevelWarn, "WARN"},
		{"Error", LevelError, "ERROR"},
		{"Unknown", Level(99), "Level(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Level
	}{
		{"debug lowercase", "debug", LevelDebug},
		{"DEBUG uppercase", "DEBUG", LevelDebug},
		{"info lowercase", "info", LevelInfo},
		{"INFO uppercase", "INFO", LevelInfo},
		{"warn lowercase", "warn", LevelWarn},
		{"WARN uppercase", "WARN", LevelWarn},
		{"warning variant", "warning", LevelWarn},
		{"WARNING variant", "WARNING", LevelWarn},
		{"error lowercase", "error", LevelError},
		{"ERROR uppercase", "ERROR", LevelError},
		{"unknown level", "unknown", LevelInfo},
		{"empty string", "", LevelInfo},
		{"random text", "foobar", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseLevelCaseInsensitivity(t *testing.T) {
	// Test supported cases (lowercase and uppercase)
	assert.Equal(t, LevelDebug, ParseLevel("DEBUG"))
	assert.Equal(t, LevelDebug, ParseLevel("debug"))
	assert.Equal(t, LevelInfo, ParseLevel("Info")) // Mixed case returns default (Info)
}
