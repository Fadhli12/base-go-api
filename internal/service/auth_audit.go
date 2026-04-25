package service

import (
	"context"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
)

// AuditAuth logs authentication-related events
// This provides explicit audit logging for auth operations that bypass the audit middleware
func (s *AuthService) AuditAuth(ctx context.Context, actorID uuid.UUID, action, ipAddress, userAgent string, metadata map[string]interface{}) {
	if s.auditService == nil {
		return
	}

	// Log asynchronously to not block the request
	go func() {
		bgCtx := context.Background()
		s.auditService.LogAction(
			bgCtx,
			actorID,
			action,
			domain.AuditResourceAuth,
			"", // No resource ID for auth operations
			nil, // No before state
			metadata,
			ipAddress,
			userAgent,
		)
	}()
}

// AuditLoginSuccess logs a successful login attempt
func (s *AuthService) AuditLoginSuccess(ctx context.Context, userID uuid.UUID, email, ipAddress, userAgent string) {
	s.AuditAuth(ctx, userID, domain.AuditActionLogin, ipAddress, userAgent, map[string]interface{}{
		"email":  email,
		"status": "success",
	})
}

// AuditLoginFailure logs a failed login attempt
// Note: actorID is uuid.Nil for failed logins (user ID unknown)
func (s *AuthService) AuditLoginFailure(ctx context.Context, email, ipAddress, userAgent, reason string) {
	s.AuditAuth(ctx, uuid.Nil, domain.AuditActionLoginFailed, ipAddress, userAgent, map[string]interface{}{
		"email":   email,
		"status":  "failed",
		"reason":  reason,
	})
}

// AuditPasswordResetRequest logs a password reset request
func (s *AuthService) AuditPasswordResetRequest(ctx context.Context, userID uuid.UUID, email, ipAddress, userAgent string) {
	s.AuditAuth(ctx, userID, domain.AuditActionPasswordReset, ipAddress, userAgent, map[string]interface{}{
		"email":  email,
		"status": "requested",
	})
}

// AuditPasswordResetComplete logs a successful password reset
func (s *AuthService) AuditPasswordResetComplete(ctx context.Context, userID uuid.UUID, email, ipAddress, userAgent string) {
	s.AuditAuth(ctx, userID, domain.AuditActionPasswordChange, ipAddress, userAgent, map[string]interface{}{
		"email":  email,
		"status": "completed",
		"method": "password_reset",
	})
}

// MED-004: AuditPotentialTokenReuse logs potential token reuse attacks
// This is called when a revoked token is attempted to be refreshed
func (s *AuthService) AuditPotentialTokenReuse(ctx context.Context, userID, familyID uuid.UUID) {
	s.AuditAuth(ctx, userID, domain.AuditActionTokenReuse, "", "", map[string]interface{}{
		"user_id":   userID.String(),
		"family_id":  familyID.String(),
		"status":     "revoked_family",
		"threat":     "potential_token_reuse_attack",
	})
}