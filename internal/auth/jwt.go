package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims
type Claims struct {
	jwt.RegisteredClaims
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

// GenerateAccessToken generates a new JWT access token
func GenerateAccessToken(userID, email string, secret string, expiry time.Duration) (string, error) {
	return GenerateAccessTokenWithClaims(userID, email, secret, expiry, "", "")
}

// GenerateAccessTokenWithClaims generates a new JWT access token with issuer and audience claims
func GenerateAccessTokenWithClaims(userID, email, secret string, expiry time.Duration, issuer, audience string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateTokenID(),
		},
		Email:  email,
		UserID: userID,
	}

	// Add issuer claim if configured (HIGH-003)
	if issuer != "" {
		claims.Issuer = issuer
	}

	// Add audience claim if configured (HIGH-003)
	if audience != "" {
		claims.Audience = []string{audience}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ParseToken parses and validates a JWT token string
func ParseToken(tokenString, secret string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return token, nil
}

// GetClaims extracts claims from a JWT token string
func GetClaims(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// generateTokenID generates a unique token ID
func generateTokenID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
