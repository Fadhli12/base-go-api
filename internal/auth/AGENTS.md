# Auth Module - JWT Authentication

**Layer:** internal/auth | **Purpose:** JWT tokens, password hashing, refresh token lifecycle

---

## OVERVIEW

JWT-based authentication with HS256 signing. Access tokens (15m) + refresh tokens (30d) with SHA256 hashing for storage. Bcrypt password hashing.

## FILES

| File | Purpose |
|------|---------|
| `jwt.go` | Token generation, parsing, claims |
| `password.go` | Bcrypt hashing and verification |
| `token_service.go` | Refresh token lifecycle |

## JWT TOKENS

### Access Token

```go
// Generate
token, err := auth.GenerateAccessToken(userID, email, secret, expiry)

// Parse and validate
claims, err := auth.GetClaims(tokenString, secret)
// Returns: UserID, Email, Subject, ExpiresAt, IssuedAt

// Middleware extraction
userID := middleware.GetUserID(c)     // uuid.UUID
email := middleware.GetUserEmail(c)   // string
```

### Token Claims

```go
type Claims struct {
    Subject   string    `json:"sub"`  // userID
    Email     string    `json:"email"`
    UserID    string    `json:"user_id"`
    ExpiresAt time.Time `json:"exp"`
    IssuedAt  time.Time `json:"iat"`
}
```

### Signing

- **Method:** HS256 (HMAC-SHA256)
- **Validation:** Rejects non-HMAC methods
- **Secret:** Minimum 32 characters (production)

### Expiry

| Token Type | Default | Config Key |
|------------|---------|------------|
| Access | 15m | `JWT_ACCESS_EXPIRY` |
| Refresh | 720h (30d) | `JWT_REFRESH_EXPIRY` |

## REFRESH TOKENS

### Generation

```go
// Raw token (shown to user once)
rawToken, err := tokenService.GenerateRefreshToken(ctx, userID)
// SHA256 hash stored in DB, raw token returned
```

### Validation

```go
token, err := tokenService.ValidateRefreshToken(ctx, rawToken)
// Checks: exists, not revoked, not expired
```

### Rotation (One-Time Use)

```go
newToken, err := tokenService.RotateRefreshToken(ctx, oldRawToken)
// 1. Validates old token
// 2. Sets RevokedAt on old token
// 3. Generates new token
// 4. Returns new raw token
```

### Revocation

```go
err := tokenService.RevokeRefreshToken(ctx, rawToken)
// Sets RevokedAt = now

err := tokenService.RevokeAllUserTokens(ctx, userID)
// Revokes all tokens for user (logout all sessions)
```

### Entity Methods

```go
token.IsRevoked()  // RevokedAt != nil
token.IsValid()     // !IsRevoked() && time.Now().Before(ExpiresAt)
```

## PASSWORD HASHING

```go
// Hash (cost=12, ~250ms)
hash, err := auth.HashPassword(password)

// Verify
err := auth.VerifyPassword(hash, password)
// Returns nil on match, error otherwise
```

**Security:**
- Bcrypt cost: 12 (configurable)
- Timing-safe comparison
- Automatic salting

## INTEGRATION

### HTTP Handler

```go
// internal/http/handler/auth.go
type Handler struct {
    service *service.AuthService
}

// Endpoints
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
```

### Request DTOs

```go
// internal/http/request/auth.go
type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token" validate:"required"`
}
```

### Middleware

```go
// internal/http/middleware/jwt.go
JWT(config JWTConfig) echo.MiddlewareFunc
// Extracts Bearer token, validates, stores claims in context

// Context key: "user"
claims := c.Get("user").(*auth.Claims)
```

## ERRORS

| Code | HTTP Status | When |
|------|-------------|------|
| `EMAIL_EXISTS` | 409 | Register with existing email |
| `INVALID_CREDENTIALS` | 401 | Wrong email/password |
| `INVALID_REFRESH_TOKEN` | 401 | Expired/revoked/not found |
| `NOT_FOUND` | 404 | Token not found for revocation |

## PATTERNS

### Login Flow

```
1. Validate credentials (email + password)
2. Generate access token (15m)
3. Generate refresh token (30d, hashed in DB)
4. Return both tokens
```

### Refresh Flow

```
1. Validate refresh token (hash lookup, not revoked/expired)
2. Rotate: revoke old, generate new
3. Generate new access token
4. Return new tokens
```

### Logout Flow

```
1. Revoke refresh token by hash
2. Client discards access token
3. Access token expires naturally (15m max)
```

## TESTING

```go
// Unit test pattern
userRepo := new(MockUserRepository)
tokenRepo := new(MockRefreshTokenRepository)
authSvc := service.NewAuthService(userRepo, tokenRepo)

// Test registration
user, err := authSvc.Register(ctx, "test@example.com", "password123")
require.NoError(t, err)
```