package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DPRateLimitKeyPrefix = "dp_rate:"
	DPRecordsKeyPrefix    = "dp_records:"
	DPRateLimitWindow     = 1 * time.Hour
	DefaultExportLimit    = 10
	DefaultImportLimit    = 5
	DefaultRecordLimit    = 50000
)

type DataPortabilityRateLimiterImpl struct {
	client      *redis.Client
	exportLimit int
	importLimit int
	recordLimit int
}

func NewDataPortabilityRateLimiter(client *redis.Client, exportLimit, importLimit, recordLimit int) *DataPortabilityRateLimiterImpl {
	return &DataPortabilityRateLimiterImpl{
		client:      client,
		exportLimit: exportLimit,
		importLimit: importLimit,
		recordLimit: recordLimit,
	}
}

func (r *DataPortabilityRateLimiterImpl) limitForAction(action string) int {
	switch action {
	case ActionExport:
		return r.exportLimit
	case ActionImport:
		return r.importLimit
	default:
		return r.exportLimit
	}
}

func (r *DataPortabilityRateLimiterImpl) Allow(ctx context.Context, userID string, action string) (bool, error) {
	key := fmt.Sprintf("%s%s:%s", DPRateLimitKeyPrefix, userID, action)
	limit := r.limitForAction(action)
	now := time.Now()
	windowStart := now.Add(-DPRateLimitWindow)

	r.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))

	count, err := r.client.ZCard(ctx, key).Result()
	if err != nil {
		return true, nil
	}

	if count >= int64(limit) {
		return false, nil
	}

	entryID := fmt.Sprintf("%d-%d", now.UnixMilli(), rand.Intn(10000))
	r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: entryID,
	})
	r.client.Expire(ctx, key, DPRateLimitWindow)

	return true, nil
}

func parseCountFromMember(member string) int64 {
	parts := strings.SplitN(member, ":", 2)
	if len(parts) < 1 {
		return 1
	}
	n, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 1
	}
	return n
}

func (r *DataPortabilityRateLimiterImpl) AllowRecords(ctx context.Context, orgID string, count int) (bool, error) {
	key := fmt.Sprintf("%s%s", DPRecordsKeyPrefix, orgID)
	now := time.Now()
	windowStart := now.Add(-DPRateLimitWindow)

	r.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))

	members, err := r.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return true, nil
	}

	var total int64
	for _, m := range members {
		total += parseCountFromMember(m)
	}

	if total+int64(count) > int64(r.recordLimit) {
		return false, nil
	}

	entryID := fmt.Sprintf("%d:%d-%d", count, now.UnixMilli(), rand.Intn(10000))
	r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixMilli()),
		Member: entryID,
	})
	r.client.Expire(ctx, key, DPRateLimitWindow)

	return true, nil
}