package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	// RequestIDHeader is the HTTP header for request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDContextKey is the key for storing request ID in echo.Context
	RequestIDContextKey = "request_id"
)

// RequestID returns a middleware that generates/extracts request ID
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Request().Header.Get(RequestIDHeader)

			if requestID == "" {
				requestID = uuid.New().String()
			}

			c.Set(RequestIDContextKey, requestID)
			c.Response().Header().Set(RequestIDHeader, requestID)

			return next(c)
		}
	}
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(c echo.Context) string {
	requestID, ok := c.Get(RequestIDContextKey).(string)
	if !ok {
		return ""
	}
	return requestID
}
