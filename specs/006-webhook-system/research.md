# Research: Webhook System

**Feature**: 006-webhook-system | **Date**: 2026-05-02

---

## Research Tasks & Decisions

### R1: HMAC-SHA256 Signature Mechanism

**Decision**: Use HMAC-SHA256 for webhook payload signatures (GitHub/Stripe pattern). Stdlib only — `crypto/hmac` + `crypto/sha256`. Secret is 32 bytes hex-encoded, generated on webhook creation. Signature header: `X-Webhook-Signature: t=1683456789,v1=abcdef...` with timestamp to prevent replay.

**Alternatives rejected**: RSA (overkill), asymmetric (key management), no signature (security risk).

### R2: Delivery & Retry Strategy

**Decision**: Adopt existing email queue async pattern (Redis sorted set + DB status tracking). Redis ZSET for priority queue, `WebhookDelivery` entity in PostgreSQL for audit trail. Exponential backoff: +1min, +5min, +30min (3 attempts max). `next_retry_at` column enables backoff in `GetNextBatch` — gap identified in email worker that we'll fix here. 90-day delivery retention. Per-webhook rate limiting: 100/min via Redis sliding window.

**Alternatives rejected**: Goroutine-per-delivery (no persistence), pure Redis (no audit), external MQ (operational complexity), cron polling (slow).

### R3: Event System Architecture

**Decision**: In-process event bus with Go channels. Events as domain constants (`WebhookEventUserCreated = "user.created"`). `EventBus` struct with buffered channel and subscriber list. Services emit events without coupling to webhook service.

**Alternatives rejected**: Redis pub/sub (overkill for in-process), event bus library (unnecessary dep), direct service calls (tight coupling).

### R4: Webhook URL Validation & Security

**Decision**: Enforce HTTPS (dev override: `WEBHOOK_ALLOW_HTTP=true`). SSRF protection: block RFC 1918, localhost, link-local. HTTP client: 10s timeout, 1MB max payload, `User-Agent: Go-API-Webhook/1.0`. Response body capped at 1KB.

### R5: Rate Limiting Per Webhook

**Decision**: Redis sliding window (`webhook:ratelimit:{id}`, 60s TTL, 100/min default). Rate-exceeded deliveries marked `rate_limited` and retried with backoff. Configurable per-webhook via `rate_limit` field.

### R6: Organization Scoping

**Decision**: Webhooks are org-scoped via `OrganizationID` (NULL = global). Consistent with existing org feature. Service checks membership before CRUD.

### R7: Database Schema

**Decision**: `webhooks` (soft delete, org-scoped) + `webhook_deliveries` (immutable, NO soft delete — like email_queue/audit_logs). Migration `000014_webhooks.up.sql`. CHECK constraint on status enum. Partial indexes.

### R8: Async Worker

**Decision**: Mirror `EmailWorker` pattern — goroutine pool with Start/Stop, 30s graceful shutdown, atomic metrics. Configurable `WEBHOOK_WORKER_CONCURRENCY` (default: 5), `WEBHOOK_RETRY_MAX` (default: 3).

### R9: HTTP Client

**Decision**: stdlib `net/http` with custom client (10s timeout, 100 max idle conns, 10 per host). No external deps.

### R10: Permission Model

**Decision**: `webhooks:view` + `webhooks:manage` via Casbin domain enforcement. Added to `permissions.yaml` and synced via `permission:sync`. Route-level `RequirePermission` middleware + handler-level ownership check (org membership via `middleware.GetOrganizationID`). Services remain permission-free per existing project convention — no service-layer `enforcer.Enforce()` calls.