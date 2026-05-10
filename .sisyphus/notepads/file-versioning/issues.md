# File Versioning Implementation - Issues & Gotchas

## 2026-05-08

### G1: Permission Manifest
- The `DefaultPermissions()` function in `cmd/api/main.go` is the source of truth for permissions
- No `config/permissions.yaml` file currently exists (the code reads it but falls back to defaults)
- Need to add 5 new `media_version:*` permissions to `DefaultPermissions()`

### G2: Media.ToResponse() Extension
- Existing `ToResponse()` doesn't include `CurrentVersion` or `Versions`
- Must add these fields ADDITIVELY to maintain backward compatibility
- `VersionCount` and `Versions` should only be populated when preloaded

### G3: Storage Driver Interface
- Existing `storage.Driver` interface has `Store(ctx, path, reader)` and `Delete(ctx, path)`
- Need to verify it also has `Get(ctx, path)` for download streaming — check before implementing
- Versioned paths will use the same driver, just different path format

### G4: Audit Log Signature
- Audit log uses 7-param `LogAction(ctx, actorID, action, resource, resourceID, before, after)`
- Version operations should log with actions like `version_upload`, `version_download`, `version_restore`, `version_delete`
- Resource should be `media_version`, resourceID should be the version UUID

### G5: Test Infrastructure
- Integration tests use `testcontainers-go` with ephemeral Postgres + Redis
- Test suite at `tests/integration/testsuite.go` — follow this pattern
- Unit tests use `testify/mock` — repos/services need mock interfaces