package response

import (
	"time"

	"github.com/labstack/echo/v4"
)

// RequestIDHeader is the header key for request IDs
const RequestIDHeader = "X-Request-ID"

// getRequestID extracts request ID from echo context
func getRequestID(c echo.Context) string {
	// Try to get from response header first (set by request_id middleware)
	if rid := c.Response().Header().Get(echo.HeaderXRequestID); rid != "" {
		return rid
	}
	// Try request header
	if rid := c.Request().Header.Get(echo.HeaderXRequestID); rid != "" {
		return rid
	}
	return ""
}

// Envelope is the standard response envelope for all API responses
type Envelope struct {
	Data  interface{}   `json:"data,omitempty"`
	Error *ErrorDetail  `json:"error,omitempty"`
	Meta  *Meta         `json:"meta,omitempty"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Meta contains response metadata
type Meta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// Success creates a success envelope with data
func Success(data interface{}) Envelope {
	return Envelope{
		Data: data,
		Meta: &Meta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// SuccessWithContext creates a success envelope with request ID from context
func SuccessWithContext(c echo.Context, data interface{}) Envelope {
	return Envelope{
		Data: data,
		Meta: &Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// Error creates an error envelope
func Error(code, message string) Envelope {
	return Envelope{
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
		Meta: &Meta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// ErrorWithDetails creates an error envelope with details
func ErrorWithDetails(code, message string, details interface{}) Envelope {
	return Envelope{
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: &Meta{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// ErrorWithContext creates an error envelope with request ID from context
func ErrorWithContext(c echo.Context, code, message string) Envelope {
	return Envelope{
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
		Meta: &Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// ErrorWithContextAndDetails creates an error envelope with request ID and details
func ErrorWithContextAndDetails(c echo.Context, code, message string, details interface{}) Envelope {
	return Envelope{
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: &Meta{
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// ErrorResponse returns an echo error response with JSON envelope.
// This is a convenience function for middleware and handlers.
func ErrorResponse(c echo.Context, statusCode int, code, message string) error {
	return c.JSON(statusCode, ErrorWithContext(c, code, message))
}

// ErrorResponseWithDetails returns an echo error response with details.
func ErrorResponseWithDetails(c echo.Context, statusCode int, code, message string, details interface{}) error {
	return c.JSON(statusCode, ErrorWithContextAndDetails(c, code, message, details))
}
