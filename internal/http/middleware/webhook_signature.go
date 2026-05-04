package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SignWebhookPayload generates an HMAC-SHA256 signature for a webhook payload.
// Returns signature header in format: t=<timestamp>,v1=<hex_signature>
// The secret should have the "whsec_" prefix stripped before use.
func SignWebhookPayload(secret string, timestamp int64, body []byte) string {
	// Strip whsec_ prefix if present
	secret = strings.TrimPrefix(secret, "whsec_")

	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}

// VerifyWebhookSignature verifies an HMAC-SHA256 signature from a webhook delivery.
// sigHeader format: t=1683456789,v1=abcdef1234567890abcdef1234567890
// The secret should have the "whsec_" prefix stripped before use.
// toleranceSeconds controls how old timestamps are accepted (replay protection).
func VerifyWebhookSignature(secret string, sigHeader string, body []byte, toleranceSeconds int64) bool {
	// Strip whsec_ prefix if present
	secret = strings.TrimPrefix(secret, "whsec_")

	timestamp, signature, err := ParseSignatureHeader(sigHeader)
	if err != nil {
		return false
	}

	// Check timestamp tolerance (replay protection)
	if toleranceSeconds > 0 {
		now := time.Now().Unix()
		if now-timestamp > toleranceSeconds || timestamp-now > toleranceSeconds {
			return false
		}
	}

	// Compute expected signature
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}

// ParseSignatureHeader extracts timestamp and v1 signature from the header.
// Returns (timestamp, signature, error).
func ParseSignatureHeader(sigHeader string) (int64, string, error) {
	var timestamp int64
	var signature string
	parts := strings.Split(sigHeader, ",")
	for _, p := range parts {
		if after, ok := strings.CutPrefix(p, "t="); ok {
			ts, err := strconv.ParseInt(after, 10, 64)
			if err != nil {
				return 0, "", err
			}
			timestamp = ts
		} else if after, ok := strings.CutPrefix(p, "v1="); ok {
			signature = after
		}
	}

	if timestamp == 0 || signature == "" {
		return 0, "", errors.New("invalid signature header")
	}

	return timestamp, signature, nil
}
