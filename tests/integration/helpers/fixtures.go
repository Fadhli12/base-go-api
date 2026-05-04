//go:build integration
// +build integration

package helpers

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const (
	// TestPassword is a valid test password meeting complexity requirements.
	TestPassword = "TestPassword123!"
	// TestEmailDomain is appended to generated email addresses.
	TestEmailDomain = "@test.example.com"
	// TestAdminRoleName is the standard admin role name.
	TestAdminRoleName = "admin"
	// TestUserRoleName is the standard user role name.
	TestUserRoleName = "user"
)

var (
	// CommonValidUserPayload is a valid user registration payload.
	CommonValidUserPayload = map[string]interface{}{
		"email":    "test@example.com",
		"password": TestPassword,
	}

	// CommonInvalidPayloads maps human-readable names to invalid payloads
	// and the expected validation error field.
	CommonInvalidPayloads = map[string]struct {
		Payload       map[string]interface{}
		ExpectedField string
	}{
		"missing_email": {
			Payload: map[string]interface{}{
				"password": TestPassword,
			},
			ExpectedField: "email",
		},
		"missing_password": {
			Payload: map[string]interface{}{
				"email": "test@example.com",
			},
			ExpectedField: "password",
		},
		"invalid_email": {
			Payload: map[string]interface{}{
				"email":    "not-an-email",
				"password": TestPassword,
			},
			ExpectedField: "email",
		},
		"weak_password": {
			Payload: map[string]interface{}{
				"email":    "test@example.com",
				"password": "123",
			},
			ExpectedField: "password",
		},
	}
)

func UniqueEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test-%d-%s%s", time.Now().UnixNano(), randomString(8), TestEmailDomain)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}