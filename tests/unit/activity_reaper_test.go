package unit

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ======================================================================
// ActivityReaper tests
// ======================================================================

func TestActivityReaper_StartStop(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.DefaultActivityConfig()
	logger := slog.Default()

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	// Start the reaper
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Give the goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	reaper.Stop()
}

func TestActivityReaper_ArchiveOldActivities(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.ActivityConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond, // fast interval for testing
	}
	logger := slog.Default()

	activityRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(5), nil)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Wait for at least one tick
	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	activityRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestActivityReaper_ArchiveOldActivities_CustomRetention(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.ActivityConfig{
		RetentionDays:  30, // custom retention
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	activityRepo.On("ArchiveOlderThan", mock.Anything, 30).Return(int64(3), nil)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Wait for at least one tick
	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	activityRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 30)
}

func TestActivityReaper_ContextCancellation(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.ActivityConfig{
		RetentionDays:  90,
		ReaperInterval: 1 * time.Hour, // long interval so we test context cancellation
	}
	logger := slog.Default()

	// Should not be called since interval is 1 hour and we cancel immediately
	activityRepo.On("ArchiveOlderThan", mock.Anything, mock.AnythingOfType("int")).Return(int64(0), nil)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())

	reaper.Start(ctx)

	// Cancel context immediately
	cancel()

	// Give goroutine time to receive cancellation
	time.Sleep(100 * time.Millisecond)

	// ArchiveOlderThan should NOT have been called (interval is 1 hour)
	activityRepo.AssertNotCalled(t, "ArchiveOlderThan")
}

func TestActivityReaper_DefaultRetention(t *testing.T) {
	activityRepo := newMockActivityRepository()
	// RetentionDays = 0 should default to 90
	cfg := config.ActivityConfig{
		RetentionDays:  0,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	// Expect default retention of 90
	activityRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(0), nil)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Wait for at least one tick
	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	activityRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestActivityReaper_DefaultInterval(t *testing.T) {
	activityRepo := newMockActivityRepository()
	// ReaperInterval = 0 should default to 60s
	cfg := config.ActivityConfig{
		RetentionDays:  90,
		ReaperInterval: 0,
	}
	logger := slog.Default()

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	// We can't directly verify the interval, but we can verify Start works
	// with a 0 interval (should default to 60s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// Should not panic or hang
	reaper.Stop()
}

func TestActivityReaper_ArchiveError(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.ActivityConfig{
		RetentionDays:  90,
		ReaperInterval: 100 * time.Millisecond,
	}
	logger := slog.Default()

	// Simulate a repository error
	activityRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(0), assert.AnError)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Wait for at least one tick
	time.Sleep(250 * time.Millisecond)

	reaper.Stop()

	// Should have called ArchiveOlderThan even though it errored
	activityRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
}

func TestActivityReaper_MultipleTicks(t *testing.T) {
	activityRepo := newMockActivityRepository()
	cfg := config.ActivityConfig{
		RetentionDays:  90,
		ReaperInterval: 50 * time.Millisecond, // fast for testing
	}
	logger := slog.Default()

	activityRepo.On("ArchiveOlderThan", mock.Anything, 90).Return(int64(1), nil)

	reaper := service.NewActivityReaper(activityRepo, cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reaper.Start(ctx)

	// Wait for multiple ticks
	time.Sleep(300 * time.Millisecond)

	reaper.Stop()

	// Should have been called at least twice
	activityRepo.AssertCalled(t, "ArchiveOlderThan", mock.Anything, 90)
	// The number of calls should be >= 2
	assert.GreaterOrEqual(t, len(activityRepo.Calls), 2)
}