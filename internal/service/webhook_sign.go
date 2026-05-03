package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// signWebhookPayload generates an HMAC-SHA256 signature for a webhook payload.
// Returns signature header in format: t=<timestamp>,v1=<hex_signature>
// The secret should have the "whsec_" prefix stripped before use.
// This is a local copy to avoid circular imports with internal/http/middleware.
func signWebhookPayload(secret string, timestamp int64, body []byte) string {
	// Strip whsec_ prefix if present
	secret = strings.TrimPrefix(secret, "whsec_")

	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}