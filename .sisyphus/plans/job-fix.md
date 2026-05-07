# Fix Job Handler Compilation Error

## TL;DR

> **Quick Summary**: Move `internal/http/handler/request/job.go` to `internal/http/request/job.go` to fix `SubmitJobRequest` not found error.
>
> **Deliverable**: File moved from wrong location to correct location
> **Estimated Effort**: Trivial (1 file move, no code changes)
> **Root Cause**: Request DTO placed in wrong directory

---

## Context

### Problem
`go build ./cmd/api/` fails:
```
internal/http/handler/job.go:62:18: undefined: request.SubmitJobRequest
```

### Root Cause
`SubmitJobRequest` lives at `internal/http/handler/request/job.go` but the handler imports `internal/http/request`. In Go, directories are separate packages even if package name is the same.

### File Locations
```
WRONG (current): internal/http/handler/request/job.go   ← SubmitJobRequest IS here
CORRECT (should be): internal/http/request/job.go        ← Should be here
```

### Fix
Move the file from `handler/request/` to `request/`. No code changes needed — package name is already `request` and imports are already correct.

---

## TODOs

- [x] 1. Move job.go from handler/request to request directory

### Blockers (pre-existing, NOT in scope):
- `server.go:984` - `s.config.Job` undefined (Config missing Job field)
- `server.go:984,987` - `s.logger` type mismatch (Logger interface vs *slog.Logger)

These are separate issues requiring their own plans.

  **What to do**:
  - Move `internal/http/handler/request/job.go` to `internal/http/request/job.go`
  - Use git mv to preserve history

  **Verification**: `go build ./cmd/api/` succeeds

  **Files**:
  - FROM: `internal/http/handler/request/job.go`
  - TO: `internal/http/request/job.go`

  **QA**:
  ```
  Scenario: cmd/api builds successfully
    Tool: Bash
    Steps:
      1. go build ./cmd/api/
    Expected Result: No output (success)
    ```

- [ ] F1. Build verification

  **Commands**:
  - `go build ./cmd/api/`
  - `go build ./internal/http/handler/`
  - `go vet ./internal/http/handler/...`

---

## Success Criteria

```bash
go build ./cmd/api/              # Expected: no output (success)
go build ./internal/http/handler/ # Expected: no output (success)
go vet ./internal/http/handler/... # Expected: no output (success)
```