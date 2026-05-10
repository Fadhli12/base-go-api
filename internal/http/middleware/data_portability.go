package middleware

import (
	"fmt"
	"net/http"

	"github.com/example/go-api-base/internal/service"
	"github.com/labstack/echo/v4"
)

func ExportRateLimit(limiter service.DataPortabilityRateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := getUserID(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "Authentication required",
					},
				})
			}

			allowed, err := limiter.Allow(c.Request().Context(), userID, service.ActionExport)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": map[string]string{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to check rate limit",
					},
				})
			}

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMITED",
						"message": "export rate limit exceeded",
					},
				})
			}

			return next(c)
		}
	}
}

func ImportRateLimit(limiter service.DataPortabilityRateLimiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := getUserID(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "Authentication required",
					},
				})
			}

			allowed, err := limiter.Allow(c.Request().Context(), userID, service.ActionImport)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": map[string]string{
						"code":    "INTERNAL_ERROR",
						"message": "Failed to check rate limit",
					},
				})
			}

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": map[string]string{
						"code":    "RATE_LIMITED",
						"message": "import rate limit exceeded",
					},
				})
			}

			return next(c)
		}
	}
}

func ImportFileSizeLimit(maxSize int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := c.Request().ParseMultipartForm(maxSize); err != nil {
				return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
					"error": map[string]string{
						"code":    "PAYLOAD_TOO_LARGE",
						"message": "import file exceeds size limit",
					},
				})
			}

			if c.Request().ContentLength > maxSize {
				return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
					"error": map[string]string{
						"code":    "PAYLOAD_TOO_LARGE",
						"message": "import file exceeds size limit",
					},
				})
			}

			return next(c)
		}
	}
}

func getUserID(c echo.Context) (string, error) {
	claims, err := GetUserClaims(c)
	if err != nil {
		return "", err
	}

	if claims.UserID == "" {
		return "", fmt.Errorf("user ID not found in claims")
	}

	return claims.UserID, nil
}