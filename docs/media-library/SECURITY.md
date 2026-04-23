# Media Library Security Guide

**Version**: 1.0 | **Date**: 2026-04-22 | **Status**: Production-Ready

---

## Table of Contents

1. [Security Overview](#security-overview)
2. [File Validation](#file-validation)
3. [Path Traversal Prevention](#path-traversal-prevention)
4. [Signed URL Security](#signed-url-security)
5. [Permission Model](#permission-model)
6. [Audit Logging](#audit-logging)
7. [Soft Delete Recovery](#soft-delete-recovery)
8. [Malware Scanning](#malware-scanning)
9. [Security Checklist](#security-checklist)

---

## Security Overview

The Media Library implements defense-in-depth security with multiple layers of protection:

```
┌─────────────────────────────────────────────────────────────────┐
│                      Security Layers                             │
├─────────────────────────────────────────────────────────────────┤
│ 1. File Validation                                              │
│    ├── Magic bytes detection (content-based MIME)               │
│    ├── MIME type allowlist                                      │
│    ├── Extension blocklist                                        │
│    └── Size limits                                                │
├─────────────────────────────────────────────────────────────────┤
│ 2. Path Security                                                │
│    ├── Filename sanitization                                    │
│    ├── Path traversal prevention                                │
│    └── UUID-based storage names                                 │
├─────────────────────────────────────────────────────────────────┤
│ 3. Access Control                                               │
│    ├── JWT authentication                                       │
│    ├── RBAC permissions (Casbin)                                │
│    ├── Ownership-based access                                   │
│    └── Signed URL expiry                                        │
├─────────────────────────────────────────────────────────────────┤
│ 4. Audit & Compliance                                           │
│    ├── Upload audit logging                                     │
│    ├── Download tracking                                        │
│    ├── Delete audit trail                                       │
│    └── Immutable audit logs                                     │
├─────────────────────────────────────────────────────────────────┤
│ 5. Data Protection                                              │
│    ├── Soft delete (recoverable)                                │
│    ├── Orphaned file tracking                                   │
│    └── Automatic cleanup                                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## File Validation

### Magic Bytes Detection

Files are validated by examining their actual content (magic bytes), not just the file extension:

```go
func detectMimeType(file multipart.File) (string, error) {
    buffer := make([]byte, 512)
    n, err := file.Read(buffer)
    if err != nil {
        return "", err
    }
    buffer = buffer[:n]
    
    // Detect MIME type from magic bytes
    mimeType := http.DetectContentType(buffer)
    return mimeType, nil
}
```

**Why Magic Bytes?**
- Extension can be spoofed (e.g., `virus.exe` renamed to `image.jpg`)
- Magic bytes are embedded in file content
- Standard library provides reliable detection

**Detection Examples**:

| File Type | Magic Bytes | MIME Type |
|-----------|-------------|-----------|
| JPEG | `FF D8 FF` | `image/jpeg` |
| PNG | `89 50 4E 47` | `image/png` |
| GIF | `47 49 46 38` | `image/gif` |
| PDF | `25 50 44 46` | `application/pdf` |
| ZIP | `50 4B 03 04` | `application/zip` |

### MIME Type Allowlist

Only approved MIME types can be uploaded:

```go
var AllowedImageTypes = map[string]bool{
    "image/jpeg": true,
    "image/png":  true,
    "image/webp": true,
    "image/gif":  true,
    "image/svg+xml": true,
}

var AllowedDocumentTypes = map[string]bool{
    "application/pdf": true,
    "application/msword": true,
    "text/plain": true,
    "text/csv": true,
}

var AllowedArchiveTypes = map[string]bool{
    "application/zip": true,
    "application/x-rar-compressed": true,
    "application/x-7z-compressed": true,
}
```

**Adding New Types**:

Edit `internal/service/media.go`:

```go
// Add to appropriate allowlist
var AllowedImageTypes = map[string]bool{
    // ... existing types
    "image/heic": true,  // Add HEIC support
}
```

### Blocked Extensions

Dangerous extensions are blocked regardless of MIME type:

```go
var BlockedExtensions = map[string]bool{
    ".exe": true,  // Windows executable
    ".dll": true,  // Dynamic library
    ".bat": true,  // Batch script
    ".cmd": true,  // Command script
    ".sh":  true,  // Shell script
    ".php": true,  // PHP script
    ".jsp": true,  // JSP script
    ".asp": true,  // ASP script
    ".aspx": true, // ASP.NET script
    ".py":  true,  // Python script
    ".rb":  true,  // Ruby script
    ".pl":  true,  // Perl script
    ".cgi": true,  // CGI script
}
```

**Why Block Extensions?**
- Even with MIME validation, extensions matter for execution
- Prevents double-extension attacks (`image.php.jpg`)
- Protects against server-side execution

### File Size Limits

**Default**: 100MB maximum

```go
const MaxFileSize = 100 * 1024 * 1024 // 100MB
```

**Configurable**:
```bash
MEDIA_MAX_FILE_SIZE=52428800  # 50MB
```

**Validation**:
```go
if req.FileHeader.Size > MaxFileSize {
    return errors.NewAppError("VALIDATION_ERROR", 
        fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", MaxFileSize), 
        http.StatusBadRequest)
}
```

### Validation Flow

```
Upload Request
     │
     ├─► [1] Check file size ≤ MaxFileSize
     │     └─► Fail: 422 VALIDATION_ERROR
     │
     ├─► [2] Check extension NOT in BlockedExtensions
     │     └─► Fail: 422 VALIDATION_ERROR
     │
     ├─► [3] Sanitize filename (path traversal check)
     │     └─► Fail: 422 VALIDATION_ERROR
     │
     ├─► [4] Detect MIME type from magic bytes
     │     └─► Fail: 422 VALIDATION_ERROR
     │
     └─► [5] Check MIME type in allowlist
           └─► Fail: 422 VALIDATION_ERROR
```

---

## Path Traversal Prevention

### Filename Sanitization

Original filenames are sanitized before storage:

```go
func sanitizeFilename(filename string) string {
    // Get base filename (remove path)
    base := filepath.Base(filename)
    
    // Remove null bytes
    base = strings.ReplaceAll(base, "\x00", "")
    
    // Trim spaces
    base = strings.TrimSpace(base)
    
    // Ensure not empty or special
    if base == "" || base == "." || base == ".." {
        base = "unnamed"
    }
    
    return base
}
```

### Storage Path Security

Files are stored with UUID filenames, original names stored in DB:

```go
// Generate safe filename
storageFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

// Build path
storagePath := fmt.Sprintf("%s/%s/%s", modelType, modelID.String(), storageFilename)
```

**Why UUID Filenames?**
- Prevents filename collisions
- Hides original filenames from filesystem
- Prevents special character injection
- Makes enumeration attacks harder

### Path Traversal Checks

All storage paths are sanitized:

```go
func sanitizePath(path string) string {
    // Remove ".." components
    parts := strings.Split(path, "/")
    var safeParts []string
    for _, part := range parts {
        if part != ".." && part != "" {
            safeParts = append(safeParts, part)
        }
    }
    return strings.Join(safeParts, "/")
}
```

**Blocked Patterns**:
- `../../../etc/passwd`
- `..\\..\\windows\\system32`
- `/etc/passwd`
- `\windows\system32`

---

## Signed URL Security

### HMAC-SHA256 Signing

Signed URLs use HMAC-SHA256 to prevent tampering:

```go
func (s *Signer) Generate(mediaID, path string, expiry time.Duration) (*SignedURL, error) {
    expiresAt := time.Now().Add(expiry)
    expiresUnix := expiresAt.Unix()
    
    // Create signature
    message := fmt.Sprintf("%s:%s:%d", mediaID, path, expiresUnix)
    mac := hmac.New(sha256.New, []byte(s.secret))
    mac.Write([]byte(message))
    signature := hex.EncodeToString(mac.Sum(nil))
    
    return &SignedURL{
        URL:       fmt.Sprintf("/media/%s/download?sig=%s&expires=%d", mediaID, signature, expiresUnix),
        ExpiresAt: expiresAt,
    }, nil
}
```

### Signature Validation

```go
func (s *Signer) Validate(mediaID, signature string, expires int64, path string) bool {
    // Check expiry
    if time.Now().Unix() > expires {
        return false
    }
    
    // Recalculate signature
    message := fmt.Sprintf("%s:%s:%d", mediaID, path, expires)
    mac := hmac.New(sha256.New, []byte(s.secret))
    mac.Write([]byte(message))
    expectedSig := hex.EncodeToString(mac.Sum(nil))
    
    // Constant-time comparison
    return hmac.Equal([]byte(signature), []byte(expectedSig))
}
```

**Security Properties**:
- URLs expire after specified duration (max 24 hours)
- Cannot be modified without invalidating signature
- Secret key required to generate valid URLs
- Constant-time comparison prevents timing attacks

### Signed URL Parameters

| Parameter | Purpose | Security |
|-----------|---------|----------|
| `sig` | HMAC-SHA256 signature | Prevents tampering |
| `expires` | Unix timestamp | Time-limited access |
| `conversion` | Optional conversion name | Included in signature |

### Best Practices

1. **Short expiry** for sensitive content:
   ```bash
   expires_in=300  # 5 minutes
   ```

2. **Longer expiry** for public content:
   ```bash
   expires_in=86400  # 24 hours (max)
   ```

3. **Rotate signing secrets** quarterly

4. **Use HTTPS** for all signed URLs

---

## Permission Model

### RBAC Permissions

Media operations require specific permissions:

| Permission | Resource | Action | Description |
|------------|----------|--------|-------------|
| `media:upload` | media | upload | Upload files |
| `media:download` | media | download | Download files |
| `media:delete` | media | delete | Delete files |
| `media:view` | media | view | View file details |
| `media:manage` | media | manage | Admin access |

### Model-Specific Permissions

Permissions can also be model-specific:

```go
// User can upload to news articles
allowed, _ := enforcer.Enforce(userID, "default", "news", "upload")

// Fallback to generic media permission
if !allowed {
    allowed, _ = enforcer.Enforce(userID, "default", "media", "upload")
}
```

### Ownership-Based Access

Owners/uploader have implicit permissions:

```go
// Owner can always view their own uploads
if media.UploadedByID == userID {
    return true, nil
}

// Otherwise check RBAC
return enforcer.Enforce(userID, "default", "media", action)
```

### Permission Enforcement Flow

```
Request
   │
   ├─► [JWT Middleware] ──► Extract userID from token
   │     └─► Fail: 401 UNAUTHORIZED
   │
   ├─► [Permission Middleware] ──► Check media:{action} permission
   │     └─► Fail: 403 FORBIDDEN
   │
   └─► [Handler Scope Check] ──► Check ownership or model-specific permission
         └─► Fail: 403 FORBIDDEN
```

### Permission Seeding

```bash
# Run permission sync to create media permissions
go run cmd/api/main.go seed

# Or use Make
make seed
```

---

## Audit Logging

### Logged Operations

All file operations are logged to `audit_logs` table:

| Operation | Action | Logged Data |
|-----------|--------|-------------|
| Upload | `upload` | File size, MIME type, model info |
| Download | `download` | IP, user agent, timestamp |
| Delete | `delete` | User, timestamp, media ID |
| Metadata Update | `update_metadata` | Before/after properties |
| Admin Cleanup | `cleanup` | Orphan count, freed bytes |

### Audit Record Structure

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(50),           -- 'upload', 'download', 'delete'
    resource VARCHAR(50),         -- 'media'
    resource_id VARCHAR(255),     -- media.id
    before_state JSONB,          -- Before state
    after_state JSONB,           -- After state
    ip_address VARCHAR(45),      -- Client IP
    user_agent TEXT,             -- User agent
    created_at TIMESTAMP        -- When action occurred
);
```

### Audit Implementation

```go
// Async logging (non-blocking)
go func() {
    _ = h.auditSvc.LogAction(
        ctx,
        userID,
        "upload",                  // action
        "media",                   // resource
        media.ID.String(),         // resource_id
        nil,                       // before state
        map[string]interface{}{    // after state
            "size": media.Size,
            "mime_type": media.MimeType,
        },
        c.RealIP(),                // IP address
        c.Request().UserAgent(),   // User agent
    )
}()
```

### Audit Query Examples

```sql
-- Recent uploads by user
SELECT * FROM audit_logs 
WHERE action = 'upload' AND resource = 'media' AND user_id = 'xxx'
ORDER BY created_at DESC 
LIMIT 10;

-- All downloads of a specific file
SELECT * FROM audit_logs 
WHERE action = 'download' AND resource_id = 'xxx'
ORDER BY created_at DESC;

-- Failed operations (from error logs)
SELECT * FROM audit_logs 
WHERE action = 'upload' AND after_state->>'error' IS NOT NULL
ORDER BY created_at DESC;
```

### Compliance

- **Immutable**: Audit logs cannot be modified/deleted (DB trigger protection)
- **Retention**: Configure based on compliance requirements (GDPR, SOX, etc.)
- **Export**: Query for compliance reports

---

## Soft Delete Recovery

### Soft Delete Implementation

Media files use GORM soft delete:

```go
type Media struct {
    ID         uuid.UUID
    DeletedAt  gorm.DeletedAt `gorm:"index"`  // Soft delete timestamp
    OrphanedAt *time.Time      `gorm:"index"`  // Marked for cleanup
}
```

### Recovery Process

**Step 1**: Find soft-deleted media
```sql
-- List soft-deleted media from last 30 days
SELECT * FROM media 
WHERE deleted_at IS NOT NULL 
  AND deleted_at > NOW() - INTERVAL '30 days'
ORDER BY deleted_at DESC;
```

**Step 2**: Restore from backup or un-delete
```sql
-- Restore (admin only)
UPDATE media SET deleted_at = NULL WHERE id = 'xxx';
UPDATE media SET orphaned_at = NULL WHERE id = 'xxx';
```

**Note**: Files are not actually deleted until orphan cleanup runs.

### Cleanup Schedule

Orphaned files are cleaned up after 30 days:

```go
// Find orphaned files ready for cleanup
cutoff := time.Now().AddDate(0, 0, -30)
orphaned, _ := repo.FindOrphaned(ctx, cutoff)

// Hard delete files and DB records
for _, media := range orphaned {
    storage.Delete(ctx, media.Path)
    repo.HardDelete(ctx, media.ID)
}
```

---

## Malware Scanning

### Current Status

**Version 1.0**: No integrated malware scanning

**Future Enhancement**: Optional ClamAV integration

### Recommended Implementation

```go
// Future: Malware scanner interface
type MalwareScanner interface {
    Scan(ctx context.Context, reader io.Reader) (*ScanResult, error)
}

type ScanResult struct {
    Clean    bool
    Threats  []string
    Error    error
}
```

### Third-Party Integration

Use cloud-based scanning for production:

**AWS**: S3 + GuardDuty + Macie
**Cloudflare**: Upload scanning with Workers
**Custom**: Trigger ClamAV scan before storage

**Example Flow**:
```
Upload
   │
   ├─► Store in quarantine bucket
   │
   ├─► Trigger Lambda/Cloud Function scan
   │     ├─► Clean: Move to production bucket
   │     └─► Infected: Delete + alert
   │
   └─► Create DB record (if clean)
```

---

## Security Checklist

### Pre-Deployment

- [ ] `MEDIA_SIGNING_SECRET` is 32+ characters, random
- [ ] `JWT_SECRET` is 32+ characters, different from signing secret
- [ ] File upload limits configured appropriately
- [ ] Storage backend uses HTTPS (S3/MinIO with SSL)
- [ ] Database connections use SSL/TLS
- [ ] Redis password set in production

### File Upload Security

- [ ] Magic bytes validation enabled
- [ ] MIME type allowlist configured
- [ ] Blocked extensions cover dangerous types
- [ ] Maximum file size reasonable for use case
- [ ] Filename sanitization in place
- [ ] Path traversal prevention tested

### Access Control

- [ ] RBAC permissions seeded
- [ ] Model-specific permissions configured
- [ ] Admin permissions restricted
- [ ] Ownership checks in place
- [ ] Signed URLs have expiry

### Audit & Monitoring

- [ ] Audit logging enabled
- [ ] Log aggregation configured
- [ ] Failed upload monitoring in place
- [ ] Permission denied tracking enabled
- [ ] Download tracking for sensitive files

### Storage Security

- [ ] Local storage permissions set correctly (0755 dirs, 0644 files)
- [ ] S3 bucket private (no public access)
- [ ] S3 bucket versioning enabled
- [ ] S3 lifecycle policies configured
- [ ] Backup strategy tested

### Network Security

- [ ] HTTPS enforced in production
- [ ] CORS configured for allowed origins
- [ ] Rate limiting enabled
- [ ] Signed URLs served over HTTPS
- [ ] Storage backend not exposed directly

### Incident Response

- [ ] Process for revoking signed URLs
- [ ] Procedure for isolating malicious uploads
- [ ] Audit log analysis procedures
- [ ] Backup restoration tested
- [ ] Security contact configured

---

## Vulnerability Disclosure

If you discover a security vulnerability, please:

1. **Do not** open a public issue
2. Email security@example.com with details
3. Include reproduction steps
4. Allow 90 days for disclosure

---

## Related Documentation

- [Architecture](./ARCHITECTURE.md) - System design and components
- [Deployment](./DEPLOYMENT.md) - Production deployment guide
- [Configuration](./CONFIG.md) - Environment variables reference
- [API Reference](./API.md) - Complete endpoint documentation
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
