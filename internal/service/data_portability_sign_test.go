package service

import (
	"testing"
	"time"
)

func TestSignDataPortabilityFile(t *testing.T) {
	secret := "testsecret"
	payload := []byte(`{"type":"export","data":"hello"}`)

	sig := SignDataPortabilityFile(payload, secret)

	if len(sig) < 8 {
		t.Fatalf("signature too short: %s", sig)
	}
	if sig[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix, got %s", sig[:7])
	}
}

func TestSignDataPortabilityFile_WhsecPrefix(t *testing.T) {
	payload := []byte("test payload")
	secret := "mysecret"
	prefixed := "whsec_mysecret"

	sig1 := SignDataPortabilityFile(payload, secret)
	sig2 := SignDataPortabilityFile(payload, prefixed)

	if sig1 != sig2 {
		t.Errorf("whsec_ prefix not stripped: sig1=%s sig2=%s", sig1, sig2)
	}
}

func TestSignDataPortabilityFile_EmptyPayload(t *testing.T) {
	sig := SignDataPortabilityFile([]byte{}, "secret")
	if sig[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix for empty payload, got %s", sig[:7])
	}
}

func TestVerifyDataPortabilitySignature(t *testing.T) {
	tests := []struct {
		name      string
		payload   []byte
		secret    string
		expectOk  bool
	}{
		{
			name:     "valid signature",
			payload:  []byte(`{"type":"export"}`),
			secret:   "testsecret",
			expectOk: true,
		},
		{
			name:     "valid signature with whsec_ prefix",
			payload:  []byte(`{"type":"export"}`),
			secret:   "whsec_testsecret",
			expectOk: true,
		},
		{
			name:     "empty payload",
			payload:  []byte{},
			secret:   "secret",
			expectOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := SignDataPortabilityFile(tt.payload, tt.secret)
			ok := VerifyDataPortabilitySignature(tt.payload, sig, tt.secret)
			if ok != tt.expectOk {
				t.Errorf("VerifyDataPortabilitySignature() = %v, want %v", ok, tt.expectOk)
			}
		})
	}
}

func TestVerifyDataPortabilitySignature_InvalidSignature(t *testing.T) {
	payload := []byte("test payload")
	secret := "secret"

	ok := VerifyDataPortabilitySignature(payload, "sha256=invalidhex", secret)
	if ok {
		t.Error("expected invalid signature to fail verification")
	}
}

func TestVerifyDataPortabilitySignature_DifferentPayload(t *testing.T) {
	payload := []byte("original")
	different := []byte("tampered")
	secret := "secret"

	sig := SignDataPortabilityFile(payload, secret)
	ok := VerifyDataPortabilitySignature(different, sig, secret)
	if ok {
		t.Error("expected tampered payload to fail verification")
	}
}

func TestVerifyDataPortabilitySignature_DifferentSecret(t *testing.T) {
	payload := []byte("test payload")
	ok := VerifyDataPortabilitySignature(payload, SignDataPortabilityFile(payload, "secret1"), "secret2")
	if ok {
		t.Error("expected different secret to fail verification")
	}
}

func TestGenerateFileSignature(t *testing.T) {
	payload := []byte(`{"type":"export"}`)
	secret := "signingkey"
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	sig := GenerateFileSignature(payload, secret, ts)

	if sig[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix, got %s", sig[:7])
	}
}

func TestGenerateFileSignature_WhsecPrefix(t *testing.T) {
	payload := []byte("test")
	ts := time.Now()

	sig1 := GenerateFileSignature(payload, "mykey", ts)
	sig2 := GenerateFileSignature(payload, "whsec_mykey", ts)

	if sig1 != sig2 {
		t.Errorf("whsec_ prefix not stripped: sig1=%s sig2=%s", sig1, sig2)
	}
}

func TestGenerateFileSignature_TimestampIncluded(t *testing.T) {
	payload := []byte("test")
	secret := "secret"

	sig1 := GenerateFileSignature(payload, secret, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	sig2 := GenerateFileSignature(payload, secret, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if sig1 == sig2 {
		t.Error("different timestamps should produce different signatures")
	}
}