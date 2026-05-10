package service

import (
	"testing"
)

func TestIsMimeTypeCompatible_ExactMatch(t *testing.T) {
	tests := []struct {
		name       string
		parentType string
		newType    string
		expected   bool
	}{
		{"image/png exact", "image/png", "image/png", true},
		{"image/jpeg exact", "image/jpeg", "image/jpeg", true},
		{"application/pdf exact", "application/pdf", "application/pdf", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMimeTypeCompatible(tt.parentType, tt.newType)
			if result != tt.expected {
				t.Errorf("isMimeTypeCompatible(%q, %q) = %v, want %v", tt.parentType, tt.newType, result, tt.expected)
			}
		})
	}
}

func TestIsMimeTypeCompatible_SameCategoryDifferentType(t *testing.T) {
	tests := []struct {
		name       string
		parentType string
		newType    string
		expected   bool
	}{
		{"image/png vs image/jpeg", "image/png", "image/jpeg", false},
		{"image/jpeg vs image/png", "image/jpeg", "image/png", false},
		{"image/png vs image/gif", "image/png", "image/gif", false},
		{"image/png vs image/webp", "image/png", "image/webp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMimeTypeCompatible(tt.parentType, tt.newType)
			if result != tt.expected {
				t.Errorf("isMimeTypeCompatible(%q, %q) = %v, want %v", tt.parentType, tt.newType, result, tt.expected)
			}
		})
	}
}

func TestIsMimeTypeCompatible_KnownAliases(t *testing.T) {
	tests := []struct {
		name       string
		parentType string
		newType    string
		expected   bool
	}{
		{"image/jpeg vs image/jpg alias", "image/jpeg", "image/jpg", true},
		{"image/jpg vs image/jpeg alias", "image/jpg", "image/jpeg", true},
		{"text/plain vs text/csv alias", "text/plain", "text/csv", true},
		{"text/csv vs text/plain alias", "text/csv", "text/plain", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMimeTypeCompatible(tt.parentType, tt.newType)
			if result != tt.expected {
				t.Errorf("isMimeTypeCompatible(%q, %q) = %v, want %v", tt.parentType, tt.newType, result, tt.expected)
			}
		})
	}
}

func TestIsMimeTypeCompatible_CompletelyDifferent(t *testing.T) {
	tests := []struct {
		name       string
		parentType string
		newType    string
		expected   bool
	}{
		{"image/png vs application/pdf", "image/png", "application/pdf", false},
		{"image/png vs text/plain", "image/png", "text/plain", false},
		{"application/pdf vs image/png", "application/pdf", "image/png", false},
		{"text/csv vs image/jpeg", "text/csv", "image/jpeg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMimeTypeCompatible(tt.parentType, tt.newType)
			if result != tt.expected {
				t.Errorf("isMimeTypeCompatible(%q, %q) = %v, want %v", tt.parentType, tt.newType, result, tt.expected)
			}
		})
	}
}