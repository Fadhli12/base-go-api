package service

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strings"

	apperrors "github.com/example/go-api-base/pkg/errors"
)

const (
	DefaultMaxFileSize     int64 = 52428800
	DefaultMaxEntityCount  int   = 10000
	DefaultMaxRecordCount  int64 = 50000
	maxContentRatio              = 10.0
)

var allowedFormats = map[string]bool{
	"json": true,
	"csv":  true,
}

type DataPortabilityValidator struct {
	MaxFileSize    int64
	MaxEntityCount int
	MaxRecordCount int64
}

func NewDataPortabilityValidator() *DataPortabilityValidator {
	return &DataPortabilityValidator{
		MaxFileSize:    DefaultMaxFileSize,
		MaxEntityCount: DefaultMaxEntityCount,
		MaxRecordCount: DefaultMaxRecordCount,
	}
}

func (v *DataPortabilityValidator) ValidateImportFile(fileSize int64, format string, entityCount int) error {
	if fileSize > v.MaxFileSize {
		return apperrors.NewAppError("PAYLOAD_TOO_LARGE", "import file exceeds maximum size limit", http.StatusRequestEntityTooLarge)
	}

	if !allowedFormats[strings.ToLower(format)] {
		return apperrors.NewAppError("BAD_REQUEST", "unsupported import format: must be json or csv", http.StatusBadRequest)
	}

	if entityCount > v.MaxEntityCount {
		return apperrors.NewAppError("PAYLOAD_TOO_LARGE", "entity count exceeds maximum limit", http.StatusRequestEntityTooLarge)
	}

	return nil
}

func (v *DataPortabilityValidator) ValidateFileContent(content []byte) error {
	if len(content) == 0 {
		return apperrors.NewAppError("BAD_REQUEST", "import file is empty", http.StatusBadRequest)
	}

	if err := v.checkCSVInjection(content); err != nil {
		return err
	}

	return nil
}

func (v *DataPortabilityValidator) checkCSVInjection(content []byte) error {
	if !looksLikeCSV(content) {
		return nil
	}

	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	dangerousPrefixes := []string{"=", "+", "-", "@"}

	for _, record := range records {
		for _, field := range record {
			trimmed := strings.TrimSpace(field)
			if trimmed == "" {
				continue
			}
			for _, prefix := range dangerousPrefixes {
				if strings.HasPrefix(trimmed, prefix) {
					return apperrors.NewAppError("BAD_REQUEST", "import file contains potentially dangerous CSV formula", http.StatusBadRequest)
				}
			}
		}
	}

	return nil
}

func looksLikeCSV(content []byte) bool {
	text := string(content[:min(len(content), 512)])
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return false
	}
	lines := strings.SplitN(text, "\n", 3)
	if len(lines) < 2 {
		return false
	}
	headerCommas := strings.Count(lines[0], ",")
	if headerCommas == 0 {
		return false
	}
	dataCommas := strings.Count(lines[1], ",")
	return dataCommas > 0 && headerCommas == dataCommas
}