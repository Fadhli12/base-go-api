# Media Library Troubleshooting Guide

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Production-Ready

---

## Table of Contents

1. [Common Issues](#common-issues)
2. [Error Code Reference](#error-code-reference)
3. [Log Analysis](#log-analysis)
4. [Performance Tuning](#performance-tuning)
5. [Monitoring](#monitoring)
6. [Debugging Tips](#debugging-tips)
7. [Recovery Procedures](#recovery-procedures)

---

## Common Issues

### Upload Failures

#### "File size exceeds maximum allowed size"

**Symptoms**: Upload returns 422 VALIDATION_ERROR

**Diagnosis**:
```bash
# Check current limit
grep MEDIA_MAX_FILE_SIZE .env

# Check file size
ls -lh /path/to/file.jpg
```

**Resolution**:
```bash
# Increase limit in .env
MEDIA_MAX_FILE_SIZE=209715200  # 200MB

# Restart server
make serve

# Update nginx/ingress if using proxy
# nginx.conf:
client_max_body_size 200m;
```

---

#### "File type not allowed"

**Symptoms**: Upload rejected despite correct extension

**Diagnosis**:
```bash
# Check actual MIME type
file --mime-type /path/to/file.jpg

# Check magic bytes
xxd -l 4 /path/to/file.jpg
```

**Resolution**:
1. Verify file is not corrupted
2. Check if MIME type is in allowlist:
   - `internal/service/media.go`
   - `AllowedImageTypes` / `AllowedDocumentTypes`
3. Re-encode file if needed:
   ```bash
   convert corrupted.jpg -quality 90 fixed.jpg
   ```

---

#### "Failed to store file" (Storage Error)

**Symptoms**: Upload fails at storage stage

**Diagnosis**:
```bash
# Check storage directory permissions
ls -la ./storage/

# Check disk space
df -h

# Check storage logs
docker-compose logs api | grep -i storage
```

**Resolution**:
```bash
# Fix permissions
chmod 755 ./storage
chown -R appuser:appgroup ./storage

# Clean up disk space
docker system prune -a

# Check storage driver config
grep MEDIA_STORAGE .env
```

---

### Download Failures

#### "Media not found"

**Symptoms**: 404 on download

**Diagnosis**:
```bash
# Check if media exists in DB
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "SELECT id, deleted_at FROM media WHERE id = 'xxx';"

# Check if file exists on disk
ls ./storage/news/xxx/xxx.jpg
```

**Resolution**:
```bash
# If soft deleted, restore from backup or un-delete
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "UPDATE media SET deleted_at = NULL WHERE id = 'xxx';"

# If file missing, restore from backup
cp /backup/storage/news/xxx/xxx.jpg ./storage/news/xxx/
```

---

#### "Invalid or expired signature" (Signed URL)

**Symptoms**: 403 on signed URL access

**Diagnosis**:
```bash
# Check URL expiry timestamp
echo "expires=1703341200" | xargs -I {} date -d @{}  # Linux
echo "expires=1703341200" | ForEach-Object { (Get-Date 1970-01-01).AddSeconds($_) }  # PowerShell

# Check signing secret consistency
grep MEDIA_SIGNING_SECRET .env
```

**Resolution**:
```bash
# Generate new signed URL with fresh expiry
curl "http://localhost:8080/api/v1/media/$MEDIA_ID/url?expires_in=3600" \
  -H "Authorization: Bearer $TOKEN"

# Verify signing secrets match across instances
```

---

#### "Permission denied"

**Symptoms**: 403 despite valid token

**Diagnosis**:
```bash
# Check user's effective permissions
curl http://localhost:8080/api/v1/users/$USER_ID/effective-permissions \
  -H "Authorization: Bearer $TOKEN"

# Check if media:download permission exists
curl http://localhost:8080/api/v1/permissions \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.data[] | select(.name == "media:download")'
```

**Resolution**:
```bash
# Assign permission to user/role
PERM_ID=$(curl -s http://localhost:8080/api/v1/permissions | jq -r '.data[] | select(.name == "media:download") | .id')
ROLE_ID=$(curl -s http://localhost:8080/api/v1/roles | jq -r '.data[] | select(.name == "User") | .id')

curl -X POST "http://localhost:8080/api/v1/roles/$ROLE_ID/permissions/$PERM_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Sync permissions
go run cmd/api/main.go permission:sync
```

---

### Conversion Issues

#### "Thumbnail not generated"

**Symptoms**: Media uploaded but conversions missing

**Diagnosis**:
```bash
# Check if conversion worker running
docker-compose ps

# Check Redis queue
docker exec go-api-redis redis-cli llen media:conversions:queue

# Check conversion logs
docker-compose logs api | grep -i conversion

# Verify libvips installed
docker exec -it go-api-api vips --version
```

**Resolution**:
```bash
# Install libvips (if missing)
# Ubuntu/Debian:
docker exec -it go-api-api apt-get update && apt-get install -y libvips-dev

# Restart worker
docker-compose restart api

# Manually trigger conversion (admin)
curl -X POST "http://localhost:8080/api/v1/admin/media/$MEDIA_ID/convert" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

#### "Conversion taking too long"

**Symptoms**: Large files slow to convert

**Diagnosis**:
```bash
# Check conversion queue depth
docker exec go-api-redis redis-cli llen media:conversions:queue

# Check worker CPU usage
docker stats go-api-api

# Check file sizes
ls -lhS ./storage/news/
```

**Resolution**:
```bash
# Increase worker count
MEDIA_CONVERSION_WORKERS=8

# Adjust sync threshold
MEDIA_SMALL_FILE_THRESHOLD=10485760  # 10MB

# Add more resources to API containers
# docker-compose.yml:
services:
  api:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
```

---

### Database Issues

#### "Failed to create media record"

**Symptoms**: File stored but no DB entry

**Diagnosis**:
```bash
# Check DB connection
docker-compose logs postgres | tail -50

# Check media table
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT COUNT(*) FROM media;"

# Check migrations
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT * FROM schema_migrations;"
```

**Resolution**:
```bash
# Run missing migrations
make migrate

# Verify tables exist
docker exec go-api-postgres psql -U postgres -d go_api_base -c "\\dt media*"

# Check constraints
docker exec go-api-postgres psql -U postgres -d go_api_base -c "SELECT * FROM information_schema.table_constraints WHERE table_name = 'media';"
```

---

#### "Duplicate filename" error

**Symptoms**: Upload fails with unique constraint error

**Diagnosis**:
```bash
# Check for duplicate
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "SELECT id, disk, filename, deleted_at FROM media WHERE filename = 'xxx.jpg';"
```

**Resolution**:
```bash
# Files with same name but deleted are OK (soft delete)
# If hard delete needed (admin only):
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "DELETE FROM media WHERE id = 'xxx';"
```

---

## Error Code Reference

### HTTP Status Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| **400** | Bad Request | Invalid file, malformed request |
| **401** | Unauthorized | Missing/invalid JWT, expired signature |
| **403** | Forbidden | No permission, invalid signed URL |
| **404** | Not Found | Media doesn't exist, wrong ID |
| **410** | Gone | Media soft-deleted |
| **422** | Validation Error | File too large, wrong type |
| **429** | Rate Limited | Too many requests |
| **500** | Internal Error | Server bug, storage failure |
| **503** | Service Unavailable | DB/Redis down, storage offline |

### Application Error Codes

| Code | Description | Resolution |
|------|-------------|------------|
| `FILE_TOO_LARGE` | Exceeds MEDIA_MAX_FILE_SIZE | Compress file or increase limit |
| `INVALID_MIME_TYPE` | Not in allowlist | Convert to allowed format |
| `BLOCKED_EXTENSION` | Dangerous file type | Remove/rename extension |
| `STORAGE_ERROR` | Backend failure | Check storage connectivity |
| `PATH_TRAVERSAL` | Unsafe filename | Rename file |
| `QUOTA_EXCEEDED` | Storage limit | Delete old files or upgrade |
| `CONVERSION_FAILED` | Thumbnail error | Check libvips, retry |
| `SIGNED_URL_INVALID` | Bad signature/expiry | Generate new URL |
| `DATABASE_ERROR` | DB connection | Check DB health |
| `PERMISSION_DENIED` | RBAC check failed | Assign permissions |

---

## Log Analysis

### Log Locations

**Docker**:
```bash
# Application logs
docker-compose logs -f api

# Database logs
docker-compose logs postgres

# Redis logs
docker-compose logs redis
```

**File System**:
```bash
# If logging to file
tail -f /var/log/go-api/app.log
```

### Log Levels

| Level | Use Case | Example |
|-------|----------|---------|
| `DEBUG` | Development troubleshooting | File validation details |
| `INFO` | Normal operations (default) | Upload success, download |
| `WARN` | Recoverable issues | Slow conversion, retry |
| `ERROR` | Failures requiring attention | Storage failure, DB error |

### Common Log Patterns

**Successful Upload**:
```json
{
  "level": "INFO",
  "msg": "Media uploaded successfully",
  "media_id": "550e8400-e29b-41d4-a716-446655440001",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "model_type": "news",
  "size": 1048576,
  "time": "2026-04-22T18:30:00Z"
}
```

**Failed Upload**:
```json
{
  "level": "ERROR",
  "msg": "Failed to store file",
  "error": "permission denied",
  "path": "news/xxx/xxx.jpg",
  "time": "2026-04-22T18:30:00Z"
}
```

**Permission Denied**:
```json
{
  "level": "WARN",
  "msg": "Permission denied",
  "user_id": "xxx",
  "action": "download",
  "media_id": "xxx",
  "time": "2026-04-22T18:30:00Z"
}
```

### Log Queries

**Find recent uploads**:
```bash
docker-compose logs api | grep "uploaded successfully"
```

**Find errors**:
```bash
docker-compose logs api | grep -i error
```

**Find specific user activity**:
```bash
docker-compose logs api | grep "user_id.*xxx"
```

**Find slow operations** (>1s):
```bash
docker-compose logs api | grep -E "(upload|download).*latency.*[0-9]{4,}"
```

---

## Performance Tuning

### Upload Performance

**Optimization Checklist**:

1. **Enable concurrent uploads**:
   ```go
   // Client-side
   const maxConcurrent = 3
   ```

2. **Adjust server limits**:
   ```bash
   # .env
   MEDIA_MAX_FILE_SIZE=52428800  # 50MB (smaller = faster validation)
   MEDIA_CONVERSION_WORKERS=8    # More workers for parallel processing
   ```

3. **Database connection pool**:
   ```go
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)
   ```

4. **Storage optimization**:
   - Use SSD for local storage
   - Enable S3 transfer acceleration
   - Use multipart upload for large files

### Download Performance

**Optimization**:

1. **CDN integration**:
   ```bash
   MEDIA_BASE_URL=https://cdn.example.com
   ```

2. **HTTP caching headers**:
   ```go
   c.Response().Header().Set("Cache-Control", "public, max-age=86400")
   ```

3. **Signed URL caching**:
   - Cache signed URLs client-side
   - Reuse until expiry

### Database Performance

**Indexes** (already in migration):
```sql
-- Ensure indexes exist
CREATE INDEX CONCURRENTLY idx_media_model ON media(model_type, model_id) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY idx_media_collection ON media(collection_name) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY idx_media_created_at ON media(created_at DESC) WHERE deleted_at IS NULL;
```

**Query optimization**:
```sql
-- Use pagination
SELECT * FROM media 
WHERE model_type = 'news' AND model_id = 'xxx'
ORDER BY created_at DESC
LIMIT 20 OFFSET 0;

-- Use collection filter
SELECT * FROM media 
WHERE model_type = 'news' 
  AND model_id = 'xxx'
  AND collection_name = 'images'
  AND deleted_at IS NULL;
```

---

## Monitoring

### Key Metrics

| Metric | Alert Threshold | Command |
|--------|-----------------|---------|
| Upload success rate | < 95% | `rate(upload_success[5m])` |
| Download latency p95 | > 1000ms | `histogram_quantile(0.95, rate(download_duration[5m]))` |
| Conversion queue depth | > 100 | `redis-cli llen media:conversions:queue` |
| Storage utilization | > 80% | `df -h` |
| Failed authentication | > 10/min | `grep "401" /var/log/app.log` |
| Database connections | > 80% | `SELECT count(*) FROM pg_stat_activity;` |

### Health Checks

**Liveness**:
```bash
curl http://localhost:8080/healthz
# Expected: {"status":"healthy"}
```

**Readiness**:
```bash
curl http://localhost:8080/readyz
# Expected: {"status":"ready","checks":{"database":"ok","redis":"ok"}}
```

**Storage**:
```bash
# Test storage connectivity
curl -X POST http://localhost:8080/api/v1/models/news/test-id/media \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test.txt"
```

### Dashboard Setup

**Prometheus metrics** (if exposed):
```yaml
# Example metrics
media_uploads_total{status="success"}
media_uploads_total{status="error"}
media_downloads_total{status="success"}
media_download_duration_seconds_bucket
```

**Grafana panels**:
- Upload rate (requests/min)
- Download latency (95th percentile)
- Storage utilization (%)
- Conversion queue depth
- Error rate by type

---

## Debugging Tips

### Enable Debug Logging

```bash
# .env
LOG_LEVEL=debug

# Restart
make serve
```

### Test Storage Driver

```go
// Debug script
package main

import (
    "context"
    "fmt"
    "strings"
    "github.com/example/go-api-base/internal/storage"
)

func main() {
    driver, _ := storage.NewDriver(storage.Config{
        Type:      "local",
        LocalPath: "./storage",
        BaseURL:   "http://localhost:8080",
    })
    
    ctx := context.Background()
    
    // Test store
    err := driver.Store(ctx, "test/file.txt", strings.NewReader("test"))
    fmt.Println("Store:", err)
    
    // Test get
    reader, err := driver.Get(ctx, "test/file.txt")
    fmt.Println("Get:", err)
    if reader != nil { reader.Close() }
    
    // Test exists
    exists, _ := driver.Exists(ctx, "test/file.txt")
    fmt.Println("Exists:", exists)
}
```

### Database Debugging

```bash
# Enable query logging
docker exec go-api-postgres psql -U postgres -d go_api_base -c "
ALTER SYSTEM SET log_statement = 'all';
SELECT pg_reload_conf();
"

# View slow queries
docker exec go-api-postgres psql -U postgres -d go_api_base -c "
SELECT query, mean_time, calls
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;
"
```

### Redis Debugging

```bash
# Monitor queue
docker exec go-api-redis redis-cli monitor

# Check queue contents
docker exec go-api-redis redis-cli lrange media:conversions:queue 0 -1

# Check locks
docker exec go-api-redis redis-cli keys "media:conversions:lock:*"

# Flush queue (caution!)
docker exec go-api-redis redis-cli del media:conversions:queue
```

### Network Debugging

```bash
# Test connectivity to S3
curl -I https://s3.amazonaws.com/your-bucket

# Check DNS resolution
nslookup s3.amazonaws.com

# Trace route
traceroute s3.amazonaws.com
```

---

## Recovery Procedures

### Restore Deleted Media

**Scenario**: User accidentally deleted media

**Steps**:
```bash
# 1. Find soft-deleted media
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "SELECT id, original_filename, deleted_at FROM media WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC LIMIT 10;"

# 2. Restore specific media
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "UPDATE media SET deleted_at = NULL, orphaned_at = NULL WHERE id = 'xxx';"

# 3. Verify
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "SELECT id, deleted_at FROM media WHERE id = 'xxx';"

# 4. Test download
curl "http://localhost:8080/api/v1/media/xxx/download" \
  -H "Authorization: Bearer $TOKEN" \
  -o restored.jpg
```

### Recover From Storage Failure

**Scenario**: Storage disk crashed

**Steps**:
```bash
# 1. Stop application
docker-compose stop api

# 2. Restore from backup
# Local storage:
tar -xzf backup-2026-04-22.tar.gz -C ./storage/

# S3:
aws s3 sync s3://backup-bucket/media ./storage/

# 3. Verify files exist
ls -la ./storage/

# 4. Restart application
docker-compose up -d api

# 5. Verify with admin stats
curl http://localhost:8080/api/v1/admin/media/stats \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Fix Orphaned Files

**Scenario**: Files exist in storage but no DB records

**Steps**:
```bash
# 1. Identify orphaned files
# Compare storage directory with database
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "SELECT path FROM media WHERE deleted_at IS NULL;" > db_files.txt

find ./storage -type f | sort > storage_files.txt

# 2. Review differences
diff db_files.txt storage_files.txt

# 3. Delete orphaned files (manual review first!)
# Or re-create DB records if files are valid
```

### Rebuild Conversion Cache

**Scenario**: Conversion files missing or corrupted

**Steps**:
```bash
# 1. Clear existing conversions
docker exec go-api-postgres psql -U postgres -d go_api_base -c \
  "DELETE FROM media_conversions WHERE media_id = 'xxx';"

# 2. Delete corrupted files
rm ./storage/news/xxx/conversions/*

# 3. Trigger reconversion (admin)
curl -X POST "http://localhost:8080/api/v1/admin/media/xxx/regenerate" \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Or manually queue
redis-cli rpush media:conversions:queue '{"media_id":"xxx","conversion":"thumbnail"}'
```

---

## Getting Help

### Before Requesting Support

1. Check logs: `docker-compose logs api | grep -i error`
2. Verify health: `curl http://localhost:8080/readyz`
3. Test storage: Upload small test file
4. Check permissions: Verify RBAC setup

### Information to Provide

- Error messages (exact text)
- Reproduction steps
- Environment: Docker vs local, storage driver
- Logs around time of issue
- Configuration (sanitized secrets)

### Resources

- [API Reference](./API.md) - Endpoint documentation
- [Configuration](./CONFIG.md) - Environment variables
- [Security](./SECURITY.md) - Security considerations
- [Architecture](./ARCHITECTURE.md) - System design

---

## Related Documentation

- [Architecture](./ARCHITECTURE.md) - System design and components
- [Deployment](./DEPLOYMENT.md) - Production deployment guide
- [Configuration](./CONFIG.md) - Environment variables reference
- [Security](./SECURITY.md) - Security features and hardening
- [API Reference](./API.md) - Complete endpoint documentation
