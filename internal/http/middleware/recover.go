package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v4"
)

// Recover returns a middleware that recovers from panics
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					requestID := GetRequestID(c)

					slog.Error("panic recovered",
						slog.String("request_id", requestID),
						slog.Any("panic", r),
						slog.String("stack", string(debug.Stack())),
					)

					err := c.JSON(http.StatusInternalServerError, map[string]interface{}{
						"error": map[string]string{
							"code":    "INTERNAL_ERROR",
							"message": "Internal server error",
						},
						"meta": map[string]string{
							"request_id": requestID,
						},
					})
					if err != nil {
						fmt.Printf("failed to write error response: %v\n", err)
					}
				}
			}()

			return next(c)
		}
	}
}
