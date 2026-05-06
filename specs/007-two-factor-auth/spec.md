# Feature Specification: Two-Factor Authentication (2FA)

**Feature Branch**: `007-two-factor-auth`
**Created**: 2026-05-05
**Status**: Draft
**Input**: User description: "Implement 2FA feature for this project"

## User Scenarios & Testing

### User Story 1 - Enable 2FA (Priority: P1)

**As a** user, **I want to** enable two-factor authentication on my account **so that** I can add an extra layer of security beyond my password.

**Why this priority**: 2FA is the primary feature being implemented - without it, nothing else matters.

**Independent Test**: User navigates to security settings, scans QR code with authenticator app, verifies with TOTP code, and 2FA becomes active.

**Acceptance Scenarios**:

1. **Given** I am logged in without 2FA, **When** I initiate 2FA setup, **Then** I receive a TOTP secret and QR code URL
2. **Given** I have a pending 2FA setup with secret, **When** I verify with a valid TOTP code, **Then** 2FA is enabled and recovery codes are shown
3. **Given** I initiate 2FA setup but don't verify, **When** I try to skip verification, **Then** 2FA remains disabled
4. **Given** I already have 2FA enabled, **When** I try to initiate setup again, **Then** I receive an error message

---

### User Story 2 - Login with 2FA (Priority: P1)

**As a** user with 2FA enabled, **I want to** complete login by entering my TOTP code **so that** I can access my account.

**Why this priority**: The core 2FA login flow is the main security benefit.

**Independent Test**: User enters email/password, receives 2FA verification required response, enters valid TOTP code, and receives final access token.

**Acceptance Scenarios**:

1. **Given** I have 2FA enabled, **When** I login with correct credentials, **Then** I receive a 2FA verification required response with a temporary session
2. **Given** I have 2FA enabled and a pending verification session, **When** I enter a valid TOTP code, **Then** I receive access and refresh tokens
3. **Given** I have 2FA enabled and a pending verification session, **When** I enter an invalid TOTP code, **Then** I receive an error and must retry
4. **Given** I have 2FA enabled and a pending verification session, **When** I enter a valid recovery code, **Then** I receive access and refresh tokens and the recovery code is consumed

---

### User Story 3 - Disable 2FA (Priority: P2)

**As a** user with 2FA enabled, **I want to** disable 2FA when I no longer need it **so that** I can use password-only login.

**Why this priority**: Users should have the option to disable if they prefer.

**Independent Test**: User requests to disable 2FA, provides valid TOTP code, and 2FA becomes disabled.

**Acceptance Scenarios**:

1. **Given** I have 2FA enabled, **When** I request to disable with a valid TOTP code, **Then** 2FA is disabled and recovery codes are invalidated
2. **Given** I have 2FA enabled, **When** I request to disable with an invalid TOTP code, **Then** 2FA remains enabled and I receive an error

---

### User Story 4 - View 2FA Status (Priority: P3)

**As a** user, **I want to** see my current 2FA status **so that** I know if it's enabled.

**Why this priority**: Lower priority - status visibility is helpful but not critical.

**Acceptance Scenarios**:

1. **Given** I am logged in, **When** I check my 2FA status, **Then** I see whether 2FA is enabled, pending, or disabled

---

### Edge Cases

- **Clock drift with authenticator app**: System tolerates ±1 time step (±30 seconds) per FR-010. If TOTP validation still fails, user can use a recovery code.
- **All recovery codes used**: User must regenerate new recovery codes via `POST /api/v1/auth/2fa/recovery-codes` (requires valid TOTP verification). Existing unused codes are invalidated. Limit: 3 regenerations per 24 hours.
- **2FA session expiration (5 minutes)**: After expiry, the pending token is invalidated. User must restart login flow from email/password step. Expired sessions are cleaned up by the Refresh() method rejecting tokens with `is_2fa_pending=true` that have passed their 5-minute window.
- **Concurrent login attempts with 2FA**: Each new login attempt with 2FA invalidates any existing pending 2FA session for that user. Only one pending session per user exists at any time. This prevents session confusion and replay attacks.
- **TOTP secret decryption failure**: If AES-GCM decryption fails (corrupted data, wrong key), system logs a CRITICAL audit event and returns an internal server error. User must disable and re-enable 2FA to generate a new encrypted secret.

## Requirements

### Functional Requirements

- **FR-001**: System MUST support TOTP (Time-based One-Time Password) algorithm per RFC 6238
- **FR-002**: System MUST be compatible with Google Authenticator and Authy apps
- **FR-003**: Users MUST be able to enable 2FA by verifying their identity with a TOTP code
- **FR-004**: Users MUST be able to disable 2FA by providing a valid TOTP code
- **FR-005**: System MUST provide recovery codes when 2FA is enabled (8 codes, 8 digits each)
- **FR-006**: Recovery codes MUST be one-time use and bcrypt hashed for storage
- **FR-007**: TOTP secret MUST be encrypted at rest (AES-GCM)
- **FR-008**: Login flow with 2FA enabled MUST return a temporary session requiring TOTP verification
- **FR-009**: 2FA session MUST expire after 5 minutes
- **FR-010**: System MUST validate TOTP codes with a ±1 time step tolerance (for clock drift)
- **FR-011**: All 2FA operations (enable, disable, verify) MUST be audit logged
- **FR-012**: 2FA is OPTIONAL - existing users can continue without 2FA
- **FR-013**: System MUST NOT allow enabling 2FA without proper verification
- **FR-014**: System MUST return helpful error messages for invalid codes

### Non-Functional Requirements

- **NFR-001**: TOTP validation latency MUST be < 50ms
- **NFR-002**: System MUST handle 1000 concurrent 2FA verifications
- **NFR-003**: QR code URL format MUST be standard `otpauth://totp/` format
- **NFR-004**: Recovery codes MUST NOT be stored in plaintext

## Key Entities

- **User** (extended): Existing user entity with new 2FA fields (TwoFactorEnabled, TwoFactorSecret, TwoFactorStatus, TwoFactorVerifiedAt)
- **TwoFactorRecoveryCode**: Stores hashed recovery codes linked to user
- **RefreshToken** (extended): Optional metadata field for 2FA pending sessions

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can complete 2FA setup in under 2 minutes
- **SC-002**: TOTP validation passes on first attempt with authenticator app (within tolerance)
- **SC-003**: Recovery codes work exactly once, then are invalidated
- **SC-004**: Login flow with 2FA completes in under 5 seconds
- **SC-005**: 2FA can be disabled and re-enabled without data loss
- **SC-006**: All 2FA operations are logged in audit trail

## Assumptions

- Users have access to an authenticator app (Google Authenticator, Authy, etc.)
- Server time is synchronized via NTP (TOTP is time-based)
- Encryption key for TOTP secret is stored securely in configuration
- 2FA is per-user, not per-organization
- Recovery codes are shown exactly once at setup time
