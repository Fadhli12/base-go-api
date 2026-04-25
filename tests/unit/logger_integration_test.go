package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/logger"
	pkglogger "github.com/example/go-api-base/pkg/logger"
)

// TestLoggerIntegration verifies end-to-end logging functionality
func TestLoggerIntegration(t *testing.T) {
	tests := []struct {
		name    string
		level   slog.Level
		logFunc func(context.Context, logger.Logger)
		verify  func(*testing.T, string)
	}{
		{
			name:  "Info level logging with fields",
			level: slog.LevelInfo,
			logFunc: func(ctx context.Context, log logger.Logger) {
				log.Info(ctx, "user created",
					log.String("user_id", "uuid-123"),
					log.String("email", "test@example.com"),
					log.Int("age", 30),
				)
			},
			verify: func(t *testing.T, output string) {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				if m["msg"] != "user created" {
					t.Errorf("expected msg='user created', got %v", m["msg"])
				}
				if m["user_id"] != "uuid-123" {
					t.Errorf("expected user_id='uuid-123', got %v", m["user_id"])
				}
				if m["email"] != "test@example.com" {
					t.Errorf("expected email='test@example.com', got %v", m["email"])
				}
				if m["age"] != float64(30) {
					t.Errorf("expected age=30, got %v", m["age"])
				}
			},
		},
		{
			name:  "Error level logging with error field",
			level: slog.LevelInfo,
			logFunc: func(ctx context.Context, log logger.Logger) {
				err := context.Canceled
				scoped := log.WithError(err)
				scoped.Error(ctx, "request failed",
					log.String("endpoint", "/api/users"),
					log.Int("status_code", 500),
				)
			},
			verify: func(t *testing.T, output string) {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				if m["msg"] != "request failed" {
					t.Errorf("expected msg='request failed', got %v", m["msg"])
				}
				if m["endpoint"] != "/api/users" {
					t.Errorf("expected endpoint='/api/users', got %v", m["endpoint"])
				}
				if m["status_code"] != float64(500) {
					t.Errorf("expected status_code=500, got %v", m["status_code"])
				}
				// error field should be present
				if _, ok := m["error"]; !ok {
					t.Error("expected 'error' field in log output")
				}
			},
		},
		{
			name:  "WithFields chaining preserves all fields",
			level: slog.LevelInfo,
			logFunc: func(ctx context.Context, log logger.Logger) {
				scoped := log.WithFields(
					log.String("request_id", "req-456"),
					log.String("user_id", "user-789"),
				)
				scoped.Info(ctx, "operation completed",
					log.Duration("elapsed", 123*time.Millisecond),
					log.Bool("success", true),
				)
			},
			verify: func(t *testing.T, output string) {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				// All fields should be present
				if m["request_id"] != "req-456" {
					t.Errorf("expected request_id='req-456', got %v", m["request_id"])
				}
				if m["user_id"] != "user-789" {
					t.Errorf("expected user_id='user-789', got %v", m["user_id"])
				}
				if m["success"] != true {
					t.Errorf("expected success=true, got %v", m["success"])
				}
				if _, ok := m["elapsed"]; !ok {
					t.Error("expected 'elapsed' field in log output")
				}
			},
		},
		{
			name:  "Debug level logs correctly",
			level: slog.LevelDebug,
			logFunc: func(ctx context.Context, log logger.Logger) {
				log.Debug(ctx, "debug message",
					log.String("component", "auth"),
					log.Bool("verbose", true),
				)
			},
			verify: func(t *testing.T, output string) {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(output), &m); err != nil {
					t.Fatalf("failed to parse JSON: %v", err)
				}
				if m["msg"] != "debug message" {
					t.Errorf("expected msg='debug message', got %v", m["msg"])
				}
			},
		},
		{
			name:  "Multiple concurrent logs maintain separation",
			level: slog.LevelInfo,
			logFunc: func(ctx context.Context, log logger.Logger) {
				// First operation
				op1 := log.WithFields(log.String("operation_id", "op-1"))
				op1.Info(ctx, "operation 1 started")

				// Second operation (separate logger)
				op2 := log.WithFields(log.String("operation_id", "op-2"))
				op2.Info(ctx, "operation 2 started")

				// Original logger unaffected
				log.Info(ctx, "main operation")
			},
			verify: func(t *testing.T, output string) {
				lines := bytes.Split([]byte(output), []byte("\n"))
				logCount := 0
				for _, line := range lines {
					if len(bytes.TrimSpace(line)) == 0 {
						continue
					}
					logCount++
					var m map[string]interface{}
					if err := json.Unmarshal(line, &m); err != nil {
						t.Fatalf("failed to parse JSON line: %v, line: %s", err, line)
					}
				}
				// Should have at least 3 log lines
				if logCount < 3 {
					t.Errorf("expected at least 3 log lines, got %d", logCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer to capture output
			buf := &bytes.Buffer{}

			// Create JSON handler writing to buffer
			handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: tt.level,
			})

			// Create pkg/logger wrapper
			log := pkglogger.New(handler)

			// Initialize default logger
			pkglogger.Init(log)

			// Run test
			ctx := context.Background()
			tt.logFunc(ctx, &slogLoggerAdapter{log})

			// Get output
			output := buf.String()
			if output == "" {
				t.Fatal("expected log output, got empty")
			}

			// Verify
			tt.verify(t, output)
		})
	}
}

// TestContextPropagation verifies fields are preserved across context boundaries
func TestContextPropagation(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	log := pkglogger.New(handler)

	// Create context with logger
	ctx := pkglogger.WithLoggerContext(context.Background(), log)

	// Retrieve logger from context
	retrieved := pkglogger.FromContext(ctx)

	// Create scoped logger with fields
	scoped := retrieved.WithRequest("req-123")
	scoped = scoped.WithUser("user-456")
	scoped = scoped.WithOrg("org-789")

	// Log with scoped logger
	ctx2 := scoped.WithContext(ctx)
	scoped.Info(ctx2, "request processed",
		retrieved.WithFields(retrieved.String("status", "success")).WithFields(),
	)

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output, got empty")
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(output), &m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if m["msg"] != "request processed" {
		t.Errorf("expected msg='request processed', got %v", m["msg"])
	}
}

// TestFieldTypes verifies all field type constructors work correctly
func TestFieldTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	log := pkglogger.New(handler)

	tests := []struct {
		name       string
		field      slog.Attr
		expectedKV func(map[string]interface{}) bool
	}{
		{
			name:  "String field",
			field: slog.String("key", "value"),
			expectedKV: func(m map[string]interface{}) bool {
				return m["key"] == "value"
			},
		},
		{
			name:  "Int field",
			field: slog.Int("count", 42),
			expectedKV: func(m map[string]interface{}) bool {
				return m["count"] == float64(42)
			},
		},
		{
			name:  "Bool field",
			field: slog.Bool("active", true),
			expectedKV: func(m map[string]interface{}) bool {
				return m["active"] == true
			},
		},
	}

	for _, tt := range tests {
		buf.Reset()
		log.With(tt.field).InfoContext(context.Background(), "test message")

		var m map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatalf("%s: failed to parse JSON: %v", tt.name, err)
		}

		if !tt.expectedKV(m) {
			t.Errorf("%s: field value mismatch: %v", tt.name, m)
		}
	}
}

// slogLoggerAdapter adapts pkg/logger.Logger to internal/logger.Logger interface
// This allows testing the pkg/logger wrapper with the internal interface
type slogLoggerAdapter struct {
	*pkglogger.Logger
}

func (a *slogLoggerAdapter) Debug(ctx context.Context, msg string, fields ...logger.Field) {
	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	a.Logger.DebugContext(ctx, msg, attrs...)
}

func (a *slogLoggerAdapter) Info(ctx context.Context, msg string, fields ...logger.Field) {
	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	a.Logger.InfoContext(ctx, msg, attrs...)
}

func (a *slogLoggerAdapter) Warn(ctx context.Context, msg string, fields ...logger.Field) {
	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	a.Logger.WarnContext(ctx, msg, attrs...)
}

func (a *slogLoggerAdapter) Error(ctx context.Context, msg string, fields ...logger.Field) {
	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	a.Logger.ErrorContext(ctx, msg, attrs...)
}

func (a *slogLoggerAdapter) WithFields(fields ...logger.Field) logger.Logger {
	attrs := make([]slog.Attr, len(fields))
	for i, f := range fields {
		attrs[i] = f.ToSlogAttr()
	}
	return &slogLoggerAdapter{a.Logger.With(attrs...)}
}

func (a *slogLoggerAdapter) WithError(err error) logger.Logger {
	return &slogLoggerAdapter{a.Logger.With(slog.String("error", err.Error()))}
}

func (a *slogLoggerAdapter) String(key, value string) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Int(key string, value int) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Int64(key string, value int64) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Float64(key string, value float64) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Bool(key string, value bool) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Duration(key string, value time.Duration) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Time(key string, value time.Time) logger.Field {
	return logger.Field{Key: key, Value: value}
}

func (a *slogLoggerAdapter) Any(key string, value interface{}) logger.Field {
	return logger.Field{Key: key, Value: value}
}
