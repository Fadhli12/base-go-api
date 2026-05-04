package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Constants for signed URL generation
const (
	// DefaultExpiry is the default signed URL expiry duration
	DefaultExpiry = time.Hour

	// MaxExpiry is the maximum allowed signed URL expiry duration
	MaxExpiry = 24 * time.Hour

	// MinExpiry is the minimum allowed signed URL expiry duration
	MinExpiry = time.Minute

	// SignatureParam is the query parameter name for the signature
	SignatureParam = "sig"

	// ExpiresParam is the query parameter name for the expiry timestamp
	ExpiresParam = "expires"
)

// Signer provides HMAC-SHA256 signed URL generation and validation
type Signer struct {
	secret []byte
}

// NewSigner creates a new Signer with the given secret
func NewSigner(secret string) *Signer {
	return &Signer{
		secret: []byte(secret),
	}
}

// SignedURL represents a generated signed URL with metadata
type SignedURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int64     `json:"expires_in"`
}

// Generate creates a signed URL for the given resource
// The signature is computed as: HMAC-SHA256(secret, resourceID + ":" + path + ":" + expiry)
//
// Parameters:
//   - resourceID: The unique identifier of the resource (e.g., media ID)
//   - path: The storage path for the resource
//   - expiry: How long the URL should be valid
//
// Returns the signed URL and expiry time, or an error if expiry is invalid
func (s *Signer) Generate(resourceID string, path string, expiry time.Duration) (*SignedURL, error) {
	// Validate expiry
	if expiry < MinExpiry {
		expiry = DefaultExpiry
	}
	if expiry > MaxExpiry {
		return nil, fmt.Errorf("expiry exceeds maximum allowed duration of %v", MaxExpiry)
	}

	// Calculate expiry timestamp
	expiresAt := time.Now().Add(expiry)
	expiresUnix := expiresAt.Unix()

	// Generate signature (use signBasic for download validation compatibility)
	signature := s.signBasic(resourceID, expiresUnix)

	// Build URL
	u := &url.URL{
		Path: fmt.Sprintf("/media/%s/download", resourceID),
		RawQuery: fmt.Sprintf("%s=%d&%s=%s",
			ExpiresParam, expiresUnix,
			SignatureParam, signature),
	}

	return &SignedURL{
		URL:       u.String(),
		ExpiresAt: expiresAt,
		ExpiresIn: int64(expiry.Seconds()),
	}, nil
}

// GenerateWithConversion creates a signed URL for a specific conversion of the resource
//
// Parameters:
//   - resourceID: The unique identifier of the resource
//   - path: The storage path for the resource
//   - conversion: The conversion name (e.g., "thumbnail", "preview")
//   - expiry: How long the URL should be valid
func (s *Signer) GenerateWithConversion(resourceID string, path string, conversion string, expiry time.Duration) (*SignedURL, error) {
	// Validate expiry
	if expiry < MinExpiry {
		expiry = DefaultExpiry
	}
	if expiry > MaxExpiry {
		return nil, fmt.Errorf("expiry exceeds maximum allowed duration of %v", MaxExpiry)
	}

	// Calculate expiry timestamp
	expiresAt := time.Now().Add(expiry)
	expiresUnix := expiresAt.Unix()

	// Generate signature (include conversion in signature for added security)
	signature := s.signWithConversion(resourceID, path, conversion, expiresUnix)

	// Build URL with conversion parameter
	u := &url.URL{
		Path: fmt.Sprintf("/media/%s/download", resourceID),
		RawQuery: fmt.Sprintf("conversion=%s&%s=%d&%s=%s",
			url.QueryEscape(conversion),
			ExpiresParam, expiresUnix,
			SignatureParam, signature),
	}

	return &SignedURL{
		URL:       u.String(),
		ExpiresAt: expiresAt,
		ExpiresIn: int64(expiry.Seconds()),
	}, nil
}

// Validate checks if a signed URL is valid and not expired
//
// Parameters:
//   - resourceID: The resource ID from the URL path
//   - signature: The signature query parameter value
//   - expires: The expires query parameter value (Unix timestamp)
//   - path: The storage path for the resource (optional, can be empty for basic validation)
//
// Returns true if the signature is valid and the URL has not expired
func (s *Signer) Validate(resourceID string, signature string, expires int64, path string) bool {
	// Check if URL has expired
	if time.Now().Unix() > expires {
		return false
	}

	// Regenerate expected signature
	var expectedSig string
	if path != "" {
		expectedSig = s.sign(resourceID, path, expires)
	} else {
		expectedSig = s.signBasic(resourceID, expires)
	}

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// ValidateWithConversion checks if a signed URL with conversion is valid
//
// Parameters:
//   - resourceID: The resource ID from the URL path
//   - signature: The signature query parameter value
//   - expires: The expires query parameter value (Unix timestamp)
//   - path: The storage path for the resource
//   - conversion: The conversion name from the query parameter
func (s *Signer) ValidateWithConversion(resourceID string, signature string, expires int64, path string, conversion string) bool {
	// Check if URL has expired
	if time.Now().Unix() > expires {
		return false
	}

	// Regenerate expected signature
	expectedSig := s.signWithConversion(resourceID, path, conversion, expires)

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// ParseExpiry parses an expiry string (Unix timestamp) into int64
func ParseExpiry(expiresStr string) (int64, error) {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expiry timestamp: %w", err)
	}
	return expires, nil
}

// IsExpired checks if a Unix timestamp has passed
func IsExpired(expires int64) bool {
	return time.Now().Unix() > expires
}

// sign generates an HMAC-SHA256 signature for the given components
// Format: HMAC(secret, resourceID + ":" + path + ":" + expiry)
func (s *Signer) sign(resourceID string, path string, expires int64) string {
	payload := fmt.Sprintf("%s:%s:%d", resourceID, path, expires)
	return s.hmac(payload)
}

// signBasic generates an HMAC-SHA256 signature with just resource ID and expiry
// Format: HMAC(secret, resourceID + ":" + expiry)
func (s *Signer) signBasic(resourceID string, expires int64) string {
	payload := fmt.Sprintf("%s:%d", resourceID, expires)
	return s.hmac(payload)
}

// signWithConversion generates an HMAC-SHA256 signature including conversion name
// Format: HMAC(secret, resourceID + ":" + path + ":" + conversion + ":" + expiry)
func (s *Signer) signWithConversion(resourceID string, path string, conversion string, expires int64) string {
	payload := fmt.Sprintf("%s:%s:%s:%d", resourceID, path, conversion, expires)
	return s.hmac(payload)
}

// hmac computes HMAC-SHA256 of the payload using the secret
func (s *Signer) hmac(payload string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// SignedURLGenerator is an interface for generating signed URLs
// This abstraction allows for easier testing and different implementations
type SignedURLGenerator interface {
	Generate(resourceID string, path string, expiry time.Duration) (*SignedURL, error)
	GenerateWithConversion(resourceID string, path string, conversion string, expiry time.Duration) (*SignedURL, error)
	Validate(resourceID string, signature string, expires int64, path string) bool
	ValidateWithConversion(resourceID string, signature string, expires int64, path string, conversion string) bool
}

// Ensure Signer implements SignedURLGenerator
var _ SignedURLGenerator = (*Signer)(nil)

// SignerConfig holds configuration for the signer
type SignerConfig struct {
	Secret        string
	DefaultExpiry time.Duration
	MaxExpiry     time.Duration
}

// NewSignerFromConfig creates a new Signer from configuration
func NewSignerFromConfig(config SignerConfig) *Signer {
	return NewSigner(config.Secret)
}

// GenerateSignature is a convenience function for generating a signature
// Use this when you need a signature without the full URL
func GenerateSignature(secret string, resourceID string, expires int64) string {
	s := NewSigner(secret)
	return s.signBasic(resourceID, expires)
}

// ValidateSignature is a convenience function for validating a signature
// Use this when you need to validate without creating a Signer instance
func ValidateSignature(secret string, resourceID string, signature string, expires int64) bool {
	s := NewSigner(secret)
	return s.Validate(resourceID, signature, expires, "")
}
