package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// GenerateRecoveryCodes generates 8 recovery codes.
// Returns plaintext codes (for user to save) and bcrypt hashes (for storage).
func GenerateRecoveryCodes() ([]string, []string) {
	plaintext := make([]string, 8)
	hashes := make([]string, 8)

	for i := 0; i < 8; i++ {
		code := generateNumericCode(8)
		plaintext[i] = code
		hash, _ := HashRecoveryCode(code)
		hashes[i] = hash
	}

	return plaintext, hashes
}

// generateNumericCode generates a random numeric code of given length.
func generateNumericCode(length int) string {
	bytes := make([]byte, length)
	// Read random bytes - need ceil(length*log2(10)/8) bytes for encoding
	_, _ = rand.Read(bytes)
	// Convert to numeric string by taking modulo 10 for each byte
	var sb strings.Builder
	for _, b := range bytes {
		sb.WriteString(fmt.Sprintf("%d", b%10))
	}
	return sb.String()
}

// HashRecoveryCode hashes a recovery code using bcrypt with cost >= 12.
func HashRecoveryCode(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyRecoveryCode verifies a plaintext code against a bcrypt hash.
func VerifyRecoveryCode(plain string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	return err == nil
}

// CanRegenerateCodes checks if enough time has passed since last regeneration.
// Allows maximum 3 regenerations per 24-hour period.
func CanRegenerateCodes(lastRegenTime time.Time) bool {
	if lastRegenTime.IsZero() {
		return true
	}
	elapsed := time.Since(lastRegenTime)
	return elapsed >= 24*time.Hour
}

// Base64Encode encodes bytes to base64 string.
func Base64Encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}