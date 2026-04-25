package logger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriterInterface verifies Writer interface is properly defined.
func TestWriterInterface(t *testing.T) {
	// Test that different writer types implement Writer
	var _ Writer = (*StdoutWriter)(nil)
	var _ Writer = (*FileWriter)(nil)
	var _ Writer = (*MultiWriter)(nil)
	var _ Writer = (*SyslogWriter)(nil)
	var _ Writer = (WriterFunc)(nil)
}

// TestWriterFunc verifies WriterFunc adapter.
func TestWriterFunc(t *testing.T) {
	var called bool
	var receivedEntry LogEntry

	wf := WriterFunc(func(ctx context.Context, entry LogEntry) error {
		called = true
		receivedEntry = entry
		return nil
	})

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "test message",
	}

	ctx := context.Background()
	err := wf.Write(ctx, entry)

	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "INFO", receivedEntry.Level)
	assert.Equal(t, "test message", receivedEntry.Message)

	// Close should be no-op
	err = wf.Close()
	assert.NoError(t, err)
}

// TestMultiWriter verifies MultiWriter functionality.
func TestMultiWriter(t *testing.T) {
	t.Run("write to multiple writers", func(t *testing.T) {
		var mu sync.Mutex
		var entries1, entries2 []LogEntry

		w1 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			mu.Lock()
			entries1 = append(entries1, entry)
			mu.Unlock()
			return nil
		})

		w2 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			mu.Lock()
			entries2 = append(entries2, entry)
			mu.Unlock()
			return nil
		})

		mw := NewMultiWriter(w1, w2)
		defer mw.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test message",
		}

		err := mw.Write(context.Background(), entry)
		assert.NoError(t, err)

		// Allow time for writes to complete
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		assert.Len(t, entries1, 1)
		assert.Len(t, entries2, 1)
		assert.Equal(t, "test message", entries1[0].Message)
		assert.Equal(t, "test message", entries2[0].Message)
		mu.Unlock()
	})

	t.Run("continues on writer error", func(t *testing.T) {
		var mu sync.Mutex
		called := false

		w1 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return assert.AnError
		})

		w2 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			mu.Lock()
			called = true
			mu.Unlock()
			return nil
		})

		mw := NewMultiWriter(w1, w2)
		defer mw.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err := mw.Write(context.Background(), entry)
		// Should not return error since one writer succeeded
		assert.NoError(t, err)

		// Allow time for writes
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		assert.True(t, called, "second writer should have been called")
		mu.Unlock()
	})

	t.Run("returns error if all writers fail", func(t *testing.T) {
		w1 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return assert.AnError
		})

		w2 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return assert.AnError
		})

		mw := NewMultiWriter(w1, w2)
		defer mw.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err := mw.Write(context.Background(), entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all writers failed")
	})

	t.Run("handles empty writer list", func(t *testing.T) {
		mw := NewMultiWriter()
		defer mw.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err := mw.Write(context.Background(), entry)
		assert.NoError(t, err)
	})

	t.Run("close is safe to call multiple times", func(t *testing.T) {
		w := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return nil
		})

		mw := NewMultiWriter(w)

		err := mw.Close()
		assert.NoError(t, err)

		err = mw.Close()
		assert.NoError(t, err)
	})

	t.Run("returns error when closed", func(t *testing.T) {
		mw := NewMultiWriter()
		mw.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err := mw.Write(context.Background(), entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("add writer", func(t *testing.T) {
		mw := NewMultiWriter()
		defer mw.Close()

		var called bool
		w := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			called = true
			return nil
		})

		err := mw.AddWriter(w)
		assert.NoError(t, err)

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err = mw.Write(context.Background(), entry)
		assert.NoError(t, err)

		assert.True(t, called)
	})

	t.Run("remove writer", func(t *testing.T) {
		// Note: WriterFunc cannot be compared with ==
		// RemoveWriter only works with comparable types
		// This test verifies the method exists and can be called
		// but doesn't test removal of function writers

		mw := NewMultiWriter()
		defer mw.Close()

		// Since we can't use WriterFunc with RemoveWriter,
		// we just verify the method doesn't panic
		removed := mw.RemoveWriter(nil)
		assert.False(t, removed)
	})

	t.Run("len returns correct count", func(t *testing.T) {
		w1 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return nil
		})

		w2 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
			return nil
		})

		mw := NewMultiWriter(w1, w2)
		defer mw.Close()

		assert.Equal(t, 2, mw.Len())
	})
}

// TestStdoutWriter verifies StdoutWriter functionality.
func TestStdoutWriter(t *testing.T) {
	t.Run("create with json format", func(t *testing.T) {
		w, err := NewStdoutWriter("json")
		require.NoError(t, err)
		defer w.Close()

		assert.Equal(t, FormatJSON, w.Format())
	})

	t.Run("create with text format", func(t *testing.T) {
		w, err := NewStdoutWriter("text")
		require.NoError(t, err)
		defer w.Close()

		assert.Equal(t, FormatText, w.Format())
	})

	t.Run("default format is json", func(t *testing.T) {
		w, err := NewStdoutWriter("")
		require.NoError(t, err)
		defer w.Close()

		assert.Equal(t, FormatJSON, w.Format())
	})

	t.Run("set format changes format", func(t *testing.T) {
		w, err := NewStdoutWriter("json")
		require.NoError(t, err)
		defer w.Close()

		assert.Equal(t, FormatJSON, w.Format())

		w.SetFormat("text")
		assert.Equal(t, FormatText, w.Format())
	})

	t.Run("close returns error when writing after close", func(t *testing.T) {
		w, err := NewStdoutWriter("json")
		require.NoError(t, err)

		err = w.Close()
		assert.NoError(t, err)

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err = w.Write(context.Background(), entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

// TestFormatParsing verifies Format parsing.
func TestFormatParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"Json", FormatJSON},
		{"text", FormatText},
		{"TEXT", FormatText},
		{"txt", FormatText},
		{"TXT", FormatText},
		{"unknown", FormatJSON},
		{"", FormatJSON},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFileWriter verifies FileWriter functionality.
func TestFileWriter(t *testing.T) {
	// Skip on Windows due to file locking issues
	if os.Getenv("CI") != "" && os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping FileWriter tests on Windows CI due to file locking")
	}

	t.Run("create with valid config", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test.log")

		config := FileConfig{
			Path:       logPath,
			MaxSize:    100,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   true,
		}

		w, err := NewFileWriter(config, "json")
		require.NoError(t, err)
		defer w.Close()

		assert.Equal(t, logPath, w.Path())
		assert.FileExists(t, logPath)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "subdir", "another", "test.log")

		config := FileConfig{
			Path:       logPath,
			MaxSize:    100,
			MaxBackups: 10,
			MaxAge:     30,
			Compress:   true,
		}

		w, err := NewFileWriter(config, "json")
		require.NoError(t, err)
		defer w.Close()

		assert.FileExists(t, logPath)
	})

	t.Run("applies defaults", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test.log")

		config := FileConfig{
			Path: logPath,
			// Leave other fields zero, but set Compress explicitly
			Compress: true,
		}

		w, err := NewFileWriter(config, "json")
		require.NoError(t, err)
		defer w.Close()

		cfg := w.Config()
		assert.Equal(t, logPath, cfg.Path)
		assert.Equal(t, 100, cfg.MaxSize)
		assert.Equal(t, 10, cfg.MaxBackups)
		assert.Equal(t, 30, cfg.MaxAge)
		assert.True(t, cfg.Compress)
	})

	t.Run("returns error when closed", func(t *testing.T) {
		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test.log")

		config := FileConfig{
			Path: logPath,
		}

		w, err := NewFileWriter(config, "json")
		require.NoError(t, err)

		w.Close()

		entry := LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "test",
		}

		err = w.Write(context.Background(), entry)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
	})
}

// TestConfig verifies Config functionality.
func TestConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := DefaultConfig()

		assert.Equal(t, "info", cfg.Level)
		assert.Equal(t, "json", cfg.Format)
		assert.Equal(t, "stdout", cfg.Outputs)
		assert.Equal(t, "/var/log/api.log", cfg.FilePath)
		assert.Equal(t, 100, cfg.FileMaxSize)
		assert.Equal(t, 10, cfg.FileMaxBackups)
		assert.Equal(t, 30, cfg.FileMaxAge)
		assert.True(t, cfg.FileCompress)
		assert.Equal(t, "", cfg.SyslogNetwork)
		assert.Equal(t, "", cfg.SyslogAddress)
		assert.Equal(t, "go-api", cfg.SyslogTag)
		assert.False(t, cfg.AddSource)
	})

	t.Run("parse outputs", func(t *testing.T) {
		tests := []struct {
			input    string
			expected []string
		}{
			{"stdout", []string{"stdout"}},
			{"file", []string{"file"}},
			{"syslog", []string{"syslog"}},
			{"stdout,file", []string{"stdout", "file"}},
			{"stdout, file, syslog", []string{"stdout", "file", "syslog"}},
			{"", []string{"stdout"}},
			{"STDOUT, FILE", []string{"stdout", "file"}},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				cfg := Config{Outputs: tt.input}
				result := cfg.ParseOutputs()
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("parse level", func(t *testing.T) {
		cfg := Config{Level: "debug"}
		assert.Equal(t, LevelDebug, cfg.ParseLevel())

		cfg = Config{Level: "info"}
		assert.Equal(t, LevelInfo, cfg.ParseLevel())

		cfg = Config{Level: "warn"}
		assert.Equal(t, LevelWarn, cfg.ParseLevel())

		cfg = Config{Level: "error"}
		assert.Equal(t, LevelError, cfg.ParseLevel())

		cfg = Config{Level: "unknown"}
		assert.Equal(t, LevelInfo, cfg.ParseLevel()) // default
	})

	t.Run("to file config", func(t *testing.T) {
		cfg := Config{
			FilePath:       "/var/log/app.log",
			FileMaxSize:    50,
			FileMaxBackups: 5,
			FileMaxAge:     7,
			FileCompress:   false,
		}

		fileCfg := cfg.ToFileConfig()
		assert.Equal(t, cfg.FilePath, fileCfg.Path)
		assert.Equal(t, cfg.FileMaxSize, fileCfg.MaxSize)
		assert.Equal(t, cfg.FileMaxBackups, fileCfg.MaxBackups)
		assert.Equal(t, cfg.FileMaxAge, fileCfg.MaxAge)
		assert.Equal(t, cfg.FileCompress, fileCfg.Compress)
	})

	t.Run("validate valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("validate invalid level", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Level = "invalid"
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LOG_LEVEL")
	})

	t.Run("validate invalid format", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Format = "invalid"
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LOG_FORMAT")
	})

	t.Run("validate invalid output", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Outputs = "invalid"
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LOG_OUTPUTS")
	})

	t.Run("validate file path required", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Outputs = "file"
		cfg.FilePath = ""
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LOG_FILE_PATH")
	})

	t.Run("validate syslog tag required", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Outputs = "syslog"
		cfg.SyslogTag = ""
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LOG_SYSLOG_TAG")
	})
}

// TestConfigFromEnv verifies ConfigFromEnv functionality.
func TestConfigFromEnv(t *testing.T) {
	t.Run("uses defaults when no env vars set", func(t *testing.T) {
		// Clear env vars
		for _, key := range []string{
			"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUTS", "LOG_FILE_PATH",
			"LOG_FILE_MAX_SIZE", "LOG_FILE_MAX_BACKUPS", "LOG_FILE_MAX_AGE",
			"LOG_FILE_COMPRESS", "LOG_SYSLOG_NETWORK", "LOG_SYSLOG_ADDRESS",
			"LOG_SYSLOG_TAG", "LOG_ADD_SOURCE",
		} {
			os.Unsetenv(key)
		}

		cfg := ConfigFromEnv()
		defaultCfg := DefaultConfig()

		assert.Equal(t, defaultCfg.Level, cfg.Level)
		assert.Equal(t, defaultCfg.Format, cfg.Format)
		assert.Equal(t, defaultCfg.Outputs, cfg.Outputs)
	})

	t.Run("reads env vars", func(t *testing.T) {
		// Clear first
		for _, key := range []string{
			"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUTS", "LOG_FILE_PATH",
			"LOG_FILE_MAX_SIZE", "LOG_FILE_MAX_BACKUPS", "LOG_FILE_MAX_AGE",
			"LOG_FILE_COMPRESS", "LOG_SYSLOG_NETWORK", "LOG_SYSLOG_ADDRESS",
			"LOG_SYSLOG_TAG", "LOG_ADD_SOURCE",
		} {
			os.Unsetenv(key)
		}

		// Set test values
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_FORMAT", "text")
		os.Setenv("LOG_OUTPUTS", "stdout,file")
		os.Setenv("LOG_FILE_PATH", "/tmp/test.log")
		os.Setenv("LOG_FILE_MAX_SIZE", "50")
		os.Setenv("LOG_FILE_MAX_BACKUPS", "5")
		os.Setenv("LOG_FILE_MAX_AGE", "7")
		os.Setenv("LOG_FILE_COMPRESS", "false")
		os.Setenv("LOG_SYSLOG_NETWORK", "tcp")
		os.Setenv("LOG_SYSLOG_ADDRESS", "localhost:514")
		os.Setenv("LOG_SYSLOG_TAG", "test-app")
		os.Setenv("LOG_ADD_SOURCE", "true")

		// Cleanup after test
		defer func() {
			for _, key := range []string{
				"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUTS", "LOG_FILE_PATH",
				"LOG_FILE_MAX_SIZE", "LOG_FILE_MAX_BACKUPS", "LOG_FILE_MAX_AGE",
				"LOG_FILE_COMPRESS", "LOG_SYSLOG_NETWORK", "LOG_SYSLOG_ADDRESS",
				"LOG_SYSLOG_TAG", "LOG_ADD_SOURCE",
			} {
				os.Unsetenv(key)
			}
		}()

		cfg := ConfigFromEnv()

		assert.Equal(t, "debug", cfg.Level)
		assert.Equal(t, "text", cfg.Format)
		assert.Equal(t, "stdout,file", cfg.Outputs)
		assert.Equal(t, "/tmp/test.log", cfg.FilePath)
		assert.Equal(t, 50, cfg.FileMaxSize)
		assert.Equal(t, 5, cfg.FileMaxBackups)
		assert.Equal(t, 7, cfg.FileMaxAge)
		assert.False(t, cfg.FileCompress)
		assert.Equal(t, "tcp", cfg.SyslogNetwork)
		assert.Equal(t, "localhost:514", cfg.SyslogAddress)
		assert.Equal(t, "test-app", cfg.SyslogTag)
		assert.True(t, cfg.AddSource)
	})
}

// TestNewLogger verifies NewLogger factory function.
func TestNewLogger(t *testing.T) {
	t.Run("create with stdout output", func(t *testing.T) {
		cfg := Config{
			Level:   "info",
			Format:  "json",
			Outputs: "stdout",
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		assert.NotNil(t, logger)

		// Verify it's a SlogLogger
		_, ok := logger.(*SlogLogger)
		assert.True(t, ok)
	})

	t.Run("create with file output", func(t *testing.T) {
		// Skip on Windows - lumberjack holds file lock that prevents cleanup
		if runtime.GOOS == "windows" {
			t.Skip("Skipping file output test on Windows due to file locking")
		}

		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test.log")

		cfg := Config{
			Level:          "info",
			Format:         "json",
			Outputs:        "file",
			FilePath:       logPath,
			FileMaxSize:    100,
			FileMaxBackups: 10,
			FileMaxAge:     30,
			FileCompress:   true,
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("create with multiple outputs", func(t *testing.T) {
		// Skip on Windows - lumberjack holds file lock that prevents cleanup
		if runtime.GOOS == "windows" {
			t.Skip("Skipping file output test on Windows due to file locking")
		}

		tempDir := t.TempDir()
		logPath := filepath.Join(tempDir, "test.log")

		cfg := Config{
			Level:          "info",
			Format:         "json",
			Outputs:        "stdout,file",
			FilePath:       logPath,
			FileMaxSize:    100,
			FileMaxBackups: 10,
			FileMaxAge:     30,
			FileCompress:   true,
		}

		logger, err := NewLogger(cfg)
		require.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("create with syslog output", func(t *testing.T) {
		// On Windows this will create a stub, on Unix it will try to connect
		cfg := Config{
			Level:         "info",
			Format:        "json",
			Outputs:       "syslog",
			SyslogNetwork: "",
			SyslogAddress: "",
			SyslogTag:     "test-app",
		}

		logger, err := NewLogger(cfg)
		// May fail on some systems without syslog
		if err == nil {
			assert.NotNil(t, logger)
		}
	})

	t.Run("fails on invalid config", func(t *testing.T) {
		cfg := Config{
			Level:   "invalid",
			Outputs: "stdout",
		}

		logger, err := NewLogger(cfg)
		assert.Error(t, err)
		assert.Nil(t, logger)
	})

	t.Run("fails on missing file path", func(t *testing.T) {
		cfg := Config{
			Level:   "info",
			Outputs: "file",
			// Missing FilePath
		}

		logger, err := NewLogger(cfg)
		assert.Error(t, err)
		assert.Nil(t, logger)
	})
}

// TestNewDefaultLogger verifies NewDefaultLogger function.
func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	assert.NotNil(t, logger)

	// Verify it can log
	ctx := context.Background()
	logger.Info(ctx, "test message", logger.String("key", "value"))
}

// TestParseBool verifies parseBool function.
func TestParseBool(t *testing.T) {
	tests := []struct {
		input         string
		defaultValue  bool
		expected      bool
	}{
		{"true", false, true},
		{"TRUE", false, true},
		{"True", false, true},
		{"1", false, true},
		{"yes", false, true},
		{"on", false, true},
		{"false", true, false},
		{"FALSE", true, false},
		{"False", true, false},
		{"0", true, false},
		{"no", true, false},
		{"off", true, false},
		{"invalid", true, true},
		{"invalid", false, false},
		{"", true, true},
		{"", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContainsString verifies containsString function.
func TestContainsString(t *testing.T) {
	tests := []struct {
		slice    []string
		s        string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"a"}, "a", true},
		{nil, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			result := containsString(tt.slice, tt.s)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// BenchmarkMultiWriter benchmarks MultiWriter performance.
func BenchmarkMultiWriter(b *testing.B) {
	w1 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
		return nil
	})

	w2 := WriterFunc(func(ctx context.Context, entry LogEntry) error {
		return nil
	})

	mw := NewMultiWriter(w1, w2)
	defer mw.Close()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "benchmark message",
		Fields:    map[string]interface{}{"key": "value"},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = mw.Write(ctx, entry)
	}
}

// BenchmarkConfigValidation benchmarks config validation.
func BenchmarkConfigValidation(b *testing.B) {
	cfg := DefaultConfig()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

// TestLogEntrySerialization verifies LogEntry JSON serialization.
func TestLogEntrySerialization(t *testing.T) {
	entry := LogEntry{
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level:      "INFO",
		Message:    "test message",
		RequestID:  "req-123",
		UserID:     "user-456",
		OrgID:      "org-789",
		TraceID:    "trace-abc",
		Fields:     map[string]interface{}{"key": "value"},
		Error:      "some error",
		StackTrace: "at line 1",
		Source:     "file.go:123",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	// Verify JSON structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "2024-01-15T10:30:00Z", result["timestamp"])
	assert.Equal(t, "INFO", result["level"])
	assert.Equal(t, "test message", result["message"])
	assert.Equal(t, "req-123", result["request_id"])
	assert.Equal(t, "user-456", result["user_id"])
	assert.Equal(t, "org-789", result["org_id"])
	assert.Equal(t, "trace-abc", result["trace_id"])
	assert.Equal(t, "some error", result["error"])
	assert.Equal(t, "at line 1", result["stack_trace"])
	assert.Equal(t, "file.go:123", result["source"])

	fields := result["fields"].(map[string]interface{})
	assert.Equal(t, "value", fields["key"])
}
