package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestImportIDMap_TableName(t *testing.T) {
	m := ImportIDMap{}
	assert.Equal(t, "import_id_maps", m.TableName())
}

func TestImportIDMap_Fields(t *testing.T) {
	id := uuid.New()
	jobID := uuid.New()
	externalID := uuid.New()
	internalID := uuid.New()

	m := ImportIDMap{
		ID:         id,
		JobID:      jobID,
		EntityType: EntityTypeUser,
		ExternalID: externalID,
		InternalID: internalID,
	}

	assert.Equal(t, id, m.ID)
	assert.Equal(t, jobID, m.JobID)
	assert.Equal(t, EntityTypeUser, m.EntityType)
	assert.Equal(t, externalID, m.ExternalID)
	assert.Equal(t, internalID, m.InternalID)
}