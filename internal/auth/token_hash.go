package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken generates a SHA256 hash for token storage
// Used for refresh tokens and password reset tokens for fast comparison
// Unlike bcrypt, SHA256 is appropriate for randomly-generated tokens (not passwords)
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// VerifyToken compares a token against its stored hash
// Returns true if the token matches the hash
func VerifyToken(token, hash string) bool {
	return HashToken(token) == hash
}