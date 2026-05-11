package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestImportJob_TableName(t *testing.T) {
	j := ImportJob{}
	assert.Equal(t, "import_jobs", j.TableName())
}

func TestImportJob_ToResponse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	orgID := uuid.New()
	createdBy := uuid.New()
	sourcePath := "/imports/data.json"
	idempotencyKey := "key-123"
	errMsg := "import failed"
	startedAt := now.Add(-5 * time.Minute)

	t.Run("maps all fields correctly", func(t *testing.T) {
		result := datatypes.JSON(`{"total_created": 10, "total_skipped": 2}`)
		job := &ImportJob{
			ID:                  uuid.New(),
			Status:              ImportCompleted,
			EntityTypes:         pq.StringArray{"users", "roles"},
			Format:              "json",
			OrgID:               &orgID,
			CreatedBy:           createdBy,
			ConflictStrategy:    "skip",
			DryRun:              false,
			SourceFilePath:     &sourcePath,
			IdempotencyKey:     &idempotencyKey,
			Result:             result,
			ErrorMessage:       &errMsg,
			ProcessingStartedAt: &startedAt,
			CreatedAt:          now,
			UpdatedAt:          now,
		}

		resp := job.ToResponse()

		assert.Equal(t, job.ID.String(), resp.ID)
		assert.Equal(t, ImportCompleted, resp.Status)
		assert.Equal(t, pq.StringArray{"users", "roles"}, resp.EntityTypes)
		assert.Equal(t, "json", resp.Format)
		require.NotNil(t, resp.OrgID)
		assert.Equal(t, orgID.String(), *resp.OrgID)
		assert.Equal(t, createdBy.String(), resp.CreatedBy)
		assert.Equal(t, "skip", resp.ConflictStrategy)
		assert.False(t, resp.DryRun)
		require.NotNil(t, resp.SourceFilePath)
		assert.Equal(t, sourcePath, *resp.SourceFilePath)
		require.NotNil(t, resp.IdempotencyKey)
		assert.Equal(t, idempotencyKey, *resp.IdempotencyKey)
		assert.Equal(t, result, resp.Result)
		require.NotNil(t, resp.ErrorMessage)
		assert.Equal(t, errMsg, *resp.ErrorMessage)
		assert.Equal(t, now, resp.CreatedAt)
		assert.Equal(t, now, resp.UpdatedAt)
	})

	t.Run("nil optional fields return zero values", func(t *testing.T) {
		job := &ImportJob{
			ID:               uuid.New(),
			Status:           ImportQueued,
			EntityTypes:      pq.StringArray{"users"},
			Format:           "json",
			OrgID:            nil,
			CreatedBy:        createdBy,
			ConflictStrategy: "skip",
			DryRun:           true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		resp := job.ToResponse()

		assert.Nil(t, resp.OrgID)
		assert.Nil(t, resp.SourceFilePath)
		assert.Nil(t, resp.IdempotencyKey)
		assert.Nil(t, resp.ErrorMessage)
		assert.Equal(t, job.ID.String(), resp.ID)
		assert.True(t, resp.DryRun)
	})

	t.Run("DeletedAt is excluded from response", func(t *testing.T) {
		job := &ImportJob{
			ID:               uuid.New(),
			Status:           ImportFailed,
			CreatedBy:        createdBy,
			ConflictStrategy: "fail",
			CreatedAt:        now,
			UpdatedAt:        now,
			DeletedAt:        gorm.DeletedAt{Valid: true, Time: now},
		}

		resp := job.ToResponse()

		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, job.ID.String(), resp.ID)
		assert.Equal(t, ImportFailed, resp.Status)
		assert.Nil(t, resp.OrgID)
		assert.Nil(t, resp.SourceFilePath)
		assert.Nil(t, resp.IdempotencyKey)
		assert.Nil(t, resp.ErrorMessage)
	})
}

func TestImportJob_StatusConstants(t *testing.T) {
	assert.Equal(t, "queued", ImportQueued)
	assert.Equal(t, "validating", ImportValidating)
	assert.Equal(t, "processing", ImportProcessing)
	assert.Equal(t, "completed", ImportCompleted)
	assert.Equal(t, "failed", ImportFailed)
	assert.Equal(t, "cancelled", ImportCancelled)
}

func TestConflictStrategy_Constants(t *testing.T) {
	assert.Equal(t, ConflictStrategy("skip"), ConflictSkip)
	assert.Equal(t, ConflictStrategy("overwrite"), ConflictOverwrite)
	assert.Equal(t, ConflictStrategy("fail"), ConflictFail)
}

func TestImportResult_Type(t *testing.T) {
	result := ImportResult{
		EntityTypes: map[string]EntityTypeResult{
			"users": {
				Created:     10,
				Skipped:     2,
				Failed:      1,
				Overwritten: 3,
			},
		},
		TotalCreated:     10,
		TotalSkipped:     2,
		TotalFailed:      1,
		TotalOverwritten: 3,
	}

	assert.Equal(t, 10, result.TotalCreated)
	assert.Equal(t, 2, result.TotalSkipped)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Equal(t, 3, result.TotalOverwritten)
	etResult, ok := result.EntityTypes["users"]
	require.True(t, ok)
	assert.Equal(t, 10, etResult.Created)
	assert.Equal(t, 2, etResult.Skipped)
	assert.Equal(t, 1, etResult.Failed)
	assert.Equal(t, 3, etResult.Overwritten)
}

func TestCreateImportRequest_StructTags(t *testing.T) {
	req := CreateImportRequest{
		EntityTypes:      pq.StringArray{"users"},
		Format:           "json",
		ConflictStrategy: "skip",
		DryRun:           false,
	}
	assert.Equal(t, pq.StringArray{"users"}, req.EntityTypes)
	assert.Equal(t, "json", req.Format)
	assert.Equal(t, "skip", req.ConflictStrategy)
	assert.False(t, req.DryRun)
}

func TestImportPreviewRequest_StructTags(t *testing.T) {
	req := ImportPreviewRequest{
		Format:           "csv",
		ConflictStrategy: "overwrite",
	}
	assert.Equal(t, "csv", req.Format)
	assert.Equal(t, "overwrite", req.ConflictStrategy)
}

func TestImportPreviewResponse_Type(t *testing.T) {
	resp := ImportPreviewResponse{
		EntityTypes: []ImportPreviewEntityType{
			{EntityType: "users", RowCount: 100},
			{EntityType: "roles", RowCount: 10},
		},
		TotalRows: 110,
		Warnings: []string{"duplicate email found"},
	}
	assert.Len(t, resp.EntityTypes, 2)
	assert.Equal(t, 110, resp.TotalRows)
	assert.Len(t, resp.Warnings, 1)
}