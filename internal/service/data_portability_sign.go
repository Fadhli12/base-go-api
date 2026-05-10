package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func SignDataPortabilityFile(payload []byte, secret string) string {
	secret = strings.TrimPrefix(secret, "whsec_")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("sha256=%s", signature)
}

func VerifyDataPortabilitySignature(payload []byte, signature string, secret string) bool {
	secret = strings.TrimPrefix(secret, "whsec_")
	expectedMAC := hmac.New(sha256.New, []byte(secret))
	expectedMAC.Write(payload)
	expectedSig := fmt.Sprintf("sha256=%s", hex.EncodeToString(expectedMAC.Sum(nil)))
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) == 1
}

func GenerateFileSignature(payload []byte, secret string, timestamp time.Time) string {
	secret = strings.TrimPrefix(secret, "whsec_")
	signedPayload := fmt.Sprintf("%d.%s", timestamp.Unix(), string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("sha256=%s", signature)
}