package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	// EncryptionVersionPrefix is the prefix used in encrypted strings for version identification.
	// Allows future algorithm migration without breaking existing encrypted values.
	EncryptionVersionPrefix = "v1:"

	// EncryptionInfoString is the HKDF info string used for key derivation.
	// Provides domain separation — the derived key will differ from keys derived for other purposes.
	EncryptionInfoString = "oauth-encryption-v1"

	// DerivedKeyLength is the length of the AES-256 key in bytes.
	DerivedKeyLength = 32

	// GCMNonceSize is the standard GCM nonce (IV) size in bytes.
	GCMNonceSize = 12

	// GCMTagSize is the GCM authentication tag size in bytes.
	GCMTagSize = 16
)

// OAuthEncryptionService handles encryption and decryption of OAuth client secrets
// using AES-256-GCM with HKDF-SHA256 key derivation.
//
// The master key is either provided via OAUTH_ENCRYPTION_KEY env var or derived from
// JWT_SECRET using HKDF-SHA256 with info "oauth-encryption-v1". The derived key is
// used for all AES-GCM operations.
//
// Encrypted format: "v1:" + base64(IV[12] + ciphertext + GCM_tag[16])
type OAuthEncryptionService struct {
	derivedKey []byte // 32-byte AES-256 key derived from master key via HKDF
}

// NewOAuthEncryptionService creates a new encryption service.
// The masterKey is either OAUTH_ENCRYPTION_KEY (if set) or derived from JWT_SECRET
// via HKDF-SHA256 with info "oauth-encryption-v1".
//
// The master key is used as input key material (IKM) for HKDF, which derives a
// 32-byte AES-256 key. This ensures that even if the master key is used elsewhere,
// the derived encryption key is cryptographically distinct.
func NewOAuthEncryptionService(masterKey []byte) (*OAuthEncryptionService, error) {
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("master key must not be empty")
	}

	// Derive a 32-byte key using HKDF-SHA256 with:
	// - Hash: sha256.New
	// - Info: []byte("oauth-encryption-v1") for key separation
	// - No salt (empty) — salt is optional in HKDF
	hkdfReader := hkdf.New(sha256.New, masterKey, nil, []byte(EncryptionInfoString))
	derivedKey := make([]byte, DerivedKeyLength)
	if _, err := io.ReadFull(hkdfReader, derivedKey); err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	if len(derivedKey) != DerivedKeyLength {
		return nil, fmt.Errorf("derived key must be %d bytes, got %d", DerivedKeyLength, len(derivedKey))
	}

	return &OAuthEncryptionService{derivedKey: derivedKey}, nil
}

// Encrypt encrypts a plaintext client secret using AES-256-GCM.
// Returns format: "v1:" + base64(IV[12] + ciphertext + GCM_tag[16])
//
// The version prefix allows future algorithm migration. When decrypting, the
// prefix determines which algorithm to use.
func (s *OAuthEncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("plaintext must not be empty")
	}

	// 1. Create AES cipher block from derived key
	block, err := aes.NewCipher(s.derivedKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 2. Create GCM mode
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// 3. Generate random 12-byte IV/nonce
	iv := make([]byte, GCMNonceSize)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// 4. Seal: ciphertext = aesGCM.Seal(nil, iv, []byte(plaintext), nil)
	// The result is ciphertext + GCM tag appended
	ciphertext := aesGCM.Seal(nil, iv, []byte(plaintext), nil)

	// 5. Concatenate: IV + ciphertext (which includes GCM tag)
	ivAndCiphertext := append(iv, ciphertext...)

	// 6. Return "v1:" + base64(IV + ciphertext + GCM_tag)
	encoded := base64.StdEncoding.EncodeToString(ivAndCiphertext)
	return EncryptionVersionPrefix + encoded, nil
}

// Decrypt decrypts a client secret that was encrypted with Encrypt.
// Expects format: "v1:" + base64(IV[12] + ciphertext + GCM_tag[16])
//
// Returns the original plaintext string.
func (s *OAuthEncryptionService) Decrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", fmt.Errorf("encrypted string must not be empty")
	}

	// 1. Strip "v1:" prefix
	if len(encrypted) < len(EncryptionVersionPrefix) {
		return "", fmt.Errorf("invalid encrypted format: too short")
	}
	prefix := encrypted[:len(EncryptionVersionPrefix)]
	if prefix != EncryptionVersionPrefix {
		return "", fmt.Errorf("invalid encrypted format: expected prefix %q, got %q", EncryptionVersionPrefix, prefix)
	}
	encoded := encrypted[len(EncryptionVersionPrefix):]

	// 2. Base64-decode the remaining string
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 3. Split into IV (first 12 bytes) and ciphertext (rest, which includes GCM tag)
	if len(data) < GCMNonceSize+GCMTagSize {
		return "", fmt.Errorf("invalid encrypted data: too short (%d bytes, need at least %d)", len(data), GCMNonceSize+GCMTagSize)
	}
	iv := data[:GCMNonceSize]
	ciphertext := data[GCMNonceSize:]

	// 4. Create AES cipher block from derived key
	block, err := aes.NewCipher(s.derivedKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 5. Create GCM mode
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// 6. Open: decrypt and verify authenticity
	plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}