# Research: Background Job Queue

**Feature**: Background Job Queue  
**Date**: 2026-05-06  
**Source**: Phase 0 of `/speckit.plan`

## Research Questions

1. Redis sorted set queue pattern for job queuing
2. Go worker pool pattern for concurrent job processing
3. Retry backoff strategies
4. Stuck job recovery
5. Webhook callback delivery patterns

---

## 1. Redis Sorted Set Queue Pattern

**Decision**: Use Redis sorted set (`ZADD`/`ZRANGE`) for job queue with hash for job data

**Rationale**:
- Score-based ordering enables priority queue and scheduled retries
- O(log N) insertion and removal
- Natural FIFO with ZRANGE (oldest first)
- Score update allows re-scheduling without full re-queue

**Queue Structure**:
```
jobs:queue          → Sorted set (score = next_retry_at timestamp)
jobs:data:{job_id}  → Hash (all job attributes)
```

**Operations**:
- **Enqueue**: `ZADD jobs:queue {timestamp} {job_id}`
- **Dequeue**: `ZRANGE jobs:queue 0 0` → get oldest, `ZREM jobs:queue {job_id}`
- **Requeue (retry)**: `ZADD jobs:queue {next_retry_ts} {job_id}`
- **Get pending count**: `ZCOUNT jobs:queue -inf +inf`

**Alternatives considered**:
- List + LPUSH/BRPOP: Simple FIFO but no priority/scheduling
- Redis Streams: More features but overkill for this use case

---

## 2. Go Worker Pool Pattern

**Decision**: Fixed-size worker pool with goroutines

**Rationale**:
- Predictable concurrency (no unbounded goroutines)
- Easy shutdown (context cancellation + wait group)
- Graceful drain on shutdown (complete in-progress jobs)

**Pattern**:
```go
type WorkerPool struct {
    workers int
    queue   <-chan *Job
    wg      sync.WaitGroup
    ctx     context.Context
    cancel  context.CancelFunc
}

func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()
    for {
        select {
        case job := <-p.queue:
            p.process(job)
        case <-p.ctx.Done():
            return
        }
    }
}
```

**Shutdown Sequence**:
1. Cancel context (stop accepting new jobs)
2. Wait for workers to drain queue
3. Wait for workers to finish in-progress jobs
4. Close connections

---

## 3. Retry Backoff Strategy

**Decision**: Exponential backoff with cap

**Backoff Schedule**:
| Attempt | Delay |
|---------|-------|
| 1 | 1 minute |
| 2 | 5 minutes |
| 3+ | 30 minutes (capped) |

**Formula**:
```go
func backoffDelay(attempt int) time.Duration {
    base := time.Minute
    maxDelay := 30 * time.Minute
    
    delay := base * time.Duration(math.Pow(5, float64(attempt-1)))
    if delay > maxDelay {
        delay = maxDelay
    }
    return delay
}
```

**Dead Letter**: After `max_retries` attempts, job status becomes `dead`

**Alternatives considered**:
- Linear backoff (too slow to recover)
- Exponential without cap (can exceed reasonable time)
- Jitter added (not needed for server-side retry)

---

## 4. Stuck Job Recovery

**Decision**: Background reaper goroutine runs periodically

**Stuck Criteria**: Job in `processing` status for > 5 minutes

**Reaper Logic**:
1. Run every 60 seconds
2. Scan jobs with `status = processing AND started_at < now - 5min`
3. Reset to `pending` with incremented `attempt_count`
4. Log warning for monitoring

**Implementation**:
```go
func (r *JobReaper) Run(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    for {
        select {
        case <-ticker.C:
            r.recoverStuckJobs(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 5. Webhook Callback Delivery

**Decision**: Best-effort HTTP POST with 5-second timeout

**Rationale**:
- Callbacks are notifications, not critical path
- If endpoint fails, job status remains completed (callback is best-effort)
- Log failures for monitoring but don't retry callback

**Implementation**:
```go
func (s *CallbackService) Deliver(ctx context.Context, url string, payload []byte) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        slog.Warn("callback delivery failed", "url", url, "error", err)
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 400 {
        slog.Warn("callback returned error", "url", url, "status", resp.StatusCode)
    }
    return nil
}
```

**Payload**:
```json
{
  "job_id": "...",
  "status": "completed",
  "result": {...},
  "completed_at": "..."
}
```

---

## Resolved Unknowns

| Unknown | Resolution |
|---------|------------|
| Job ID generation | UUID v4 via `uuid.New()` |
| Payload size limit | Max 1MB, enforced at submission |
| Callback timeout | 5 seconds, best-effort |
| Worker count | Configurable via `JOB_WORKER_COUNT` env (default 5) |
| Queue keys | `jobs:queue` (sorted set), `jobs:data:{id}` (hash) |
| Job retention | Results expire via Redis TTL (7 days) |

---

## Decisions Summary

| Decision | Chosen | Rationale |
|----------|--------|-----------|
| Queue storage | Redis sorted set | Priority + scheduled retry support |
| Worker pattern | Fixed goroutine pool | Predictable concurrency |
| Retry strategy | Exponential (1m, 5m, 30m cap) | Balance between quick retry and load |
| Stuck recovery | Background reaper | Automatic recovery without manual intervention |
| Callbacks | Best-effort HTTP POST | Non-blocking, log failures only |
