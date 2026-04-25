package service

import (
	"sync/atomic"
)

// AuthMetrics tracks authentication-related metrics
// MED-006: Auth observability for production monitoring
type AuthMetrics struct {
	// Login metrics
	LoginSuccess      atomic.Int64 // Successful login attempts
	LoginFailed       atomic.Int64 // Failed login attempts (invalid credentials)
	LoginRateLimited  atomic.Int64 // Login attempts blocked by rate limiting

	// Password reset metrics
	PasswordResetRequested  atomic.Int64 // Password reset requests
	PasswordResetCompleted  atomic.Int64 // Successful password resets
	PasswordResetFailed     atomic.Int64 // Failed password reset attempts (invalid token)

	// Token metrics
	TokenRefreshSuccess atomic.Int64 // Successful token refreshes
	TokenRefreshFailed  atomic.Int64 // Failed token refreshes (invalid/expired token)
	TokenReuseDetected  atomic.Int64 // Potential token reuse attacks detected

	// Session metrics
	ActiveSessions      atomic.Int64 // Currently active sessions
	SessionRevoked      atomic.Int64 // Sessions revoked by user
	SessionsExpired     atomic.Int64 // Sessions expired naturally
}

// Global auth metrics instance
var authMetrics = &AuthMetrics{}

// GetAuthMetrics returns the global auth metrics instance
func GetAuthMetrics() *AuthMetrics {
	return authMetrics
}

// Increment helpers for auth metrics
func (m *AuthMetrics) IncrementLoginSuccess() {
	m.LoginSuccess.Add(1)
}

func (m *AuthMetrics) IncrementLoginFailed() {
	m.LoginFailed.Add(1)
}

func (m *AuthMetrics) IncrementLoginRateLimited() {
	m.LoginRateLimited.Add(1)
}

func (m *AuthMetrics) IncrementPasswordResetRequested() {
	m.PasswordResetRequested.Add(1)
}

func (m *AuthMetrics) IncrementPasswordResetCompleted() {
	m.PasswordResetCompleted.Add(1)
}

func (m *AuthMetrics) IncrementPasswordResetFailed() {
	m.PasswordResetFailed.Add(1)
}

func (m *AuthMetrics) IncrementTokenRefreshSuccess() {
	m.TokenRefreshSuccess.Add(1)
}

func (m *AuthMetrics) IncrementTokenRefreshFailed() {
	m.TokenRefreshFailed.Add(1)
}

func (m *AuthMetrics) IncrementTokenReuseDetected() {
	m.TokenReuseDetected.Add(1)
}

func (m *AuthMetrics) IncrementSessionRevoked() {
	m.SessionRevoked.Add(1)
}

func (m *AuthMetrics) SetActiveSessions(count int64) {
	m.ActiveSessions.Store(count)
}

// Snapshot returns a snapshot of current auth metrics
func (m *AuthMetrics) Snapshot() AuthMetricsSnapshot {
	return AuthMetricsSnapshot{
		LoginSuccess:             m.LoginSuccess.Load(),
		LoginFailed:              m.LoginFailed.Load(),
		LoginRateLimited:         m.LoginRateLimited.Load(),
		PasswordResetRequested:   m.PasswordResetRequested.Load(),
		PasswordResetCompleted:   m.PasswordResetCompleted.Load(),
		PasswordResetFailed:      m.PasswordResetFailed.Load(),
		TokenRefreshSuccess:      m.TokenRefreshSuccess.Load(),
		TokenRefreshFailed:       m.TokenRefreshFailed.Load(),
		TokenReuseDetected:       m.TokenReuseDetected.Load(),
		ActiveSessions:           m.ActiveSessions.Load(),
		SessionRevoked:           m.SessionRevoked.Load(),
	}
}

// AuthMetricsSnapshot represents a point-in-time snapshot of auth metrics
type AuthMetricsSnapshot struct {
	LoginSuccess             int64 `json:"login_success"`
	LoginFailed              int64 `json:"login_failed"`
	LoginRateLimited         int64 `json:"login_rate_limited"`
	PasswordResetRequested   int64 `json:"password_reset_requested"`
	PasswordResetCompleted   int64 `json:"password_reset_completed"`
	PasswordResetFailed      int64 `json:"password_reset_failed"`
	TokenRefreshSuccess      int64 `json:"token_refresh_success"`
	TokenRefreshFailed       int64 `json:"token_refresh_failed"`
	TokenReuseDetected       int64 `json:"token_reuse_detected"`
	ActiveSessions           int64 `json:"active_sessions"`
	SessionRevoked           int64 `json:"session_revoked"`
}

// Reset resets all metrics to zero (useful for testing)
func (m *AuthMetrics) Reset() {
	m.LoginSuccess.Store(0)
	m.LoginFailed.Store(0)
	m.LoginRateLimited.Store(0)
	m.PasswordResetRequested.Store(0)
	m.PasswordResetCompleted.Store(0)
	m.PasswordResetFailed.Store(0)
	m.TokenRefreshSuccess.Store(0)
	m.TokenRefreshFailed.Store(0)
	m.TokenReuseDetected.Store(0)
	m.ActiveSessions.Store(0)
	m.SessionRevoked.Store(0)
}