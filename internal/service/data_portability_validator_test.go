package service

import (
	"testing"

	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDataPortabilityValidator() *DataPortabilityValidator {
	return &DataPortabilityValidator{
		MaxFileSize:    DefaultMaxFileSize,
		MaxEntityCount: DefaultMaxEntityCount,
		MaxRecordCount: DefaultMaxRecordCount,
	}
}

func TestNewDataPortabilityValidator(t *testing.T) {
	v := NewDataPortabilityValidator()
	assert.Equal(t, int64(DefaultMaxFileSize), v.MaxFileSize)
	assert.Equal(t, DefaultMaxEntityCount, v.MaxEntityCount)
	assert.Equal(t, int64(DefaultMaxRecordCount), v.MaxRecordCount)
}

func TestValidateImportFile(t *testing.T) {
	tests := []struct {
		name        string
		fileSize    int64
		format      string
		entityCount int
		wantErr     bool
		errCode     string
	}{
		{
			name:        "valid json file",
			fileSize:    1024,
			format:      "json",
			entityCount: 100,
			wantErr:     false,
		},
		{
			name:        "valid csv file",
			fileSize:    2048,
			format:      "csv",
			entityCount: 500,
			wantErr:     false,
		},
		{
			name:        "valid uppercase format",
			fileSize:    1024,
			format:      "JSON",
			entityCount: 100,
			wantErr:     false,
		},
		{
			name:        "file too large",
			fileSize:    DefaultMaxFileSize + 1,
			format:      "json",
			entityCount: 100,
			wantErr:     true,
			errCode:     "PAYLOAD_TOO_LARGE",
		},
		{
			name:        "file at exact size limit",
			fileSize:    DefaultMaxFileSize,
			format:      "json",
			entityCount: 100,
			wantErr:     false,
		},
		{
			name:        "unsupported format xml",
			fileSize:    1024,
			format:      "xml",
			entityCount: 100,
			wantErr:     true,
			errCode:     "BAD_REQUEST",
		},
		{
			name:        "unsupported format empty",
			fileSize:    1024,
			format:      "",
			entityCount: 100,
			wantErr:     true,
			errCode:     "BAD_REQUEST",
		},
		{
			name:        "entity count too high",
			fileSize:    1024,
			format:      "json",
			entityCount: DefaultMaxEntityCount + 1,
			wantErr:     true,
			errCode:     "PAYLOAD_TOO_LARGE",
		},
		{
			name:        "entity count at exact limit",
			fileSize:    1024,
			format:      "json",
			entityCount: DefaultMaxEntityCount,
			wantErr:     false,
		},
		{
			name:        "zero file size",
			fileSize:    0,
			format:      "json",
			entityCount: 1,
			wantErr:     false,
		},
		{
			name:        "zero entity count",
			fileSize:    1024,
			format:      "json",
			entityCount: 0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDataPortabilityValidator()
			err := v.ValidateImportFile(tt.fileSize, tt.format, tt.entityCount)

			if tt.wantErr {
				require.Error(t, err)
				appErr := apperrors.GetAppError(err)
				require.NotNil(t, appErr)
				assert.Equal(t, tt.errCode, appErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileContent(t *testing.T) {
	tests := []struct {
		name      string
		content   []byte
		wantErr   bool
		errCode   string
	}{
		{
			name:    "valid json content",
			content: []byte(`{"type":"export","data":[]}`),
			wantErr: false,
		},
		{
			name:    "valid csv content",
			content: []byte("id,name\n1,Alice\n2,Bob"),
			wantErr: false,
		},
		{
			name:    "empty content",
			content: []byte{},
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "nil content",
			content: nil,
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "csv with equals formula",
			content: []byte("id,name\n=SUM(A1:A10),Alice"),
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "csv with plus formula",
			content: []byte("id,name\n1,+SUM(A1:A10)"),
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "csv with minus formula",
			content: []byte("id,name\n1,-SUM(A1:A10)"),
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "csv with at formula",
			content: []byte("id,name\n@SUM(A1:A10),Alice"),
			wantErr: true,
			errCode: "BAD_REQUEST",
		},
		{
			name:    "csv safe content no dangerous prefixes",
			content: []byte("id,name\n1,Alice\n2,Bob"),
			wantErr: false,
		},
		{
			name:    "json with equals in value is not csv",
			content: []byte(`{"formula":"=SUM()"}`),
			wantErr: false,
		},
		{
			name:    "csv with equals in middle of value is safe",
			content: []byte("id,name\n1,foo=bar"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDataPortabilityValidator()
			err := v.ValidateFileContent(tt.content)

			if tt.wantErr {
				require.Error(t, err)
				appErr := apperrors.GetAppError(err)
				require.NotNil(t, appErr)
				assert.Equal(t, tt.errCode, appErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateImportFile_CustomLimits(t *testing.T) {
	v := &DataPortabilityValidator{
		MaxFileSize:    1000,
		MaxEntityCount: 10,
		MaxRecordCount: 100,
	}

	err := v.ValidateImportFile(500, "json", 5)
	assert.NoError(t, err)

	err = v.ValidateImportFile(1001, "json", 5)
	require.Error(t, err)
	assert.Equal(t, "PAYLOAD_TOO_LARGE", apperrors.GetAppError(err).Code)

	err = v.ValidateImportFile(500, "json", 11)
	require.Error(t, err)
	assert.Equal(t, "PAYLOAD_TOO_LARGE", apperrors.GetAppError(err).Code)
}

func TestLooksLikeCSV(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name:   "simple csv",
			input:  "id,name\n1,Alice",
			expect: true,
		},
		{
			name:   "json object",
			input:  `{"key": "value"}`,
			expect: false,
		},
		{
			name:   "json array",
			input:  `[1, 2, 3]`,
			expect: false,
		},
		{
			name:   "plain text",
			input:  "hello world",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeCSV([]byte(tt.input))
			assert.Equal(t, tt.expect, result)
		})
	}
}