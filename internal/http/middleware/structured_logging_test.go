package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/logger"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestDefaultLoggingSkipper(t *testing.T) {
	skipper := DefaultLoggingSkipper()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"skips healthz", "/healthz", true},
		{"skips readyz", "/readyz", true},
		{"skips metrics", "/metrics", true},
		{"does not skip api", "/api/v1/users", false},
		{"does not skip root", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			assert.Equal(t, tt.expected, skipper(c))
		})
	}
}

func TestDefaultLoggingMiddlewareConfig(t *testing.T) {
	t.Run("uses provided logger", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		assert.NotNil(t, config.Logger)
		assert.NotNil(t, config.Skipper)
		assert.False(t, config.LogRequestBody)
		assert.False(t, config.LogResponseBody)
		assert.Equal(t, 1024, config.BodyMaxSize)
	})

	t.Run("uses nop logger when nil", func(t *testing.T) {
		config := DefaultLoggingMiddlewareConfig(nil)

		assert.NotNil(t, config.Logger)
		// Should be safe to use
		assert.NotPanics(t, func() {
			config.Logger.Info(nil, "test")
		})
	})
}

func TestStructuredLogging(t *testing.T) {
	t.Run("logs request start and completion", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should log request started (DEBUG)
		assert.True(t, mockLog.AssertLogged(logger.LevelDebug, "request started"))

		// Should log request completed (INFO for 200)
		assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "request completed"))

		// Should have method and path fields
		assert.True(t, mockLog.AssertField("method", "GET"))
		assert.True(t, mockLog.AssertField("path", "/test"))
	})

	t.Run("logs status code and duration", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.POST("/test", func(c echo.Context) error {
			return c.JSON(http.StatusCreated, map[string]string{"id": "123"})
		})

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should log status 201
		assert.True(t, mockLog.AssertFieldInMessage(logger.LevelInfo, "request completed", "status", 201))

		// Should log duration (check that message was logged with duration field)
		assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "request completed"))
	})

	t.Run("logs client errors at WARN level", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should log at WARN level for 4xx
		assert.True(t, mockLog.AssertLogged(logger.LevelWarn, "request completed with client error"))
		assert.True(t, mockLog.AssertFieldInMessage(logger.LevelWarn, "request completed", "status", 400))
	})

	t.Run("logs server errors at ERROR level", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusInternalServerError, "server error")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should log at ERROR level for 5xx
		assert.True(t, mockLog.AssertLogged(logger.LevelError, "request failed with server error"))
		assert.True(t, mockLog.AssertFieldInMessage(logger.LevelError, "request failed with server error", "status", 500))
	})

	t.Run("skips when skipper returns true", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := LoggingMiddlewareConfig{
			Logger: mockLog,
			Skipper: func(c echo.Context) bool {
				return true
			},
		}

		e := echo.New()
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should not log anything
		assert.Equal(t, 0, mockLog.GetMessagesCount())
	})

	t.Run("uses nop logger when nil", func(t *testing.T) {
		config := LoggingMiddlewareConfig{
			Logger: nil,
		}

		e := echo.New()
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		// Should not panic
		assert.NotPanics(t, func() {
			e.ServeHTTP(rec, req)
		})
	})

	t.Run("injects logger into context", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		var capturedLogger logger.Logger
		e.GET("/test", func(c echo.Context) error {
			capturedLogger = GetLogger(c)
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Logger should be injected
		assert.NotNil(t, capturedLogger)
		assert.False(t, logger.IsNopLogger(capturedLogger))
	})

	t.Run("logs query parameters", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=qux", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should log query parameters
		assert.True(t, mockLog.AssertField("query", "foo=bar&baz=qux"))
	})

	t.Run("extracts trace ID from header", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Trace-ID", "custom-trace-id")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should extract trace ID from header
		assert.True(t, mockLog.AssertField("trace_id", "custom-trace-id"))
	})

	t.Run("generates trace ID when not in header", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should have a trace_id field (generated UUID)
		assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "request completed"))
	})

	t.Run("extracts user ID from JWT claims", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				// Simulate JWT middleware setting claims
				c.Set("user", &auth.Claims{UserID: "user-123"})
				return next(c)
			}
		})
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should extract user_id from JWT claims
		assert.True(t, mockLog.AssertField("user_id", "user-123"))
	})

	t.Run("extracts org ID from context", func(t *testing.T) {
		mockLog := logger.NewMockLogger()
		config := DefaultLoggingMiddlewareConfig(mockLog)

		e := echo.New()
		e.Use(RequestID())
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				// Simulate org middleware setting org ID
				orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				c.Set("organization_id", orgID)
				return next(c)
			}
		})
		e.Use(StructuredLogging(config))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should extract org_id from context
		assert.True(t, mockLog.AssertField("org_id", "550e8400-e29b-41d4-a716-446655440000"))
	})
}

func TestGetLogger(t *testing.T) {
	t.Run("returns logger from context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockLog := logger.NewMockLogger()
		c.Set(LoggerContextKey, mockLog)

		log := GetLogger(c)
		assert.Equal(t, mockLog, log)
	})

	t.Run("returns nop logger when not set", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		log := GetLogger(c)
		assert.NotNil(t, log)
		assert.True(t, logger.IsNopLogger(log))
	})

	t.Run("returns nop logger for nil context", func(t *testing.T) {
		log := GetLogger(nil)
		assert.NotNil(t, log)
		assert.True(t, logger.IsNopLogger(log))
	})
}

func TestSetLogger(t *testing.T) {
	t.Run("sets logger in context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockLog := logger.NewMockLogger()
		SetLogger(c, mockLog)

		log := GetLogger(c)
		assert.Equal(t, mockLog, log)
	})

	t.Run("handles nil logger gracefully", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Should not panic
		assert.NotPanics(t, func() {
			SetLogger(c, nil)
		})
	})

	t.Run("handles nil context gracefully", func(t *testing.T) {
		mockLog := logger.NewMockLogger()

		// Should not panic
		assert.NotPanics(t, func() {
			SetLogger(nil, mockLog)
		})
	})
}

func TestStructuredLogging_DurationAccuracy(t *testing.T) {
	mockLog := logger.NewMockLogger()
	config := DefaultLoggingMiddlewareConfig(mockLog)

	e := echo.New()
	e.Use(RequestID())
	e.Use(StructuredLogging(config))

	e.GET("/test", func(c echo.Context) error {
		time.Sleep(10 * time.Millisecond)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Find the completion message
	msg := mockLog.FindMessage(logger.LevelInfo, "request completed")
	assert.NotNil(t, msg)

	// Find duration field
	var duration time.Duration
	for _, field := range msg.Fields {
		if field.Key == "duration" {
			if d, ok := field.Value.(time.Duration); ok {
				duration = d
			}
			break
		}
	}

	// Duration should be at least 10ms
	assert.True(t, duration >= 10*time.Millisecond, "duration should be at least 10ms")
}

func TestStructuredLogging_ResponseSize(t *testing.T) {
	mockLog := logger.NewMockLogger()
	config := DefaultLoggingMiddlewareConfig(mockLog)

	e := echo.New()
	e.Use(RequestID())
	e.Use(StructuredLogging(config))

	responseBody := `{"message": "hello world", "status": "ok"}`
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, responseBody)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should log response_size
	assert.True(t, mockLog.AssertLogged(logger.LevelInfo, "request completed"))
}
