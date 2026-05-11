package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExportJob_TableName(t *testing.T) {
	e := ExportJob{}
	assert.Equal(t, "export_jobs", e.TableName())
}

func TestExportJob_ToResponse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	orgID := uuid.New()
	createdBy := uuid.New()
	filePath := "/exports/data.json"
	expiresAt := now.Add(24 * time.Hour)
	recordCount := 42
	errMsg := "something went wrong"
	hmacSig := "sha256=abc123"

	t.Run("maps all fields correctly", func(t *testing.T) {
		job := &ExportJob{
			ID:            uuid.New(),
			Status:        ExportCompleted,
			EntityTypes:   pq.StringArray{"users", "roles"},
			Format:        "json",
			OrgID:         &orgID,
			CreatedBy:     createdBy,
			FilePath:      &filePath,
			FileExpiresAt: &expiresAt,
			RecordCount:   &recordCount,
			ErrorMessage:  &errMsg,
			HmacSignature: &hmacSig,
			Sync:          true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		resp := job.ToResponse()

		assert.Equal(t, job.ID.String(), resp.ID)
		assert.Equal(t, ExportCompleted, resp.Status)
		assert.Equal(t, pq.StringArray{"users", "roles"}, resp.EntityTypes)
		assert.Equal(t, "json", resp.Format)
		require.NotNil(t, resp.OrgID)
		assert.Equal(t, orgID.String(), *resp.OrgID)
		assert.Equal(t, createdBy.String(), resp.CreatedBy)
		require.NotNil(t, resp.FilePath)
		assert.Equal(t, filePath, *resp.FilePath)
		require.NotNil(t, resp.FileExpiresAt)
		assert.Equal(t, expiresAt, *resp.FileExpiresAt)
		require.NotNil(t, resp.RecordCount)
		assert.Equal(t, 42, *resp.RecordCount)
		require.NotNil(t, resp.ErrorMessage)
		assert.Equal(t, errMsg, *resp.ErrorMessage)
		require.NotNil(t, resp.HmacSignature)
		assert.Equal(t, hmacSig, *resp.HmacSignature)
		assert.True(t, resp.Sync)
		assert.Equal(t, now, resp.CreatedAt)
		assert.Equal(t, now, resp.UpdatedAt)
	})

	t.Run("nil optional fields return zero values", func(t *testing.T) {
		job := &ExportJob{
			ID:          uuid.New(),
			Status:      ExportQueued,
			EntityTypes: pq.StringArray{"users"},
			Format:      "json",
			OrgID:       nil,
			CreatedBy:   createdBy,
			Sync:        false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		resp := job.ToResponse()

		assert.Nil(t, resp.OrgID)
		assert.Nil(t, resp.FilePath)
		assert.Nil(t, resp.FileExpiresAt)
		assert.Nil(t, resp.RecordCount)
		assert.Nil(t, resp.ErrorMessage)
		assert.Nil(t, resp.HmacSignature)
		assert.Equal(t, job.ID.String(), resp.ID)
		assert.Equal(t, ExportQueued, resp.Status)
		assert.False(t, resp.Sync)
	})

	t.Run("DeletedAt is excluded from response", func(t *testing.T) {
		job := &ExportJob{
			ID:        uuid.New(),
			Status:    ExportCompleted,
			CreatedBy: createdBy,
			CreatedAt: now,
			UpdatedAt: now,
			DeletedAt: gorm.DeletedAt{Valid: true, Time: now},
		}

		resp := job.ToResponse()

		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, job.ID.String(), resp.ID)
		assert.Equal(t, ExportCompleted, resp.Status)
		assert.Nil(t, resp.OrgID)
		assert.Nil(t, resp.FilePath)
		assert.Nil(t, resp.RecordCount)
		assert.Nil(t, resp.ErrorMessage)
		assert.Nil(t, resp.HmacSignature)
	})
}

func TestExportJob_StatusConstants(t *testing.T) {
	assert.Equal(t, "queued", ExportQueued)
	assert.Equal(t, "processing", ExportProcessing)
	assert.Equal(t, "completed", ExportCompleted)
	assert.Equal(t, "failed", ExportFailed)
}

func TestCreateExportRequest_StructTags(t *testing.T) {
	req := CreateExportRequest{
		EntityTypes:    pq.StringArray{"users", "roles"},
		Format:         "json",
		Sync:           false,
		IncludeDeleted: false,
	}
	assert.Equal(t, pq.StringArray{"users", "roles"}, req.EntityTypes)
	assert.Equal(t, "json", req.Format)
	assert.False(t, req.Sync)
	assert.False(t, req.IncludeDeleted)
}