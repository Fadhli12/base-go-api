//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
}

func testJobConfig() *config.JobConfig {
	return &config.JobConfig{
		WorkerPoolSize:            2,
		PollerCount:               1,
		JobTimeoutSeconds:         30,
		MaxRetries:                3,
		ResultTTLSeconds:          604800,
		StuckJobThresholdSeconds:  1,
		ReaperIntervalSeconds:     1,
		CallbackTimeoutSeconds:    5,
		QueueKey:                  "jobs:queue",
		JobDataKeyPrefix:          "jobs:data:",
	}
}

func testPayload() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"action": "test_action",
		"value":  42,
	})
	return data
}

type failingHandler struct {
	errMsg string
}

func (h *failingHandler) Handle(ctx context.Context, jobType string, payload map[string]interface{}) ([]byte, error) {
	return nil, assert.AnError
}

func (h *failingHandler) SupportedTypes() []string {
	return []string{"test.fail"}
}

// ---------------------------------------------------------------------------
// Repository tests
// ---------------------------------------------------------------------------

func TestJobRepository_SubmitAndGetByID(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("submit creates a pending job", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, retrieved.ID)
		assert.Equal(t, "test.nop", retrieved.Type)
		assert.Equal(t, domain.JobStatusPending, retrieved.Status)
		assert.Equal(t, 1, retrieved.AttemptCount)
		assert.Equal(t, userID, retrieved.UserID)
	})

	t.Run("get by ID returns error for nonexistent job", func(t *testing.T) {
		suite.SetupTest(t)

		_, err := repo.GetByID(ctx, uuid.New())
		require.Error(t, err)
	})
}

func TestJobRepository_Update(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("update modifies an existing job", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		job.Status = domain.JobStatusProcessing
		now := time.Now()
		job.StartedAt = &now
		require.NoError(t, repo.Update(ctx, job))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusProcessing, retrieved.Status)
		assert.NotNil(t, retrieved.StartedAt)
	})
}

func TestJobRepository_EnqueueAndDequeue(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("enqueue and dequeue job", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		require.NoError(t, repo.Enqueue(ctx, job.ID, time.Now()))

		dequeued, err := repo.Dequeue(ctx, 5*time.Second)
		require.NoError(t, err)
		require.NotNil(t, dequeued)
		assert.Equal(t, job.ID, dequeued.ID)
	})

	t.Run("dequeue returns nil for empty queue", func(t *testing.T) {
		suite.SetupTest(t)

		dequeued, err := repo.Dequeue(ctx, 100*time.Millisecond)
		require.NoError(t, err)
		assert.Nil(t, dequeued)
	})
}

func TestJobRepository_SetProcessing(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("sets job to processing from pending", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		updated, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusProcessing, updated.Status)
		assert.NotNil(t, updated.StartedAt)
	})
}

func TestJobRepository_SetCompleted(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("completes a processing job", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))
		_, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		result := []byte(`{"status":"ok"}`)
		require.NoError(t, repo.SetCompleted(ctx, job.ID, result))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusCompleted, retrieved.Status)
		assert.Equal(t, result, retrieved.Result)
		assert.NotNil(t, retrieved.CompletedAt)
	})
}

func TestJobRepository_SetFailed(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("sets to failed with retry when retries remain", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))
		_, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "test error", &nextRetry))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusFailed, retrieved.Status)
		assert.Equal(t, "test error", retrieved.LastError)
		assert.NotNil(t, retrieved.NextRetryAt)
	})

	t.Run("sets to dead when no retries remain", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		job.AttemptCount = 4
		require.NoError(t, repo.Submit(ctx, job))
		_, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		require.NoError(t, repo.SetFailed(ctx, job.ID, "exhausted", nil))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, retrieved.Status)
	})
}

func TestJobRepository_ListByUser(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("lists jobs for a user", func(t *testing.T) {
		suite.SetupTest(t)

		for i := 0; i < 3; i++ {
			job := domain.NewJob("test.nop", testPayload(), userID)
			require.NoError(t, repo.Submit(ctx, job))
		}

		jobs, total, err := repo.ListByUser(ctx, userID, "", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, jobs, 3)
	})

	t.Run("filters jobs by status", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		jobs, _, err := repo.ListByUser(ctx, userID, domain.JobStatusPending, 10, 0)
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
	})

	t.Run("returns empty list for user with no jobs", func(t *testing.T) {
		suite.SetupTest(t)

		jobs, total, err := repo.ListByUser(ctx, uuid.New(), "", 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, jobs)
	})
}

func TestJobRepository_GetStuckJobs(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("finds jobs stuck in processing", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		_, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		past := time.Now().Add(-10 * time.Minute)
		fetched.StartedAt = &past
		require.NoError(t, repo.Update(ctx, fetched))

		stuckJobs, err := repo.GetStuckJobs(ctx, 1*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, stuckJobs)
		assert.Equal(t, job.ID, stuckJobs[0].ID)
	})

	t.Run("does not flag recently started jobs", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))
		_, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		stuckJobs, err := repo.GetStuckJobs(ctx, 10*time.Minute)
		require.NoError(t, err)
		assert.Empty(t, stuckJobs)
	})
}

func TestJobRepository_Delete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	userID := uuid.New()

	t.Run("deletes a job", func(t *testing.T) {
		suite.SetupTest(t)

		job := domain.NewJob("test.nop", testPayload(), userID)
		require.NoError(t, repo.Submit(ctx, job))

		require.NoError(t, repo.Delete(ctx, job.ID))

		_, err := repo.GetByID(ctx, job.ID)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Service tests
// ---------------------------------------------------------------------------

func TestJobService_Submit(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	svc := service.NewJobService(repo, cfg, testLogger())
	userID := uuid.New()

	t.Run("submits a new job in pending state", func(t *testing.T) {
		suite.SetupTest(t)

		req := &service.SubmitRequest{Type: "test.nop", Payload: testPayload()}
		job, err := svc.Submit(ctx, userID, req)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusPending, job.Status)
		assert.Equal(t, "test.nop", job.Type)
		assert.Equal(t, userID, job.UserID)
		assert.Equal(t, 3, job.MaxRetries)
	})

	t.Run("submits with custom max retries", func(t *testing.T) {
		suite.SetupTest(t)

		req := &service.SubmitRequest{Type: "test.nop", Payload: testPayload(), MaxRetries: 5}
		job, err := svc.Submit(ctx, userID, req)
		require.NoError(t, err)
		assert.Equal(t, 5, job.MaxRetries)
	})

	t.Run("submits with webhook callback URL", func(t *testing.T) {
		suite.SetupTest(t)

		req := &service.SubmitRequest{
			Type:       "test.nop",
			Payload:    testPayload(),
			WebhookURL: "https://example.com/callback",
		}
		job, err := svc.Submit(ctx, userID, req)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/callback", job.CallbackURL)
	})
}

func TestJobService_GetByID(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("retrieves an existing job", func(t *testing.T) {
		suite.SetupTest(t)

		req := &service.SubmitRequest{Type: "test.nop", Payload: testPayload()}
		submitted, err := svc.Submit(ctx, userID, req)
		require.NoError(t, err)

		retrieved, err := svc.GetByID(ctx, submitted.ID)
		require.NoError(t, err)
		assert.Equal(t, submitted.ID, retrieved.ID)
	})

	t.Run("returns error for nonexistent job", func(t *testing.T) {
		suite.SetupTest(t)

		_, err := svc.GetByID(ctx, uuid.New())
		require.Error(t, err)
	})
}

func TestJobService_ListByUser(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("lists user jobs with pagination", func(t *testing.T) {
		suite.SetupTest(t)

		for i := 0; i < 5; i++ {
			_, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
			require.NoError(t, err)
		}

		jobs, total, err := svc.ListByUser(ctx, userID, "", 3, 0)
		require.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, jobs, 3)

		jobs, total, err = svc.ListByUser(ctx, userID, "", 3, 3)
		require.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, jobs, 2)
	})

	t.Run("filters by status", func(t *testing.T) {
		suite.SetupTest(t)

		_, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		jobs, _, err := svc.ListByUser(ctx, userID, domain.JobStatusPending, 10, 0)
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
	})

	t.Run("returns empty for other user", func(t *testing.T) {
		suite.SetupTest(t)

		_, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		jobs, _, err := svc.ListByUser(ctx, uuid.New(), "", 10, 0)
		require.NoError(t, err)
		assert.Empty(t, jobs)
	})
}

func TestJobService_Cancel(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("cancels a pending job", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		require.NoError(t, svc.Cancel(ctx, job.ID, userID))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, retrieved.Status)
	})

	t.Run("cancels a failed job", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "error", &nextRetry))

		require.NoError(t, svc.Cancel(ctx, job.ID, userID))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, retrieved.Status)
	})

	t.Run("forbids cancellation by another user", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		err = svc.Cancel(ctx, job.ID, uuid.New())
		require.Error(t, err)
	})

	t.Run("cannot cancel a completed job", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		require.NoError(t, repo.SetCompleted(ctx, job.ID, []byte(`{}`)))

		err = svc.Cancel(ctx, job.ID, userID)
		require.Error(t, err)
	})
}

func TestJobService_Resubmit(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("resubmits a dead job", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)
		require.NoError(t, svc.Cancel(ctx, job.ID, userID))

		resubmitted, err := svc.Resubmit(ctx, job.ID, userID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusPending, resubmitted.Status)
		assert.Equal(t, 0, resubmitted.AttemptCount)
		assert.Nil(t, resubmitted.Result)
		assert.Empty(t, resubmitted.LastError)
	})

	t.Run("forbids resubmit by another user", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)
		require.NoError(t, svc.Cancel(ctx, job.ID, userID))

		_, err = svc.Resubmit(ctx, job.ID, uuid.New())
		require.Error(t, err)
	})

	t.Run("cannot resubmit a job not in dead state", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		_, err = svc.Resubmit(ctx, job.ID, userID)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Handler tests
// ---------------------------------------------------------------------------

func TestNoopHandler(t *testing.T) {
	t.Run("returns empty result for test.nop", func(t *testing.T) {
		h := service.NewNoopHandler(testLogger())
		result, err := h.Handle(context.Background(), "test.nop", map[string]interface{}{})
		require.NoError(t, err)
		assert.Equal(t, []byte("{}"), result)
	})

	t.Run("supports test.nop type", func(t *testing.T) {
		h := service.NewNoopHandler(testLogger())
		assert.Contains(t, h.SupportedTypes(), "test.nop")
	})
}

func TestEchoHandler(t *testing.T) {
	t.Run("echoes back the payload as JSON", func(t *testing.T) {
		h := service.NewEchoHandler(testLogger())
		payload := map[string]interface{}{"echo": "hello", "num": 123.0}
		result, err := h.Handle(context.Background(), "test.echo", payload)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"echo":"hello"`)
	})
}

func TestJobHandlerService(t *testing.T) {
	t.Run("dispatches to registered handler", func(t *testing.T) {
		svc := service.NewJobHandlerService(config.DefaultJobConfig(), testLogger())
		svc.Register("test.nop", service.NewNoopHandler(testLogger()))
		svc.Register("test.echo", service.NewEchoHandler(testLogger()))

		result, err := svc.Handle(context.Background(), "test.nop", nil)
		require.NoError(t, err)
		assert.Equal(t, []byte("{}"), result)

		result, err = svc.Handle(context.Background(), "test.echo", map[string]interface{}{"x": "y"})
		require.NoError(t, err)
		assert.Contains(t, string(result), `"x":"y"`)

		types := svc.SupportedTypes()
		assert.Contains(t, types, "test.nop")
		assert.Contains(t, types, "test.echo")
	})

	t.Run("returns error for unregistered job type", func(t *testing.T) {
		svc := service.NewJobHandlerService(config.DefaultJobConfig(), testLogger())
		_, err := svc.Handle(context.Background(), "unknown.type", nil)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Worker tests
// ---------------------------------------------------------------------------

func TestJobWorker_ProcessJob(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	userID := uuid.New()

	t.Run("processes a job to completion", func(t *testing.T) {
		suite.SetupTest(t)

		handlerSvc := service.NewJobHandlerService(config.DefaultJobConfig(), testLogger())
		handlerSvc.Register("test.nop", service.NewNoopHandler(testLogger()))

		worker := service.NewJobWorker(repo, suite.RedisClient, cfg, testLogger())
		worker.SetJobHandler(handlerSvc)

		job, err := service.NewJobService(repo, cfg, testLogger()).Submit(
			ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()},
		)
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(job.Payload, &payload))
		result, err := handlerSvc.Handle(ctx, "test.nop", payload)
		require.NoError(t, err)

		require.NoError(t, repo.SetCompleted(ctx, job.ID, result))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusCompleted, retrieved.Status)
	})
}

func TestJobWorker_FailureAndRetry(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	userID := uuid.New()

	t.Run("retries a failed job", func(t *testing.T) {
		suite.SetupTest(t)

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.fail", Payload: testPayload()})
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "processing error", &nextRetry))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusFailed, retrieved.Status)
		assert.Equal(t, "processing error", retrieved.LastError)
		assert.NotNil(t, retrieved.NextRetryAt)
		assert.True(t, retrieved.IsRetryable())
	})

	t.Run("job becomes dead after exhausting retries", func(t *testing.T) {
		suite.SetupTest(t)

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.fail", Payload: testPayload(), MaxRetries: 1})
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		require.NoError(t, repo.SetFailed(ctx, job.ID, "exhausted all retries", nil))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, retrieved.Status)
		assert.False(t, retrieved.IsRetryable())
	})
}

// ---------------------------------------------------------------------------
// Reaper tests
// ---------------------------------------------------------------------------

func TestJobReaper_ReapStuckJobs(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	userID := uuid.New()

	t.Run("recovers a stuck processing job", func(t *testing.T) {
		suite.SetupTest(t)

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		past := time.Now().Add(-10 * time.Minute)
		fetched.StartedAt = &past
		require.NoError(t, repo.Update(ctx, fetched))

		reaper := service.NewJobReaper(repo, cfg, testLogger())
		recovered, err := reaper.Reap(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, recovered)

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusPending, retrieved.Status)
		assert.Greater(t, retrieved.AttemptCount, 1)
	})

	t.Run("reaper returns 0 when no stuck jobs", func(t *testing.T) {
		suite.SetupTest(t)

		reaper := service.NewJobReaper(repo, cfg, testLogger())
		recovered, err := reaper.Reap(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, recovered)
	})
}

// ---------------------------------------------------------------------------
// Retry flow integration tests (Failed → Processing transition)
// ---------------------------------------------------------------------------

func TestJobRetryFlow_Integration(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	userID := uuid.New()

	t.Run("failed job with retries remaining can transition back to processing", func(t *testing.T) {
		suite.SetupTest(t)

		// Submit a job (default MaxRetries=3)
		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)
		require.Equal(t, 3, job.MaxRetries)

		// Step 1: Pending → Processing (first attempt)
		updated, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusProcessing, updated.Status)
		assert.Equal(t, 1, updated.AttemptCount)

		// Step 2: Processing → Failed (first failure, retries remain)
		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "temporary error", &nextRetry))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusFailed, retrieved.Status)
		assert.Equal(t, "temporary error", retrieved.LastError)
		assert.Equal(t, 2, retrieved.AttemptCount) // AttemptCount incremented
		assert.NotNil(t, retrieved.NextRetryAt)
		assert.True(t, retrieved.IsRetryable())

		// Step 3: Failed → Processing (retry — the P0-5 fix)
		retryProcessing, err := repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusProcessing, retryProcessing.Status)
		// AttemptCount should still be 2 (SetProcessing doesn't increment it)
		assert.Equal(t, 2, retryProcessing.AttemptCount)
		assert.NotNil(t, retryProcessing.StartedAt)

		// Step 4: Processing → Completed (successful retry)
		require.NoError(t, repo.SetCompleted(ctx, job.ID, []byte(`{"status":"ok"}`)))

		final, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusCompleted, final.Status)
		assert.Equal(t, 2, final.AttemptCount) // Completed on second attempt
		assert.Equal(t, []byte(`{"status":"ok"}`), final.Result)
	})

	t.Run("failed job dies after exhausting retries across multiple cycles", func(t *testing.T) {
		suite.SetupTest(t)

		// Submit with MaxRetries=3 so we can cycle through: attempt 1→fail→retry→attempt 2→fail→dead
		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{
			Type:       "test.nop",
			Payload:    testPayload(),
			MaxRetries: 3,
		})
		require.NoError(t, err)

		// Attempt 1 (AttemptCount=1→2): Pending → Processing → Failed (retryable)
		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "attempt 1 failed", &nextRetry))

		job, err = repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusFailed, job.Status)
		assert.Equal(t, 2, job.AttemptCount) // SetFailed increments: 1→2

		// Attempt 2 (AttemptCount=2→3): Failed → Processing → Failed (dead: 3 == MaxRetries)
		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err, "AttemptCount < MaxRetries (2<3) should allow re-processing")

		require.NoError(t, repo.SetFailed(ctx, job.ID, "attempt 2 failed - exhausted", nil))

		job, err = repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, job.Status)
		assert.Equal(t, "attempt 2 failed - exhausted", job.LastError)
		assert.False(t, job.IsRetryable())
	})

	t.Run("Failed→Processing rejects when no retries remain", func(t *testing.T) {
		suite.SetupTest(t)

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{
			Type:       "test.nop",
			Payload:    testPayload(),
			MaxRetries: 1,
		})
		require.NoError(t, err)

		// Exhaust the single retry
		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "failed", nil))

		retrieved, err := repo.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.JobStatusDead, retrieved.Status)

		// Dead job should NOT be able to transition back to Processing
		_, err = repo.SetProcessing(ctx, job.ID)
		require.Error(t, err, "dead job should not be re-processable via SetProcessing")
	})

	t.Run("Failed→Processing updates Redis status indexes correctly", func(t *testing.T) {
		suite.SetupTest(t)

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.nop", Payload: testPayload()})
		require.NoError(t, err)

		// Pending → Processing
		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		// Processing → Failed (with retry)
		nextRetry := time.Now().Add(1 * time.Minute)
		require.NoError(t, repo.SetFailed(ctx, job.ID, "temporary error", &nextRetry))

		// Verify job appears in failed status index
		failedKey := fmt.Sprintf("jobs:status:%s", domain.JobStatusFailed)
		isMember, err := suite.RedisClient.SIsMember(ctx, failedKey, job.ID.String()).Result()
		require.NoError(t, err)
		assert.True(t, isMember, "job should be in failed status index after SetFailed")

		// Processing key should not contain this job anymore
		processingKey := fmt.Sprintf("jobs:status:%s", domain.JobStatusProcessing)
		isMember, err = suite.RedisClient.SIsMember(ctx, processingKey, job.ID.String()).Result()
		require.NoError(t, err)
		assert.False(t, isMember, "job should not be in processing status index after SetFailed")

		// Failed → Processing (retry)
		_, err = repo.SetProcessing(ctx, job.ID)
		require.NoError(t, err)

		// Verify job removed from failed index and added to processing index
		isMember, err = suite.RedisClient.SIsMember(ctx, failedKey, job.ID.String()).Result()
		require.NoError(t, err)
		assert.False(t, isMember, "job should be removed from failed index after retry")

		isMember, err = suite.RedisClient.SIsMember(ctx, processingKey, job.ID.String()).Result()
		require.NoError(t, err)
		assert.True(t, isMember, "job should be in processing index after retry")
	})
}

// ---------------------------------------------------------------------------
// Domain state transition tests
// ---------------------------------------------------------------------------

func TestJobStateTransitions(t *testing.T) {
	t.Run("pending can only transition to processing", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())

		assert.True(t, job.CanTransitionTo(domain.JobStatusProcessing))
		assert.False(t, job.CanTransitionTo(domain.JobStatusCompleted))
		assert.False(t, job.CanTransitionTo(domain.JobStatusFailed))
		assert.False(t, job.CanTransitionTo(domain.JobStatusDead))
		assert.False(t, job.CanTransitionTo(domain.JobStatusPending))
	})

	t.Run("processing can transition to completed or failed", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())
		job.Status = domain.JobStatusProcessing

		assert.True(t, job.CanTransitionTo(domain.JobStatusCompleted))
		assert.True(t, job.CanTransitionTo(domain.JobStatusFailed))
		assert.False(t, job.CanTransitionTo(domain.JobStatusPending))
		assert.False(t, job.CanTransitionTo(domain.JobStatusDead))
	})

	t.Run("failed with retries remaining can retry", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())
		job.Status = domain.JobStatusFailed
		job.AttemptCount = 2
		job.MaxRetries = 5

		assert.True(t, job.CanTransitionTo(domain.JobStatusProcessing))
		assert.True(t, job.CanTransitionTo(domain.JobStatusDead))
		assert.False(t, job.CanTransitionTo(domain.JobStatusCompleted))
	})

	t.Run("failed with no retries can only go dead", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())
		job.Status = domain.JobStatusFailed
		job.AttemptCount = 5
		job.MaxRetries = 5

		assert.False(t, job.CanTransitionTo(domain.JobStatusProcessing))
		assert.True(t, job.CanTransitionTo(domain.JobStatusDead))
	})

	t.Run("completed and dead are terminal states", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())

		job.Status = domain.JobStatusCompleted
		assert.False(t, job.CanTransitionTo(domain.JobStatusPending))
		assert.False(t, job.CanTransitionTo(domain.JobStatusProcessing))
		assert.False(t, job.CanTransitionTo(domain.JobStatusFailed))
		assert.False(t, job.CanTransitionTo(domain.JobStatusDead))

		job.Status = domain.JobStatusDead
		assert.False(t, job.CanTransitionTo(domain.JobStatusPending))
		assert.False(t, job.CanTransitionTo(domain.JobStatusProcessing))
		assert.False(t, job.CanTransitionTo(domain.JobStatusFailed))
		assert.False(t, job.CanTransitionTo(domain.JobStatusCompleted))
	})
}

func TestJobResponse(t *testing.T) {
	t.Run("ToResponse creates correct response DTO", func(t *testing.T) {
		job := domain.NewJob("test.nop", testPayload(), uuid.New())
		now := time.Now()
		job.StartedAt = &now

		resp := job.ToResponse()
		assert.Equal(t, job.ID, resp.JobID)
		assert.Equal(t, "test.nop", resp.Type)
		assert.Equal(t, domain.JobStatusPending, resp.Status)
		assert.Equal(t, 1, resp.AttemptCount)
		assert.Equal(t, 3, resp.MaxRetries)
		assert.NotNil(t, resp.StartedAt)
		assert.Nil(t, resp.CompletedAt)
	})
}

// ---------------------------------------------------------------------------
// Concurrent submission tests
// ---------------------------------------------------------------------------

func TestJobService_ConcurrentSubmission(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("multiple concurrent submissions succeed", func(t *testing.T) {
		suite.SetupTest(t)

		const numJobs = 10
		errCh := make(chan error, numJobs)
		jobCh := make(chan *domain.Job, numJobs)

		for i := 0; i < numJobs; i++ {
			go func() {
				req := &service.SubmitRequest{Type: "test.nop", Payload: testPayload()}
				job, err := svc.Submit(ctx, userID, req)
				errCh <- err
				if err == nil {
					jobCh <- job
				}
			}()
		}

		var jobs []*domain.Job
		for i := 0; i < numJobs; i++ {
			require.NoError(t, <-errCh)
			select {
			case job := <-jobCh:
				jobs = append(jobs, job)
			default:
			}
		}

		ids := make(map[uuid.UUID]bool)
		for _, j := range jobs {
			assert.False(t, ids[j.ID], "duplicate job ID %s", j.ID)
			ids[j.ID] = true
		}
	})
}

// ---------------------------------------------------------------------------
// Worker integration with queue
// ---------------------------------------------------------------------------

func TestJobWorker_IntegrationWithQueue(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	repo := repository.NewJobRepository(suite.RedisClient)
	cfg := testJobConfig()
	userID := uuid.New()

	t.Run("worker picks up and processes enqueued jobs", func(t *testing.T) {
		suite.SetupTest(t)

		handlerSvc := service.NewJobHandlerService(config.DefaultJobConfig(), testLogger())
		handlerSvc.Register("test.echo", service.NewEchoHandler(testLogger()))

		worker := service.NewJobWorker(repo, suite.RedisClient, cfg, testLogger())
		worker.SetJobHandler(handlerSvc)

		workerCtx, workerCancel := context.WithCancel(ctx)
		defer workerCancel()

		go worker.Start(workerCtx)
		defer worker.Stop()

		svc := service.NewJobService(repo, cfg, testLogger())
		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{Type: "test.echo", Payload: testPayload()})
		require.NoError(t, err)

		require.NoError(t, repo.Enqueue(ctx, job.ID, time.Now()))

		deadline := time.After(12 * time.Second)
		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()

		processed := false
		for !processed {
			select {
			case <-deadline:
				t.Fatal("timeout waiting for worker to process job")
			case <-ticker.C:
				retrieved, err := repo.GetByID(ctx, job.ID)
				if err != nil {
					continue
				}
				if retrieved.Status == domain.JobStatusCompleted {
					processed = true
					assert.NotEmpty(t, retrieved.Result)
				}
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestJobService_EdgeCases(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()

	ctx := context.Background()
	repo := repository.NewJobRepository(suite.RedisClient)
	svc := service.NewJobService(repo, testJobConfig(), testLogger())
	userID := uuid.New()

	t.Run("submit with zero max retries falls back to config default", func(t *testing.T) {
		suite.SetupTest(t)

		job, err := svc.Submit(ctx, userID, &service.SubmitRequest{
			Type: "test.nop", Payload: testPayload(), MaxRetries: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, 3, job.MaxRetries)
	})

	t.Run("job IDs are unique", func(t *testing.T) {
		suite.SetupTest(t)

		req := &service.SubmitRequest{Type: "test.nop", Payload: testPayload()}
		job1, _ := svc.Submit(ctx, userID, req)
		job2, _ := svc.Submit(ctx, userID, req)
		assert.NotEqual(t, job1.ID, job2.ID)
	})

	t.Run("cancel nonexistent job returns error", func(t *testing.T) {
		suite.SetupTest(t)
		require.Error(t, svc.Cancel(ctx, uuid.New(), userID))
	})

	t.Run("resubmit nonexistent job returns error", func(t *testing.T) {
		suite.SetupTest(t)
		_, err := svc.Resubmit(ctx, uuid.New(), userID)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Callback service tests
// ---------------------------------------------------------------------------

func TestJobCallbackService_Deliver(t *testing.T) {
	t.Run("creates callback service with timeout", func(t *testing.T) {
		svc := service.NewJobCallbackService(5 * time.Second)
		require.NotNil(t, svc)

		err := svc.Deliver(context.Background(), "http://127.0.0.1:19999/nonexistent", &service.JobCallbackPayload{
			JobID:        uuid.New().String(),
			Status:       "completed",
			Result:       []byte(`{}`),
			AttemptCount: 1,
			CompletedAt:  time.Now(),
		})
		require.Error(t, err)
	})
}
