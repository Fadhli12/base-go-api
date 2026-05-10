# Production Readiness Guide

**Project:** Go API Base
**Last Updated:** 2026-05-08
**Target Audience:** DevOps engineers, SREs, backend leads preparing for production deployment.

This guide covers everything needed to deploy the Go API Base securely and reliably in production. It assumes you have already reviewed the [Operations Runbook](runbook.md) for day-to-day procedures and the [Configuration Guide](configuration.md) for subsystem configuration.

---

## Table of Contents

1. [Pre-Production Checklist](#1-pre-production-checklist)
2. [Security Hardening](#2-security-hardening)
3. [Performance Tuning](#3-performance-tuning)
4. [Monitoring and Observability](#4-monitoring-and-observability)
5. [Backup and Disaster Recovery](#5-backup-and-disaster-recovery)
6. [Deployment](#6-deployment)
7. [Environment-Specific Configuration](#7-environment-specific-configuration)

---

## 1. Pre-Production Checklist

Complete every item before considering the deployment production-ready. These are one-time readiness checks, distinct from the per-deploy checklist in the [runbook](runbook.md).

### 1.1 Infrastructure

- [ ] PostgreSQL 15+ deployed with replication (at least one standby)
- [ ] Redis 7+ deployed with at least one replica for failover
- [ ] TLS certificates provisioned for all endpoints (API, PostgreSQL, Redis)
- [ ] Load balancer or reverse proxy (nginx, HAProxy, AWS ALB) in front of the API
- [ ] Internal network isolation: PostgreSQL and Redis not exposed publicly
- [ ] DNS records configured for the API domain and any subdomains
- [ ] Container registry access configured (Docker Hub, ECR, GCR, or private registry)
- [ ] Secrets management solution in place (Kubernetes Secrets, AWS Secrets Manager, HashiCorp Vault)

### 1.2 Database

- [ ] All migrations applied: `make migrate`
- [ ] Roles and permissions seeded: `make seed`
- [ ] Casbin policies synchronized: `make permission-sync`
- [ ] Connection pool limits set in `DATABASE_URL` query parameters (see [Section 3.1](#31-database-connection-pool))
- [ ] SSL enforced: `DATABASE_SSLMODE=verify-full` or equivalent in connection string
- [ ] Audit log table retention policy defined (see [Section 5.2](#52-log-retention))
- [ ] Index usage verified on high-traffic queries (users by email, refresh tokens by user_id, Casbin policies)

### 1.3 Secrets

- [ ] `JWT_SECRET` generated with at least 32 random characters (`openssl rand -base64 32`)
- [ ] `JWT_ISSUER` and `JWT_AUDIENCE` set to production values
- [ ] `STORAGE_SIGNING_KEY` generated and stored securely
- [ ] Database password meets organizational complexity requirements
- [ ] Redis password set if Redis is not on an isolated network
- [ ] Email provider credentials configured (SMTP password, SendGrid API key, or AWS IAM credentials)
- [ ] All secrets different from development and staging environments
- [ ] No secrets in `.env` files or version control (use secret injection at deploy time)

### 1.4 Application

- [ ] `GIN_MODE=release` to disable debug features
- [ ] `SWAGGER_ENABLED=false` unless documentation access is explicitly required
- [ ] `LOG_LEVEL=info` (use `warn` for high-traffic environments)
- [ ] `LOG_FORMAT=json` for structured log aggregation
- [ ] `LOG_ADD_SOURCE=false` to reduce overhead
- [ ] `ENABLE_REGISTRATION` set according to business rules (disable if using invite-only)
- [ ] `BCRYPT_COST=12` for stronger password hashing (increase from default 10)
- [ ] CORS origins restricted to production domains only
- [ ] Rate limits tuned for expected traffic patterns

### 1.5 Testing

- [ ] Full test suite passes: `make test && make test-integration`
- [ ] Load test at 2x expected peak traffic without degradation
- [ ] Chaos test: kill PostgreSQL, Redis, and API instances individually to verify recovery
- [ ] Security scan complete (OWASP ZAP, gosec, dependency audit)
- [ ] Penetration test conducted on production-like staging environment

---

## 2. Security Hardening

### 2.1 Container Security

The Docker image already applies these measures. Verify they remain intact in your deployment pipeline.

| Measure | Implementation | Verifiable |
|---------|---------------|-----------|
| Non-root user | `USER appuser:appgroup` in Dockerfile | `docker inspect` shows non-root User |
| Minimal base image | `alpine:3.19` runtime stage | Image size under 20 MB |
| Static binary | `CGO_ENABLED=0` build | `file ./api` shows statically linked |
| Stripped binary | `-ldflags="-w -s"` | Symbol table removed |
| No shell in container | Alpine base, no shell installed | `docker exec -it <id> sh` fails |
| Read-only root filesystem | Mount as read-only in orchestrator | Kubernetes `readOnlyRootFilesystem: true` |

**Additional hardening you should apply:**

```yaml
# Docker Compose additions for production
services:
  api:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp:mode=1777,size=64M
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
```

### 2.2 TLS Everywhere

| Connection | TLS Setting | Environment Variable |
|-----------|-------------|---------------------|
| Client to API | Terminate at load balancer or configure with reverse proxy | Reverse proxy config |
| API to PostgreSQL | `sslmode=verify-full` | `DATABASE_SSLMODE` or in `DATABASE_URL` |
| API to Redis | `rediss://` with `--tls` on Redis server | `REDIS_URL` with `rediss://` prefix |
| API to SMTP | STARTTLS on port 587 or SSL on port 465 | `EMAIL_SMTP_PORT` |
| API to S3/MinIO | HTTPS endpoint | `STORAGE_S3_USE_SSL=true` |

### 2.3 Security Headers

The application includes a `SecurityHeaders` middleware in `internal/http/middleware/security_headers.go`. For production, use the strict variant:

```go
// In cmd/api/main.go or server configuration
e.Use(middleware.ProductionSecurityHeaders("your-domain.com"))
```

This sends the following headers on every response:

```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: camera=(), microphone=(), geolocation=()
```

### 2.4 Authentication and Authorization

**JWT configuration for production:**

```bash
JWT_SECRET=<generate with: openssl rand -base64 32>
JWT_ISSUER=go-api-base
JWT_AUDIENCE=api
ACCESS_TOKEN_TTL=15m          # Keep short, refresh frequently
REFRESH_TOKEN_TTL=720h        # 30 days (constitution default)
PASSWORD_RESET_TOKEN_EXPIRY=1h
```

**Rate limiting for auth endpoints (already implemented):**

| Endpoint | Limit | Rationale |
|----------|-------|-----------|
| Login | 5 per 15 min per IP+email | Prevents credential stuffing |
| Registration | 5 per hour per IP | Prevents account farming |
| Password reset | 3 per hour per email | Prevents reset spam |
| Token refresh | Uncapped (global rate limit) | Necessary for token rotation |

**Token family tracking (already implemented):**

If a refresh token from a previously revoked chain is reused, the entire token family is revoked and an `AuditActionTokenReuse` event is logged. Monitor `token_reuse_detected > 0` in metrics as a security alert.

### 2.5 Input Validation and Sanitization

- All request payloads validated via Echo binding with struct tags (`validate:"required,min=8"`)
- Passwords validated with `password_strength` custom validator (min 8 chars, uppercase, lowercase, number, special char)
- Email addresses normalized and validated
- UUIDs validated on path parameters
- SQL injection prevented by GORM parameterized queries
- XSS prevented in email templates (HTML escaping in Go `html/template`)

### 2.6 Audit Logging

Audit logging is active for all security-sensitive operations:

| Event | Trigger | Audit Action |
|-------|---------|-------------|
| Login success | `POST /auth/login` 200 | `AuditActionLoginSuccess` |
| Login failure | `POST /auth/login` 401 | `AuditActionLoginFailed` |
| Password reset requested | `POST /auth/password-reset` | `AuditActionPasswordReset` |
| Password reset complete | `POST /auth/password-reset/confirm` | `AuditActionPasswordChange` |
| Token reuse detected | Refresh token from revoked family | `AuditActionTokenReuse` |

Audit logs are stored in the `audit_logs` table with before/after JSONB payloads. The table is protected with a database trigger that prevents UPDATE and DELETE operations, ensuring immutability for compliance.

### 2.7 Dependency Security

- All Go dependencies tracked in `go.mod` and `go.sum` with exact versions
- `govulncheck` integration recommended in CI pipeline
- `golangci-lint` with `gosec` enabled catches common vulnerabilities
- Run `go mod tidy` and `go mod verify` before each release

---

## 3. Performance Tuning

### 3.1 Database Connection Pool

Configure these GORM connection pool parameters in your environment:

```bash
# Append to DATABASE_URL or configure via Go code
DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=verify-full"
```

GORM pool settings are configured at connection time. The defaults are conservative. For production:

| Setting | Recommended Value | Calculation |
|---------|-------------------|-------------|
| Max open connections | `50` | 25 per vCPU, or `max_db_connections * 0.8 / instance_count` |
| Max idle connections | `10` | 20% of max open |
| Connection max lifetime | `30m` | Prevent stale connections |
| Connection max idle time | `5m` | Release idle connections swiftly |

### 3.2 Redis Configuration

| Setting | Recommended Value | Notes |
|---------|-------------------|-------|
| Max memory policy | `allkeys-lru` | Evicts least recently used keys when memory fills |
| Max memory | 256MB to 1GB depending on workload | Monitor with `redis-cli info memory` |
| Persistence | RDB snapshots every 5 minutes if >= 1 change | `save 300 1` |
| TCP keepalive | 60 seconds | Detect dead connections faster |

### 3.3 Go Runtime

```bash
# Set via environment or Go flags
GOMAXPROCS=4         # Match vCPU count on container
GOMEMLIMIT=512MiB    # Soft memory limit, triggers GC earlier
```

The application binary is built with `-ldflags="-w -s"` for a smaller binary and faster startup. Set `GOMEMLIMIT` to about 80% of your container memory limit to prevent OOM kills during GC spikes.

### 3.4 Worker Concurrency

Background workers handle email delivery and webhook dispatch. Tune these based on expected load:

| Worker | Env Variable | Default | Recommendation |
|--------|-------------|---------|---------------|
| Email workers | `EMAIL_WORKER_CONCURRENCY` | 10 | Match SMTP server connection limits |
| Webhook workers | `WEBHOOK_WORKER_CONCURRENCY` | 5 | 1 per 20 expected webhooks |
| Email retry max | `EMAIL_RETRY_MAX` | 5 | Keep at 5 for transient failures |
| Webhook retry max | `WEBHOOK_RETRY_MAX` | 3 | Exponential backoff: 1m, 5m, 30m |

### 3.5 HTTP Server

Echo v4 is already high-performance. Additional tuning:

| Setting | Value | Notes |
|---------|-------|-------|
| Read timeout | 15s | Prevents slow-client attacks |
| Write timeout | 15s | Matches read timeout |
| Idle timeout | 60s | Release idle connections |
| Max header bytes | 1MB | Prevent large header attacks |

### 3.6 Cache Strategy

| Cache | Driver | TTL | Notes |
|-------|--------|-----|-------|
| Permission cache | Redis | 300s (5 min) | Pub/sub invalidation on changes |
| Rate limiting | Redis | 60s | Sliding window per IP |
| Application data | Redis (optional) | Configurable | Use `CACHE_DEFAULT_TTL` |
| Image processing | Filesystem or memory | Request-scoped | Prevents re-compression |

---

## 4. Monitoring and Observability

### 4.1 Health Endpoints

| Endpoint | Purpose | Expected | Kubernetes Probe |
|----------|---------|----------|-----------------|
| `GET /healthz` | Liveness | `{"status":"healthy"}` | `livenessProbe` |
| `GET /readyz` | Readiness | `{"status":"ready"}` | `readinessProbe` |

`readyz` verifies both PostgreSQL and Redis connectivity. If either is down, it returns 503, signal the orchestrator to stop routing traffic.

```yaml
# Kubernetes example
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 15
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

### 4.2 Metrics

Auth metrics are available at `GET /api/v1/metrics/auth` (requires authentication). These counters are atomic, goroutine-safe, and process-scoped.

```json
{
  "data": {
    "login_success": 0,
    "login_failed": 0,
    "login_rate_limited": 0,
    "password_reset_requested": 0,
    "password_reset_completed": 0,
    "password_reset_failed": 0,
    "token_refresh_success": 0,
    "token_refresh_failed": 0,
    "token_reuse_detected": 0,
    "active_sessions": 0,
    "session_revoked": 0
  }
}
```

**Recommended Prometheus setup:**

Expose these metrics via a Prometheus-compatible endpoint or push to a gateway. Integrate with a monitoring stack (Grafana, Datadog, New Relic) for visualization.

### 4.3 Structured Logging

Every request generates a JSON log entry with these automatic fields:

| Field | Source | Example |
|-------|--------|---------|
| `request_id` | X-Request-ID header or generated UUID | `a1b2c3d4-...` |
| `user_id` | JWT claims (if authenticated) | `e5f6g7h8-...` |
| `org_id` | X-Organization-ID header (if present) | `i9j0k1l2-...` |
| `trace_id` | X-Trace-ID header or generated | `m3n4o5p6-...` |
| `method`, `path`, `status`, `duration` | HTTP request metadata | `GET /api/v1/users 200 45ms` |

**Log aggregation recommendations:**

- **ELK Stack (Elasticsearch, Logstash, Kibana):** Send JSON logs directly, no parsing required.
- **Datadog / New Relic:** Configure their agents to ingest JSON logs.
- **Loki + Grafana:** Use Promtail with JSON parser.
- **AWS CloudWatch:** Configure the CloudWatch agent with JSON log format.

### 4.4 Recommended Alerts

| Alert | Condition | Severity | Action |
|-------|-----------|----------|--------|
| API instance down | `/healthz` fails 3 times | **Critical** | Page on-call immediately |
| API not ready | `/readyz` returns 503 for 2 minutes | **Critical** | Check DB/Redis connectivity |
| High error rate | 5xx > 5% for 5 minutes | **Warning** | Investigate logs for root cause |
| High latency | p95 > 500ms for 5 minutes | **Warning** | Check DB query performance, worker backpressure |
| Token reuse detected | `token_reuse_detected > 0` | **Critical** | Potential credential theft, investigate user account |
| Login rate limited spike | `login_rate_limited` exceeds hourly baseline by 5x | **Warning** | Possible brute force attack |
| Database connection pool exhausted | `readyz` intermittent failures | **Warning** | Scale up pool or add read replicas |
| Redis memory high | Redis used_memory > 80% maxmemory | **Warning** | Scale Redis or tune eviction policy |
| Certificate expiry | TLS cert expiring within 14 days | **Warning** | Renew certificate |

### 4.5 Distributed Tracing

The `request_id` and `trace_id` fields propagate through all logs. For full distributed tracing:

1. Forward `X-Trace-ID` from upstream services or proxies.
2. If using OpenTelemetry, instrument the API with the `otelhttp` or `otelecho` middleware.
3. The existing structured logging middleware already extracts and propagates `trace_id`, so traces will connect across services that forward the header.

---

## 5. Backup and Disaster Recovery

### 5.1 PostgreSQL Backups

**Daily full backups (recommended):**

```bash
# Automated via cron or scheduled job
pg_dump -Fc -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME \
  > /backups/go-api-base-$(date +%Y%m%d).dump

# Upload to off-site storage
aws s3 cp /backups/go-api-base-*.dump s3://my-backups-bucket/postgres/ \
  --storage-class STANDARD_IA
```

**Point-in-time recovery with WAL archiving:**

Configure PostgreSQL `archive_command` to ship WAL segments to S3 or a dedicated archive host. This enables recovery to any point in time, not just the last daily backup.

```ini
# postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'aws s3 cp %p s3://my-backups-bucket/wal/%f'
```

**RTO/RPO targets:**

| Target | Value | How |
|--------|-------|-----|
| Recovery Point Objective (RPO) | < 5 minutes | WAL archiving with 5-minute shipping intervals |
| Recovery Time Objective (RTO) | < 30 minutes | Automated restore scripts, pre-warmed standby |

**Restore procedure:**

```bash
# 1. Stop the application
# 2. Restore from latest backup
pg_restore -h $TARGET_HOST -U $TARGET_USER -d $TARGET_DB --clean --if-exists \
  /backups/go-api-base-$(date +%Y%m%d).dump

# 3. Apply WAL segments if doing point-in-time recovery
# (handled by PostgreSQL recovery.conf or restore_command)

# 4. Run any new migrations from the deployed version
make migrate

# 5. Verify data integrity
# Run application smoke tests

# 6. Start the application
```

### 5.2 Log Retention

| Log Type | Retention | Reason |
|----------|-----------|--------|
| Application logs | 30 days | Debugging, trend analysis |
| Audit logs | 365 days minimum | Compliance, forensic analysis |
| Access logs | 90 days | Security investigations |
| Webhook deliveries | Configurable via `WEBHOOK_DELIVERY_RETENTION_DAYS` (default: 90) | Delivery history |

The `audit_logs` table should never be truncated without legal/compliance approval. Use table partitioning by month for easier management of large audit datasets.

### 5.3 Redis Backups

Redis persistence modes:

| Mode | Trade-off | Recommended |
|------|-----------|-------------|
| RDB (snapshots) | Periodic snapshots, minimal performance impact. May lose minutes of data. | **Use this** for rate limit and cache data. |
| AOF (append-only file) | Logs every write. More durable, higher IO. | Use only if Redis stores critical data that cannot be recreated. |
| Both | Best durability, highest IO overhead. | Overkill for this application. |

```bash
# Redis configuration
save 900 1      # Save after 15 minutes if at least 1 key changed
save 300 10     # Save after 5 minutes if at least 10 keys changed
save 60 10000   # Save after 1 minute if at least 10000 keys changed
```

In this application, Redis stores permission caches, rate limit counters, email queues, and webhook queues. All of this data can be recreated if lost. RDB snapshots every 5 minutes are sufficient.

### 5.4 Disaster Recovery Plan

| Scenario | Impact | Recovery Steps |
|----------|--------|---------------|
| PostgreSQL data loss | Critical | [Restore from backup](#51-postgresql-backups), apply WAL, verify |
| Redis data loss | Minor (caches + queues) | Redis restart re-populates from empty. Queues will re-process from database state. |
| API instance loss | None (orchestrator replaces) | Orchestrator starts new instance. Workers resume from persisted state. |
| Full region outage | Critical | Fail over to secondary region. Deploy from infrastructure-as-code. Restore database from off-site backup. |
| Credential compromise | Critical | Rotate all secrets immediately. Revoke all active sessions via `DELETE /api/v1/sessions/others`. Audit token reuse metrics. |

---

## 6. Deployment

### 6.1 Standard Deployment

For day-to-day deployments, follow the [Operations Runbook](runbook.md#deployment-procedures). The high-level flow:

```bash
# 1. Verify pre-deploy conditions
make test && make lint

# 2. Build and tag
docker build -t go-api-base:v$(cat VERSION) .
docker tag go-api-base:v$(cat VERSION) go-api-base:latest

# 3. Push to registry
docker push your-registry/go-api-base:v$(cat VERSION)

# 4. Deploy (orchestrator-specific)
# Docker Compose: docker-compose up -d --no-deps api
# Kubernetes: kubectl set image deployment/go-api api=your-registry/go-api-base:v$(cat VERSION)
# Nomad: nomad job run api.nomad

# 5. Verify
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz

# 6. Monitor for 5 minutes
# Watch logs, error rates, latency
```

### 6.2 Rollback

```bash
# Docker Compose
docker tag your-registry/go-api-base:v$(PREV_VERSION) go-api-base:latest
docker-compose up -d --no-deps api

# Kubernetes
kubectl rollout undo deployment/go-api

# Verify rollback
curl -s http://localhost:8080/readyz
```

If a migration was applied during the failed deployment, roll it back:

```bash
make migrate-down  # Rolls back the most recent migration
```

### 6.3 Graceful Shutdown

The application implements a 30-second graceful shutdown with this order:

1. **HTTP server** stops accepting new requests, drains in-flight connections
2. **Event bus** stops publishing new events
3. **Email worker** finishes in-flight emails (up to timeout)
4. **Webhook worker** finishes in-flight deliveries
5. **Permission enforcer** closes Casbin connections
6. **Database** and **Redis** connections close

This order ensures no data loss or orphaned background work. Your orchestrator must allow at least 30 seconds for `SIGTERM` to `SIGKILL`.

### 6.4 Zero-Downtime Deployments

Requirements:
- At least 2 API instances running behind a load balancer
- Database migrations must be backward-compatible (additive only: new columns, new tables)
- Migrations applied before new instances receive traffic
- Drain connections from old instances before termination

---

## 7. Environment-Specific Configuration

### 7.1 Development

Local development uses Docker Compose with `docker-compose.yml`. Defaults are set for convenience, not security.

```bash
# .env (development)
SERVER_PORT=8080
GIN_MODE=debug
LOG_LEVEL=debug
LOG_FORMAT=text
LOG_OUTPUTS=stdout
CACHE_DRIVER=memory
SWAGGER_ENABLED=true
ENABLE_REGISTRATION=true
DATABASE_SSL_MODE=disable
BCRYPT_COST=10
```

Run with:

```bash
cp .env.example .env
docker-compose up -d postgres redis
make migrate && make seed && make serve
```

### 7.2 Staging

Staging should mirror production as closely as possible. Use the same infrastructure, secrets manager, and deployment pipeline. Differences should be limited to:

```bash
# .env (staging)
SERVER_PORT=8080
GIN_MODE=release
LOG_LEVEL=debug
LOG_FORMAT=json
LOG_OUTPUTS=stdout,file
CACHE_DRIVER=redis
SWAGGER_ENABLED=true            # Allow API exploration in staging
ENABLE_REGISTRATION=true
EMAIL_PROVIDER=smtp
EMAIL_SMTP_HOST=mailhog.staging.internal  # Catch-all SMTP for testing
BCRYPT_COST=10                  # Lower cost acceptable for staging
WEBHOOK_ALLOW_HTTP=true         # Allow HTTP endpoints for testing
```

### 7.3 Production

Production requires strict security, minimal logging, and maximum performance. All secrets come from a secrets manager, not environment files.

```bash
# Production environment (injected via orchestrator or secrets manager)
SERVER_PORT=8080
GIN_MODE=release
LOG_LEVEL=info                   # Use 'warn' for very high traffic
LOG_FORMAT=json
LOG_OUTPUTS=stdout,file          # Also send to syslog if using centralized logging
LOG_ADD_SOURCE=false

# Security
JWT_SECRET=<from secrets manager>
JWT_ISSUER=go-api-base
JWT_AUDIENCE=api
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
BCRYPT_COST=12
REQUIRE_EMAIL_VERIFICATION=true  # Strongly recommended
ENABLE_REGISTRATION=<business decision>
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m

# Database
DATABASE_URL=postgres://<user>:<pass>@<host>:5432/<db>?sslmode=verify-full&pool_max_conns=50
# Or use individual vars:
DATABASE_HOST=<from secrets>
DATABASE_PORT=5432
DATABASE_USER=<from secrets>
DATABASE_PASSWORD=<from secrets>
DATABASE_NAME=go_api_base
DATABASE_SSL_MODE=verify-full

# Redis
REDIS_URL=rediss://:<password>@<host>:6379/0
# Or individual:
REDIS_HOST=<from secrets>
REDIS_PORT=6379
REDIS_PASSWORD=<from secrets>
REDIS_DB=0

# Cache
CACHE_DRIVER=redis
CACHE_DEFAULT_TTL=300
CACHE_PERMISSION_TTL=300
CACHE_RATE_LIMIT_TTL=60

# Storage
STORAGE_DRIVER=s3
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=<bucket name>
# Credentials from IAM roles or secrets manager
STORAGE_S3_USE_SSL=true
STORAGE_BASE_URL=https://cdn.example.com/storage

# Email
EMAIL_PROVIDER=<smtp|sendgrid|ses>
# Provider-specific credentials from secrets manager
EMAIL_SMTP_FROM_ADDRESS=noreply@example.com
EMAIL_SMTP_FROM_NAME=Your App Name
EMAIL_WORKER_CONCURRENCY=10
EMAIL_RETRY_MAX=5
EMAIL_RATE_LIMIT_PER_HOUR=100

# Webhooks
WEBHOOK_WORKER_CONCURRENCY=5
WEBHOOK_RETRY_MAX=3
WEBHOOK_RATE_LIMIT=100
WEBHOOK_ALLOW_HTTP=false
WEBHOOK_DELIVERY_TIMEOUT=10s
WEBHOOK_DELIVERY_RETENTION_DAYS=90
WEBHOOK_MAX_PAYLOAD_SIZE=1048576

# Features
SWAGGER_ENABLED=false
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com

# Password Reset
PASSWORD_RESET_TOKEN_EXPIRY=1h
```

### 7.4 Full Environment Variable Reference

Every configurable variable organized by subsystem.

#### Server

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `SERVER_PORT` | int | 8080 | No | HTTP listen port |
| `GIN_MODE` | string | debug | No | `debug` or `release` |

#### Logging

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `LOG_LEVEL` | string | info | No | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | json | No | `json` or `text` |
| `LOG_OUTPUTS` | string | stdout | No | Comma-separated: `stdout`, `file`, `syslog` |
| `LOG_FILE_PATH` | string | /var/log/api.log | No | Path when outputs includes `file` |
| `LOG_FILE_MAX_SIZE` | int | 100 | No | MB before rotation |
| `LOG_FILE_MAX_BACKUPS` | int | 10 | No | Retained backup files |
| `LOG_FILE_MAX_AGE` | int | 30 | No | Days to keep backups |
| `LOG_FILE_COMPRESS` | bool | true | No | Compress rotated files |
| `LOG_SYSLOG_NETWORK` | string | (empty) | No | `tcp`, `udp`, or empty for local |
| `LOG_SYSLOG_ADDRESS` | string | (empty) | No | Server address |
| `LOG_SYSLOG_TAG` | string | go-api | No | Syslog identifier |
| `LOG_ADD_SOURCE` | bool | false | No | Include file:line in output |

#### Database

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `DATABASE_URL` | string | (empty) | Yes* | Full PostgreSQL URL (takes precedence) |
| `DATABASE_HOST` | string | localhost | Yes* | Host (if `DATABASE_URL` not set) |
| `DATABASE_PORT` | int | 5432 | No | Port |
| `DATABASE_USER` | string | postgres | Yes* | Username |
| `DATABASE_PASSWORD` | string | (empty) | No | Password |
| `DATABASE_NAME` | string | go_api_base | Yes* | Database name |
| `DATABASE_SSL_MODE` | string | disable | No | `disable`, `require`, `verify-ca`, `verify-full` |

\* Either `DATABASE_URL` or the individual variables are required.

#### Redis

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `REDIS_URL` | string | (empty) | Yes* | Full Redis URL (takes precedence) |
| `REDIS_HOST` | string | localhost | Yes* | Host (if `REDIS_URL` not set) |
| `REDIS_PORT` | int | 6379 | No | Port |
| `REDIS_PASSWORD` | string | (empty) | No | Password (required for production) |
| `REDIS_DB` | int | 0 | No | Database index |

\* Either `REDIS_URL` or the individual variables are required.

#### JWT Authentication

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `JWT_SECRET` | string | (none) | **Yes** | Min 32 characters. Application fails to start without it. |
| `JWT_ISSUER` | string | go-api-base | No | Token issuer claim |
| `JWT_AUDIENCE` | string | api | No | Token audience claim |
| `ACCESS_TOKEN_TTL` | duration | 15m | No | Access token lifetime |
| `REFRESH_TOKEN_TTL` | duration | 720h | No | Refresh token lifetime (30 days) |
| `PASSWORD_RESET_TOKEN_EXPIRY` | string | 1h | No | Password reset token expiry |

#### Security

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `BCRYPT_COST` | int | 10 | No | Password hashing cost (10-12) |
| `RATE_LIMIT_REQUESTS` | int | 100 | No | Max requests per IP per window |
| `RATE_LIMIT_WINDOW` | duration | 1m | No | Rate limit window |
| `ENABLE_REGISTRATION` | bool | true | No | Allow new user registration |
| `REQUIRE_EMAIL_VERIFICATION` | bool | false | No | Require email verification |

#### CORS

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `CORS_ALLOWED_ORIGINS` | string | http://localhost:3000 | No | Comma-separated allowed origins. Supports `*.example.com` wildcards. |

#### Storage

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `STORAGE_DRIVER` | string | local | No | `local`, `s3`, `minio` |
| `STORAGE_LOCAL_PATH` | string | ./storage/uploads | No | Path for local driver |
| `STORAGE_BASE_URL` | string | http://localhost:8080/storage | No | Public URL prefix |
| `STORAGE_SIGNING_KEY` | string | (empty) | No | Key for signed URLs |
| `STORAGE_S3_ENDPOINT` | string | (empty) | No | S3 endpoint (empty for AWS) |
| `STORAGE_S3_REGION` | string | us-east-1 | No | AWS region |
| `STORAGE_S3_BUCKET` | string | (empty) | Yes* | Bucket name |
| `STORAGE_S3_ACCESS_KEY` | string | (empty) | Yes* | Access key |
| `STORAGE_S3_SECRET_KEY` | string | (empty) | Yes* | Secret key |
| `STORAGE_S3_USE_SSL` | bool | true | No | Use HTTPS |
| `STORAGE_S3_PATH_STYLE` | bool | false | No | Path-style addressing (true for MinIO) |

\* Required when `STORAGE_DRIVER` is `s3` or `minio`.

#### Image Compression

| Variable | Type | Default | Range | Description |
|----------|------|---------|-------|-------------|
| `IMAGE_COMPRESSION_ENABLED` | bool | true | - | Enable compression |
| `IMAGE_COMPRESSION_THUMBNAIL_QUALITY` | int | 85 | 1-100 | Thumbnail JPEG quality |
| `IMAGE_COMPRESSION_THUMBNAIL_WIDTH` | int | 300 | 1-4096 | Thumbnail width |
| `IMAGE_COMPRESSION_THUMBNAIL_HEIGHT` | int | 300 | 1-4096 | Thumbnail height |
| `IMAGE_COMPRESSION_PREVIEW_QUALITY` | int | 90 | 1-100 | Preview JPEG quality |
| `IMAGE_COMPRESSION_PREVIEW_WIDTH` | int | 800 | 1-4096 | Preview width |
| `IMAGE_COMPRESSION_PREVIEW_HEIGHT` | int | 600 | 1-4096 | Preview height |

#### Cache

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `CACHE_DRIVER` | string | redis | No | `redis`, `memory`, `none` |
| `CACHE_DEFAULT_TTL` | int | 300 | No | Default TTL in seconds |
| `CACHE_PERMISSION_TTL` | int | 300 | No | Permission cache TTL |
| `CACHE_RATE_LIMIT_TTL` | int | 60 | No | Rate limit window |

#### Email

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `EMAIL_PROVIDER` | string | smtp | No | `smtp`, `sendgrid`, `ses` |
| `EMAIL_SMTP_HOST` | string | localhost | Yes* | SMTP server |
| `EMAIL_SMTP_PORT` | int | 587 | Yes* | SMTP port |
| `EMAIL_SMTP_USER` | string | (empty) | No | SMTP username |
| `EMAIL_SMTP_PASSWORD` | string | (empty) | No | SMTP password |
| `EMAIL_SMTP_FROM_ADDRESS` | string | noreply@example.com | Yes* | From address |
| `EMAIL_SMTP_FROM_NAME` | string | Go API Base | No | From display name |
| `EMAIL_WORKER_CONCURRENCY` | int | 10 | No | Concurrent workers |
| `EMAIL_RETRY_MAX` | int | 5 | No | Max retry attempts |
| `EMAIL_RATE_LIMIT_PER_HOUR` | int | 100 | No | Max emails per user per hour |

For SendGrid, also set: `SENDGRID_API_KEY`, `SENDGRID_FROM_ADDRESS`, `SENDGRID_FROM_NAME`.
For SES, also set: `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `SES_FROM_ADDRESS`, `SES_FROM_NAME`.

#### Webhooks

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `WEBHOOK_WORKER_CONCURRENCY` | int | 5 | No | Concurrent delivery workers |
| `WEBHOOK_RETRY_MAX` | int | 3 | No | Max retry attempts |
| `WEBHOOK_RATE_LIMIT` | int | 100 | No | Deliveries per webhook per minute |
| `WEBHOOK_ALLOW_HTTP` | bool | false | No | Allow HTTP endpoints |
| `WEBHOOK_DELIVERY_TIMEOUT` | duration | 10s | No | HTTP request timeout |
| `WEBHOOK_DELIVERY_RETENTION_DAYS` | int | 90 | No | Retain delivery records |
| `WEBHOOK_MAX_PAYLOAD_SIZE` | int | 1048576 | No | Max payload bytes (1MB) |

#### Swagger

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `SWAGGER_ENABLED` | bool | true | No | Enable Swagger UI |
| `SWAGGER_PATH` | string | /swagger | No | Swagger path prefix |

---

## Appendix: Useful Production Commands

```bash
# Build for production
docker build -t go-api-base:latest .

# Run with production configs
docker run -d \
  --name go-api \
  --read-only \
  --tmpfs /tmp:mode=1777,size=64M \
  --security-opt no-new-privileges:true \
  --cap-drop ALL \
  --cap-add NET_BIND_SERVICE \
  -p 8080:8080 \
  -e GIN_MODE=release \
  -e DATABASE_URL=postgres://... \
  -e REDIS_URL=rediss://... \
  -e JWT_SECRET=... \
  go-api-base:latest

# Verify database connectivity
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT version()"

# Check active connections
docker exec go-api-postgres psql -U postgres -d go_api_base \
  -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'"

# Redis memory usage
docker exec go-api-redis redis-cli info memory | grep used_memory_human

# Force Redis snapshot
docker exec go-api-redis redis-cli BGSAVE

# Database backup
docker exec go-api-postgres pg_dump -Fc -U postgres go_api_base > backup.dump

# Database restore
docker exec -i go-api-postgres pg_restore -U postgres -d go_api_base --clean < backup.dump

# Watch API logs for errors
docker logs -f go-api-server 2>&1 | grep '"level":"ERROR"'

# Smoke test all critical endpoints
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
```

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [Operations Runbook](runbook.md) | Day-to-day operations, deployment procedures, troubleshooting |
| [Configuration Guide](configuration.md) | Storage, image, cache, Swagger configuration details |
| [Settings Feature](SETTINGS.md) | Per-org and per-user settings system |
| [Database Migrations](database-migrations.md) | Migration workflow and conventions |
| [API Examples](api-examples.md) | Common API request examples |
| [Security Headers Implementation](media-library/SECURITY.md) | Media library security configuration |

---

*This document should be reviewed and updated before each major release. The environment variable table in Section 7.4 is the canonical reference for all configuration options.*
