# Media Library Deployment Guide

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Production-Ready

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Database Migration](#database-migration)
4. [Permission Seeding](#permission-seeding)
5. [Docker Deployment](#docker-deployment)
6. [Kubernetes Deployment](#kubernetes-deployment)
7. [Health Checks](#health-checks)
8. [Production Checklist](#production-checklist)

---

## Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| Go | 1.22+ | 1.23+ |
| PostgreSQL | 14+ | 15+ |
| Redis | 6+ | 7+ |
| Disk Space | 10GB | 50GB+ |
| RAM | 2GB | 4GB+ |

### Optional: Image Processing Libraries

For optimal image conversion performance:

**Linux (Ubuntu/Debian)**:
```bash
sudo apt-get update
sudo apt-get install -y libvips-dev
```

**macOS**:
```bash
brew install vips
```

**Windows**:
- Download from [libvips website](https://www.libvips.org/install.html)
- Add to PATH environment variable

> **Note**: The system includes a Go stdlib fallback if libvips is not available. Performance will be slower but functionality remains.

---

## Environment Setup

### Required Environment Variables

```bash
# Database
DATABASE_URL=postgres://user:password@localhost:5432/go_api_base?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# JWT (must be 32+ characters)
JWT_SECRET=your-super-secret-key-here-min-32-chars-long

# Media Storage
MEDIA_STORAGE_DRIVER=local          # Options: local, s3, minio
MEDIA_STORAGE_PATH=./storage        # Local path or S3 bucket name
MEDIA_BASE_URL=http://localhost:8080 # Base URL for generated URLs

# Signed URL Secret (for download URLs)
MEDIA_SIGNING_SECRET=another-secret-key-for-signing-urls
```

### Optional Environment Variables

```bash
# File Upload Limits
MEDIA_MAX_FILE_SIZE=104857600       # 100MB in bytes (default)

# S3 Configuration (if using S3/MinIO)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
S3_BUCKET=your-bucket-name
S3_ENDPOINT=https://s3.amazonaws.com  # For MinIO: http://minio:9000
S3_USE_SSL=true
S3_PATH_STYLE=false                 # Set true for MinIO

# Conversion Settings
MEDIA_CONVERSION_WORKERS=4          # Number of async workers
MEDIA_SMALL_FILE_THRESHOLD=5242880  # 5MB threshold for sync/async
```

### Local Development .env File

```bash
# Copy from example
cp config/.env.example .env

# Edit with your settings
cat > .env << 'EOF'
DATABASE_URL=postgres://postgres:postgres@localhost:5432/go_api_base?sslmode=disable
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=$(openssl rand -base64 32)
MEDIA_STORAGE_DRIVER=local
MEDIA_STORAGE_PATH=./storage
MEDIA_BASE_URL=http://localhost:8080
MEDIA_SIGNING_SECRET=$(openssl rand -base64 32)
LOG_LEVEL=info
SERVER_PORT=8080
EOF
```

---

## Database Migration

### 1. Install Migration Tool

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate -version
```

### 2. Run Migrations

```bash
# Using Make
make migrate

# Or manually
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/go_api_base?sslmode=disable"
migrate -path migrations -database "$DATABASE_URL" up
```

### Expected Output

```
1/u init (24.567ms)
2/u audit_logs (12.345ms)
3/u permissions_update (8.901ms)
4/u media_tables (18.234ms)
```

### Migration Files

| File | Description |
|------|-------------|
| `000001_init.up.sql` | Initial schema (users, roles, permissions) |
| `000002_audit_logs.up.sql` | Audit logging table |
| `000004_media_tables.up.sql` | Media library tables |

### Rollback (if needed)

```bash
# Rollback last migration
make migrate-down

# Or manually
migrate -path migrations -database "$DATABASE_URL" down 1
```

---

## Permission Seeding

After migrations, seed the media-related permissions:

### 1. Run Permission Sync

```bash
# Using Make
make seed

# Or manually
go run cmd/api/main.go seed
```

### 2. Verify Permissions Created

```bash
# Connect to database
docker exec -it go-api-postgres psql -U postgres -d go_api_base -c "
SELECT name, resource, action FROM permissions WHERE resource = 'media';
"
```

### Expected Permissions

| Permission | Resource | Action | Description |
|------------|----------|--------|-------------|
| `media:upload` | media | upload | Upload files to models |
| `media:download` | media | download | Download media files |
| `media:delete` | media | delete | Delete media (soft delete) |
| `media:view` | media | view | View media details |
| `media:manage` | media | manage | Admin access to all media |

### Assign Permissions to Roles

```bash
# Get admin token (see API examples)
ADMIN_TOKEN="your-jwt-token"

# Get permission IDs
PERM_UPLOAD=$(curl -s http://localhost:8080/api/v1/permissions | jq -r '.data[] | select(.name=="media:upload") | .id')
PERM_DOWNLOAD=$(curl -s http://localhost:8080/api/v1/permissions | jq -r '.data[] | select(.name=="media:download") | .id')
PERM_DELETE=$(curl -s http://localhost:8080/api/v1/permissions | jq -r '.data[] | select(.name=="media:delete") | .id')

# Get admin role ID
ADMIN_ROLE=$(curl -s http://localhost:8080/api/v1/roles | jq -r '.data[] | select(.name=="Admin") | .id')

# Attach permissions to admin role
curl -X POST http://localhost:8080/api/v1/roles/$ADMIN_ROLE/permissions/$PERM_UPLOAD \
  -H "Authorization: Bearer $ADMIN_TOKEN"
  
curl -X POST http://localhost:8080/api/v1/roles/$ADMIN_ROLE/permissions/$PERM_DOWNLOAD \
  -H "Authorization: Bearer $ADMIN_TOKEN"
  
curl -X POST http://localhost:8080/api/v1/roles/$ADMIN_ROLE/permissions/$PERM_DELETE \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## Docker Deployment

### Using Docker Compose (Recommended)

```yaml
# docker-compose.yml
version: '3.8'

services:
  api:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://postgres:postgres@postgres:5432/go_api_base?sslmode=disable
      - REDIS_URL=redis://redis:6379/0
      - JWT_SECRET=${JWT_SECRET}
      - MEDIA_STORAGE_DRIVER=local
      - MEDIA_STORAGE_PATH=/app/storage
      - MEDIA_BASE_URL=http://localhost:8080
      - MEDIA_SIGNING_SECRET=${MEDIA_SIGNING_SECRET}
    volumes:
      - media-storage:/app/storage
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: go_api_base
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  media-storage:
  postgres-data:
  redis-data:
```

### Deploy with Docker Compose

```bash
# Start all services
docker-compose up -d

# Run migrations
docker-compose exec api go run cmd/api/main.go migrate

# Seed permissions
docker-compose exec api go run cmd/api/main.go seed

# Verify deployment
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

### Production Docker Build

```dockerfile
# Dockerfile
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api cmd/api/main.go

# Final image
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Create storage directory
RUN mkdir -p /app/storage && chown -R appuser:appgroup /app/storage

COPY --from=builder /app/api .

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

CMD ["./api", "serve"]
```

---

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: go-api
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-config
  namespace: go-api
data:
  SERVER_PORT: "8080"
  LOG_LEVEL: "info"
  MEDIA_STORAGE_DRIVER: "s3"
  MEDIA_BASE_URL: "https://cdn.example.com"
  GIN_MODE: "release"
```

### Secrets

```yaml
# k8s/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-secrets
  namespace: go-api
type: Opaque
stringData:
  DATABASE_URL: "postgres://user:pass@postgres:5432/go_api_base?sslmode=require"
  REDIS_URL: "redis://redis:6379/0"
  JWT_SECRET: "your-32-char-jwt-secret-here"
  MEDIA_SIGNING_SECRET: "your-signing-secret-here"
  AWS_ACCESS_KEY_ID: "your-aws-access-key"
  AWS_SECRET_ACCESS_KEY: "your-aws-secret-key"
```

### Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-api
  namespace: go-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: go-api
  template:
    metadata:
      labels:
        app: go-api
    spec:
      containers:
      - name: api
        image: your-registry/go-api:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: api-config
        - secretRef:
            name: api-secrets
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

### Service

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: go-api
  namespace: go-api
spec:
  selector:
    app: go-api
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: go-api
  namespace: go-api
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: go-api
            port:
              number: 80
```

### Apply to Kubernetes

```bash
# Apply all manifests
kubectl apply -f k8s/

# Verify deployment
kubectl get pods -n go-api
kubectl logs -f deployment/go-api -n go-api

# Run migrations
kubectl exec -it deployment/go-api -n go-api -- go run cmd/api/main.go migrate

# Seed permissions
kubectl exec -it deployment/go-api -n go-api -- go run cmd/api/main.go seed
```

---

## Health Checks

### Endpoints

| Endpoint | Purpose | Expected Response |
|----------|---------|-------------------|
| `GET /healthz` | Liveness probe | `{"status":"healthy"}` |
| `GET /readyz` | Readiness probe | `{"status":"ready","checks":{...}}` |

### Kubernetes Probe Configuration

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

### Manual Health Check

```bash
# Liveness
curl http://localhost:8080/healthz
# {"status":"healthy"}

# Readiness (checks DB + Redis)
curl http://localhost:8080/readyz
# {"status":"ready","checks":{"database":"ok","redis":"ok"}}
```

### Monitoring Metrics

Monitor these metrics in production:

| Metric | Alert Threshold |
|--------|-----------------|
| `/healthz` failure rate | > 1% for 5 min |
| `/readyz` response time | > 500ms for 5 min |
| Upload success rate | < 95% for 5 min |
| Download latency p95 | > 1000ms for 5 min |

---

## Production Checklist

### Pre-Deployment

- [ ] Set `GIN_MODE=release`
- [ ] Set `LOG_LEVEL=info` (or `warn`)
- [ ] Generate secure `JWT_SECRET` (32+ chars)
- [ ] Generate secure `MEDIA_SIGNING_SECRET` (32+ chars)
- [ ] Configure `MEDIA_STORAGE_DRIVER` (s3/minio for production)
- [ ] Set up S3 bucket / MinIO with proper CORS
- [ ] Run `make migrate` successfully
- [ ] Run `make seed` successfully
- [ ] Verify all migrations applied

### Security

- [ ] Enable SSL/TLS for database connections
- [ ] Set Redis password in production
- [ ] Configure CORS allowed origins
- [ ] Set up rate limiting
- [ ] Enable audit logging
- [ ] Configure log aggregation (Fluentd/Fluent Bit)

### Storage

- [ ] Configure S3/MinIO bucket lifecycle policies
- [ ] Set up CDN (CloudFront/Cloudflare) for media serving
- [ ] Configure backup strategy for media files
- [ ] Set up orphaned file cleanup job (cron)

### Monitoring

- [ ] Configure health check endpoints in load balancer
- [ ] Set up alerts for health check failures
- [ ] Monitor storage capacity
- [ ] Set up error tracking (Sentry/DataDog)
- [ ] Configure log retention policies

### High Availability

- [ ] Deploy multiple API replicas (2+)
- [ ] Configure load balancer
- [ ] Use managed PostgreSQL (RDS/Cloud SQL)
- [ ] Use managed Redis (ElastiCache/Memorystore)
- [ ] Configure auto-scaling policies

### Backup & Recovery

- [ ] Set up automated database backups
- [ ] Configure point-in-time recovery
- [ ] Test restore procedures
- [ ] Document recovery runbooks

---

## Post-Deployment Verification

```bash
# 1. Verify API is responding
curl http://localhost:8080/healthz

# 2. Verify readiness (DB + Redis)
curl http://localhost:8080/readyz

# 3. Test authentication
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password"}' \
  | jq -r '.data.access_token')

# 4. Test upload
curl -X POST http://localhost:8080/api/v1/models/news/$(uuidgen)/media \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "file=@test-image.jpg" \
  -F "collection=images"

# 5. Test download
MEDIA_ID=$(curl -s http://localhost:8080/api/v1/models/news/.../media | jq -r '.data[0].id')
curl http://localhost:8080/api/v1/media/$MEDIA_ID/download \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -o downloaded-image.jpg

# 6. Verify file content
file downloaded-image.jpg
```

---

## Troubleshooting Deployment

### Issue: "connection refused" to PostgreSQL

```bash
# Check PostgreSQL container
docker-compose ps postgres
docker-compose logs postgres

# Verify DATABASE_URL
echo $DATABASE_URL

# Test connection manually
docker exec -it go-api-postgres psql -U postgres -d go_api_base -c "SELECT 1;"
```

### Issue: Redis connection errors

```bash
# Check Redis container
docker-compose ps redis
docker-compose logs redis

# Test Redis connection
docker exec -it go-api-redis redis-cli ping
# Expected: PONG
```

### Issue: Permission denied on uploads

```bash
# Check storage directory permissions
docker-compose exec api ls -la /app/storage

# Fix permissions
docker-compose exec api chown -R appuser:appgroup /app/storage
```

### Issue: Migrations failing

```bash
# Check current migration version
migrate -path migrations -database "$DATABASE_URL" version

# Force version if stuck
migrate -path migrations -database "$DATABASE_URL" force 4

# Reset (DESTRUCTIVE - loses all data)
docker-compose down -v
docker-compose up -d postgres redis
make migrate
make seed
```

---

## Related Documentation

- [Architecture](./ARCHITECTURE.md) - System design and components
- [Configuration](./CONFIG.md) - Environment variables reference
- [API Reference](./API.md) - Complete endpoint documentation
- [Security](./SECURITY.md) - Security features and hardening
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
