# Go API Base - Operations Runbook

This runbook provides operational procedures for deploying, monitoring, and troubleshooting the Go API Base application.

## Table of Contents

1. [Deployment Procedures](#deployment-procedures)
2. [Monitoring](#monitoring)
3. [Rollback Procedures](#rollback-procedures)
4. [Common Troubleshooting](#common-troubleshooting)
5. [Logging and Log Aggregation](#logging-and-log-aggregation)
6. [Security Considerations](#security-considerations)

---

## Deployment Procedures

### Pre-Deployment Checklist

- [ ] All tests passing (`make test`)
- [ ] Linting clean (`make lint`)
- [ ] Database migrations reviewed
- [ ] Environment variables configured
- [ ] JWT_SECRET generated (min 32 characters)
- [ ] Health check endpoints verified
- [ ] SSL certificates valid

### Standard Deployment

```bash
# 1. Pull latest code
git pull origin main

# 2. Run migrations (if any)
make migrate

# 3. Build Docker image
make docker-build

# 4. Deploy with zero downtime
docker-compose up -d --no-deps --build api

# 5. Verify health
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Blue-Green Deployment

```bash
# 1. Build new image with version tag
docker build -t go-api-base:v1.2.0 .

# 2. Tag as latest
docker tag go-api-base:v1.2.0 go-api-base:latest

# 3. Update docker-compose with new image
# Edit docker-compose.yml: image: go-api-base:v1.2.0

# 4. Deploy new version
docker-compose up -d --no-deps api

# 5. Monitor health during transition
watch -n 2 'curl -s http://localhost:8080/readyz'
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `REDIS_URL` | Yes | Redis connection string |
| `JWT_SECRET` | Yes | JWT signing secret (32+ chars) |
| `ACCESS_TOKEN_TTL` | No | Access token lifetime (default: 15m) |
| `REFRESH_TOKEN_TTL` | No | Refresh token lifetime (default: 168h) |
| `LOG_LEVEL` | No | Log level (default: info) |
| `GIN_MODE` | No | Gin mode (default: debug) |

---

## Monitoring

### Health Endpoints

| Endpoint | Purpose | Expected Response |
|----------|---------|-------------------|
| `GET /healthz` | Liveness probe | `{"status": "healthy"}` |
| `GET /readyz` | Readiness probe | `{"status": "ready"}` |

### Health Check Configuration

```yaml
# Kubernetes liveness probe
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

# Kubernetes readiness probe
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

### Metrics

The application exposes structured logs that can be parsed for metrics:

- Request count per endpoint
- Response time per request
- Error rates by status code
- Database connection pool status
- Redis connection status

### Recommended Alerts

| Alert | Condition | Severity |
|-------|-----------|----------|
| API Down | `/healthz` fails 3 times | Critical |
| High Latency | p95 > 500ms for 5 min | Warning |
| Database Connection | `/readyz` returns 503 | Critical |
| Redis Connection | `/readyz` returns 503 | Critical |
| High Error Rate | 5xx > 5% for 5 min | Warning |

---

## Rollback Procedures

### Immediate Rollback

```bash
# 1. Stop current deployment
docker-compose stop api

# 2. Tag previous version as current
docker tag go-api-base:previous go-api-base:latest

# 3. Start previous version
docker-compose up -d api

# 4. Verify health
curl http://localhost:8080/healthz
```

### Database Migration Rollback

```bash
# Check current migration version
make migrate-down

# Or with explicit version
migrate -path ./migrations -database "${DATABASE_URL}" down 1
```

### Rollback Decision Tree

```
Issue Detected
    │
    ├── Application Error?
    │   └── Yes → Check logs → Fix or rollback
    │
    ├── Database Error?
    │   └── Yes → Rollback migration → Investigate
    │
    └── Infrastructure Error?
        └── Yes → Check Docker/network → Escalate
```

---

## Common Troubleshooting

### API Not Starting

**Symptoms:** Container exits immediately or won't start

**Diagnosis:**
```bash
# Check container logs
docker logs go-api-server

# Check exit code
docker inspect go-api-server --format='{{.State.ExitCode}}'
```

**Common Causes:**
1. Database connection refused
   - Verify PostgreSQL is running: `docker ps`
   - Check DATABASE_URL is correct
   - Ensure network connectivity

2. Invalid configuration
   - Validate .env file exists
   - Check for typos in environment variables

3. Permission denied
   - Check file permissions
   - Verify volume mounts

### Database Connection Issues

**Symptoms:** `/readyz` returns 503, logs show connection errors

**Diagnosis:**
```bash
# Test database connectivity
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT 1"

# Check connection string
echo $DATABASE_URL
```

**Resolution:**
```bash
# Restart database
docker-compose restart postgres

# Wait for health check
docker-compose logs postgres | grep "ready"
```

### Redis Connection Issues

**Symptoms:** Permission cache errors, session issues

**Diagnosis:**
```bash
# Test Redis connectivity
docker exec go-api-redis redis-cli ping

# Check Redis info
docker exec go-api-redis redis-cli info
```

**Resolution:**
```bash
# Restart Redis
docker-compose restart redis

# Clear cache if corrupted
docker exec go-api-redis redis-cli FLUSHALL
```

### High Memory Usage

**Symptoms:** OOM kills, slow performance

**Diagnosis:**
```bash
# Check container stats
docker stats go-api-server

# Check for goroutine leaks
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

**Resolution:**
1. Restart the container
2. Review recent code changes
3. Check for unclosed connections

### Slow Response Times

**Symptoms:** High latency, timeouts

**Diagnosis:**
```bash
# Check database query times
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT * FROM pg_stat_activity"

# Check for slow queries
# Enable query logging temporarily
```

**Resolution:**
1. Add appropriate indexes
2. Check for N+1 queries
3. Enable connection pooling

---

## Logging and Log Aggregation

### Log Format

The application outputs structured JSON logs:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "Request processed",
  "method": "GET",
  "path": "/api/v1/users",
  "status": 200,
  "latency_ms": 45,
  "request_id": "abc123"
}
```

### Log Levels

| Level | Usage |
|-------|-------|
| `DEBUG` | Development troubleshooting |
| `INFO` | Normal operations (default) |
| `WARN` | Recoverable errors |
| `ERROR` | Failures requiring attention |

### Log Collection

**Docker logs:**
```bash
# Stream logs
docker logs -f go-api-server

# Filter by level
docker logs go-api-server 2>&1 | grep '"level":"ERROR"'

# Last N lines
docker logs --tail 100 go-api-server
```

**Integration with log aggregators:**

```yaml
# Fluentd example
<source>
  @type forward
  port 24224
</source>

<filter docker.**>
  @type parser
  key_name log
  <parse>
    @type json
  </parse>
</filter>
```

### Log Retention

- Development: 7 days
- Staging: 14 days
- Production: 90 days (compliance)

---

## Security Considerations

### Container Security

- **Non-root user**: Application runs as `appuser:appgroup`
- **Read-only filesystem**: Consider mounting volumes read-only
- **No privilege escalation**: Container security options configured

### Network Security

- **Internal network**: API only accessible via defined ports
- **Database isolation**: PostgreSQL not exposed externally
- **Redis isolation**: Redis not exposed externally

### Secrets Management

**DO NOT:**
- Commit secrets to version control
- Use default JWT secrets in production
- Store plaintext credentials in .env

**DO:**
- Use Kubernetes Secrets or HashiCorp Vault
- Rotate secrets regularly
- Use environment-specific secrets

### Security Checklist

- [ ] JWT_SECRET is at least 32 characters
- [ ] Database SSL enabled in production
- [ ] Redis password set in production
- [ ] Rate limiting configured
- [ ] CORS configured appropriately
- [ ] Input validation enabled
- [ ] Audit logging enabled

### Incident Response

1. **Identify**: Check logs and metrics for anomalies
2. **Contain**: Isolate affected services, revoke compromised tokens
3. **Eradicate**: Fix vulnerability, update credentials
4. **Recover**: Restore from clean state if needed
5. **Review**: Post-incident analysis and documentation

---

## Emergency Contacts

| Role | Contact |
|------|---------|
| On-call Engineer | [Configure in your system] |
| Security Team | [Configure in your system] |
| Database Admin | [Configure in your system] |

---

## Appendix

### Useful Commands

```bash
# View all container logs
docker-compose logs -f

# Restart specific service
docker-compose restart api

# Check resource usage
docker stats

# Execute shell in container
docker exec -it go-api-server sh

# Database backup
docker exec go-api-postgres pg_dump -U postgres go_api_base > backup.sql

# Database restore
docker exec -i go-api-postgres psql -U postgres go_api_base < backup.sql
```

### Architecture Overview

```
                    ┌─────────────┐
                    │   Client    │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  API Server │ (Port 8080)
                    │  go-api     │
                    └──────┬──────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
     ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐
     │ PostgreSQL │ │   Redis   │ │   Casbin    │
     │   (5432)   │ │  (6379)   │ │   Policy    │
     └─────────────┘ └───────────┘ └─────────────┘
```

---

*Last updated: January 2024*