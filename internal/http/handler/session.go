package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// SessionResponse represents an active user session
type SessionResponse struct {
	ID         uuid.UUID `json:"id"`
	DeviceName string    `json:"device_name,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	CreatedAt  string    `json:"created_at"`
	ExpiresAt  string    `json:"expires_at"`
	IsCurrent  bool      `json:"is_current"` // Whether this is the current session
}

// ListSessions returns all active sessions for the authenticated user
func (h *AuthHandler) ListSessions(c echo.Context) error {
	// Get authenticated user ID from context
	userID, ok := c.Get("userID").(uuid.UUID)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	// Get all active sessions for the user
	sessions, err := h.authService.GetActiveSessions(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to retrieve sessions")
	}

	// Get current session token from context
	currentTokenID, _ := c.Get("tokenID").(uuid.UUID)

	// Convert to response format
	response := make([]SessionResponse, len(sessions))
	for i, session := range sessions {
		response[i] = SessionResponse{
			ID:         session.ID,
			DeviceName: session.DeviceName,
			UserAgent:  session.UserAgent,
			IPAddress:  session.IPAddress,
			CreatedAt:  session.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt:  session.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			IsCurrent:  session.ID == currentTokenID,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": response,
	})
}

// RevokeSession revokes a specific session by ID
func (h *AuthHandler) RevokeSession(c echo.Context) error {
	// Get authenticated user ID from context
	userID, ok := c.Get("userID").(uuid.UUID)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	// Get session ID from URL
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	// Get current session token from context
	currentTokenID, _ := c.Get("tokenID").(uuid.UUID)

	// Prevent revoking current session (use logout for that)
	if sessionID == currentTokenID {
		return echo.NewHTTPError(http.StatusBadRequest, "Cannot revoke current session. Use logout instead.")
	}

	// Revoke the session
	if err := h.authService.RevokeSession(c.Request().Context(), userID, sessionID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to revoke session")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]string{
			"message": "Session revoked successfully",
		},
	})
}

// RevokeAllOtherSessions revokes all sessions except the current one
func (h *AuthHandler) RevokeAllOtherSessions(c echo.Context) error {
	// Get authenticated user ID from context
	userID, ok := c.Get("userID").(uuid.UUID)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	// Get current session token from context
	currentTokenID, ok := c.Get("tokenID").(uuid.UUID)
	if !ok {
		// If we don't have the token ID, revoke all sessions
		currentTokenID = uuid.Nil
	}

	// Revoke all other sessions
	count, err := h.authService.RevokeAllOtherSessions(c.Request().Context(), userID, currentTokenID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to revoke sessions")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"message":       "All other sessions revoked successfully",
			"revoked_count": count,
		},
	})
}