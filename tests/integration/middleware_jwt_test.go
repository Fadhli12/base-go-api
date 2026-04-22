//go:build integration
// +build integration

package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/auth"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// testJWTSecret is the secret used for signing tokens in tests
const testJWTSecret = "test-secret-key-for-jwt-testing-min-32-ch"

// TestJWTMiddleware_NoToken validates that requests without authorization
// header are rejected with 401 Unauthorized.
func TestJWTMiddleware_NoToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 for missing token")
	require.Contains(t, rec.Body.String(), "Missing authorization header", "Should indicate missing header")
}

// TestJWTMiddleware_ValidToken validates that requests with a valid Bearer token
// pass through the middleware and user context is available.
func TestJWTMiddleware_ValidToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "test@example.com"

	// Generate a valid token
	token, err := auth.GenerateAccessToken(userID, email, testJWTSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate valid token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		claims, err := middleware.GetUserClaims(c)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{
			"user_id": claims.UserID,
			"email":   claims.Email,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for valid token")
	require.Contains(t, rec.Body.String(), userID, "Should contain user ID")
	require.Contains(t, rec.Body.String(), email, "Should contain email")
}

// TestJWTMiddleware_ExpiredToken validates that requests with an expired token
// are rejected with 401 Unauthorized.
func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "test@example.com"

	// Generate an expired token (negative duration makes it already expired)
	token, err := auth.GenerateAccessToken(userID, email, testJWTSecret, -1*time.Hour)
	require.NoError(t, err, "Should generate expired token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 for expired token")
	require.Contains(t, rec.Body.String(), "Invalid or expired token", "Should indicate invalid/expired token")
}

// TestJWTMiddleware_MalformedToken validates that requests with a malformed token
// are rejected with 401 Unauthorized.
func TestJWTMiddleware_MalformedToken(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Test with malformed token (not a valid JWT format)
	malformedToken := "not-a.valid.jwt.token"

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+malformedToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 for malformed token")
	require.Contains(t, rec.Body.String(), "Invalid or expired token", "Should indicate invalid token")
}

// TestJWTMiddleware_InvalidSignature validates that requests with a token signed
// with a different secret are rejected with 401 Unauthorized.
func TestJWTMiddleware_InvalidSignature(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "test@example.com"

	// Generate token with a different secret
	wrongSecret := "wrong-secret-key-for-signing-the-token-min-32"
	token, err := auth.GenerateAccessToken(userID, email, wrongSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate token with wrong secret")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret // Use correct secret in middleware
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 for invalid signature")
	require.Contains(t, rec.Body.String(), "Invalid or expired token", "Should indicate invalid token")
}

// TestJWTMiddleware_InvalidHeaderFormat validates that requests with an invalid
// Authorization header format are rejected with 401 Unauthorized.
func TestJWTMiddleware_InvalidHeaderFormat(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	tests := []struct {
		name        string
		authHeader  string
		wantMessage string
	}{
		{
			name:        "missing bearer prefix",
			authHeader:  "sometokenvalue",
			wantMessage: "Invalid authorization header format",
		},
		{
			name:        "wrong auth scheme",
			authHeader:  "Basic sometokenvalue",
			wantMessage: "Invalid authorization header format",
		},
		{
			name:        "bearer without token",
			authHeader:  "Bearer ",
			wantMessage: "Missing bearer token",
		},
		{
			name:        "bearer with whitespace only",
			authHeader:  "Bearer   ",
			wantMessage: "Missing bearer token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401")
			require.Contains(t, rec.Body.String(), tt.wantMessage, "Should indicate correct error")
		})
	}
}

// TestGetUserClaims_ExtractsCorrectUserInfo validates that GetUserClaims helper
// correctly extracts user information from the context.
func TestGetUserClaims_ExtractsCorrectUserInfo(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "testuser@example.com"

	// Generate a valid token
	token, err := auth.GenerateAccessToken(userID, email, testJWTSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate valid token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	var extractedClaims *auth.Claims
	e.GET("/protected", func(c echo.Context) error {
		claims, err := middleware.GetUserClaims(c)
		if err != nil {
			return err
		}
		extractedClaims = claims
		return c.JSON(http.StatusOK, claims)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for valid token")
	require.NotNil(t, extractedClaims, "Claims should be extracted")
	require.Equal(t, userID, extractedClaims.UserID, "Should have correct user ID")
	require.Equal(t, email, extractedClaims.Email, "Should have correct email")
	require.Equal(t, userID, extractedClaims.Subject, "Subject should match user ID")
}

// TestGetUserClaims_NoClaimsInContext validates that GetUserClaims returns an error
// when no claims are stored in the context.
func TestGetUserClaims_NoClaimsInContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test without any claims in context
	claims, err := middleware.GetUserClaims(c)
	require.Error(t, err, "Should return error when no claims in context")
	require.Nil(t, claims, "Claims should be nil")
	require.ErrorIs(t, err, middleware.ErrNoUserInContext, "Should return ErrNoUserInContext")
}

// TestGetUserID_ValidatesUUID validates that GetUserID correctly parses the UUID
// from the claims.
func TestGetUserID_ValidatesUUID(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New()
	email := "uuidtest@example.com"

	// Generate a valid token
	token, err := auth.GenerateAccessToken(userID.String(), email, testJWTSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate valid token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	var extractedUserID uuid.UUID
	e.GET("/protected", func(c echo.Context) error {
		userID, err := middleware.GetUserID(c)
		if err != nil {
			return err
		}
		extractedUserID = userID
		return c.JSON(http.StatusOK, map[string]string{"user_id": userID.String()})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for valid token")
	require.Equal(t, userID, extractedUserID, "Should extract correct user ID as UUID")
}

// TestGetUserEmail_ExtractsCorrectly validates that GetUserEmail correctly extracts
// the email from the claims.
func TestGetUserEmail_ExtractsCorrectly(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "emailtest@example.com"

	// Generate a valid token
	token, err := auth.GenerateAccessToken(userID, email, testJWTSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate valid token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	var extractedEmail string
	e.GET("/protected", func(c echo.Context) error {
		email, err := middleware.GetUserEmail(c)
		if err != nil {
			return err
		}
		extractedEmail = email
		return c.JSON(http.StatusOK, map[string]string{"email": email})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for valid token")
	require.Equal(t, email, extractedEmail, "Should extract correct email")
}

// TestJWTMiddleware_SkipperFunction validates that the skipper function allows
// certain routes to bypass JWT authentication.
func TestJWTMiddleware_SkipperFunction(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	// Skip authentication for /health endpoint
	config.Skipper = func(c echo.Context) bool {
		return c.Request().URL.Path == "/health"
	}
	e.Use(middleware.JWT(config))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Test skipped endpoint without token
	t.Run("skipped endpoint without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for skipped endpoint")
		require.Contains(t, rec.Body.String(), "healthy", "Should return healthy status")
	})

	// Test protected endpoint without token
	t.Run("protected endpoint without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, "Should return 401 for protected endpoint")
	})
}

// TestGetClaims_LegacyHelper tests the GetClaims helper function for backward compatibility.
func TestGetClaims_LegacyHelper(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	userID := uuid.New().String()
	email := "legacy@example.com"

	token, err := auth.GenerateAccessToken(userID, email, testJWTSecret, 15*time.Minute)
	require.NoError(t, err, "Should generate valid token")

	e := echo.New()
	config := middleware.DefaultJWTConfig()
	config.Secret = testJWTSecret
	e.Use(middleware.JWT(config))

	e.GET("/protected", func(c echo.Context) error {
		claims := middleware.GetClaims(c)
		if claims == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "claims not found")
		}
		return c.JSON(http.StatusOK, map[string]string{
			"user_id": claims.UserID,
			"email":   claims.Email,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Should return 200 for valid token")
	require.Contains(t, rec.Body.String(), userID, "Should contain user ID")
	require.Contains(t, rec.Body.String(), email, "Should contain email")
}

// TestGetClaims_NoClaims tests that GetClaims returns nil when no claims exist.
func TestGetClaims_NoClaims(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	claims := middleware.GetClaims(c)
	require.Nil(t, claims, "Should return nil when no claims in context")
}

// TestJWTWithSkipper tests the JWTWithSkipper convenience function.
func TestJWTWithSkipper(t *testing.T) {
	suite := SetupIntegrationTest(t)
	defer suite.TeardownTest(t)

	e := echo.New()
	// Use JWTWithSkipper convenience function
	e.Use(middleware.JWTWithSkipper(testJWTSecret, func(c echo.Context) bool {
		return strings.HasPrefix(c.Request().URL.Path, "/public/")
	}))

	e.GET("/public/info", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "public"})
	})

	e.GET("/protected", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "protected"})
	})

	// Test public path without token
	t.Run("public path without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public/info", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "Public path should be accessible")
	})

	// Test protected path without token
	t.Run("protected path without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, "Protected path should require auth")
	})
}