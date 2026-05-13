# SSRF Protection Implementation Plan

**Feature Branch**: `020-ssrf-protection`
**Created**: 2026-05-13
**Status**: Ready for implementation

---

## Context

### User Request Summary

Implement a centralized SSRF (Server-Side Request Forgery) protection package for the Go API. Three components currently make outbound HTTP requests:

1. **webhook_worker.go** — has inline SSRF checks in `NewHTTPClient()` DialContext (incomplete: no DNS re-resolution)
2. **webhook.go** — has `validateURL()` and `isPrivateIP()` (used at webhook registration time only)
3. **email_sendgrid_provider.go** — **NO SSRF PROTECTION AT ALL** (security vulnerability)

The goal is to:
- Create a reusable `internal/ssrf/` package with centralized SSRF validation
- Replace all inline SSRF logic with the new package
- Add DNS rebinding prevention (validate resolved IPs at dial time, not just at registration)
- Add IPv6-mapped IPv4 normalization
- Add configurable blocked hosts, allowed CIDRs, allowed schemes
- Apply SSRF-safe transport uniformly to ALL outbound HTTP clients (including SendGrid)
- Preserve testability via `SetHTTPClient()` pattern

### Clarifications from Spec Session

1. **Scope**: Webhook worker + SendGrid provider refactored to use centralized SSRF package. SES/S3 (AWS SDK) out of scope.
2. **DNS caching**: No cache — fresh resolution at every dial time for maximum security.
3. **Redirect policy**: Block at dial time — DialContext validates each redirected connection automatically.
4. **Fixed-target exemptions**: None — all outbound HTTP clients use SSRF-safe transport uniformly.
5. **Metrics**: Structured logs only — metrics counters deferred to OpenTelemetry feature.

### Key Design Decisions

- **Fail-closed**: DNS failure = blocked request. No silent fallbacks.
- **No DNS cache**: Fresh resolution at every connection dial.
- **Dual validation**: URL validation at registration time (early rejection) + dial-time validation in transport (DNS rebinding protection).
- **Uniform enforcement**: No exemptions, even for known endpoints like api.sendgrid.com.
- **Immutable config**: SSRFConfig is constructed once and never mutated. Thread-safe by design.
- **Package location**: `internal/ssrf/` — a standalone package with zero service-layer dependencies.

---

## Hidden Intentions & AI Failure Points

### Hidden Intentions

1. **OAuth readiness (FR-009)**: The "reusable SSRF-safe HTTP client wrapper" is explicitly preparing infrastructure for future OAuth callbacks and inbound webhook processing. This isn't just deduplication — it's security infrastructure.
2. **Uniform security posture**: No exemptions (FR-016) means the team values consistency and auditability over micro-optimization. The package must be trivially easy to adopt for new features.
3. **DNS rebinding is the primary threat model**: The current `validateURL()` at registration time is bypassable by DNS rebinding. Dual validation (FR-011) is the real security fix.
4. **Defense in depth**: The two layers (URL validation at creation + dial-time check) protect against different attack vectors. URL validation catches obvious violations early; dial-time catches runtime rebinding.

### AI Failure Points (What Could Go Wrong)

| # | Failure Mode | Risk | Mitigation |
|---|---|---|---|
| 1 | **IPv6-mapped IPv4 bypass**: `::ffff:127.0.0.1` bypasses `isPrivateIP()` if not normalized first | Critical | Always call `ip.To16()` then check `.To4() != nil` on 16-byte IPs to detect mapped addresses |
| 2 | **DialContext DNS gap**: Current `NewHTTPClient()` only checks already-resolved IPs, no DNS re-resolution | Critical | New DialContext must resolve hostname, check ALL resolved IPs, then dial |
| 3 | **Over-engineering CIDR parsing**: Building complex CIDR matchers when Go has `net.ParseCIDR` | Medium | Use `net.ParseCIDR` + `(*net.IPNet).Contains()` — it's standard library |
| 4 | **Breaking webhook tests**: `SetHTTPClient()` pattern must be preserved | Medium | SSRF-safe client is set in constructors; tests override via `SetHTTPClient()` |
| 5 | **Breaking SendGrid tests**: `SetHTTPClient()` must still work | Medium | Same pattern — constructor sets SSRF client, tests override |
| 6 | **Race condition on config**: Mutable SSRF config read during requests | Low | Config is immutable struct — no setters, no mutation after creation |
| 7 | **Missing `net.JoinHostPort`**: DNS-resolved dial must reassemble host:port | Medium | Use `net.JoinHostPort(resolvedIP, port)` in DialContext |
| 8 | **localhost DNS resolution in tests**: `httptest.NewServer` binds to 127.0.0.1, which is private | High | `SSRF_ALLOW_PRIVATE_IPS=true` in test config, or pass custom allowed CIDRs |

### Ambiguities Resolved

| # | Ambiguity | Resolution |
|---|---|---|
| 1 | Should SendGrid use SSRF-safe transport? | Yes — no exemptions (FR-016). Uniform enforcement. |
| 2 | DNS caching strategy? | No cache — fresh resolution every connection (FR-015, clarified). |
| 3 | Redirect handling? | Block at dial time — DialContext catches redirects automatically (clarified). |
| 4 | Fixed-target exemptions? | None — all outbound HTTP uses SSRF-safe transport (clarified). |
| 5 | Metrics for blocked requests? | Structured logs only — metrics deferred to OpenTelemetry (clarified). |

---

## Task Dependency Graph

| Task | Depends On | Reason |
|------|------------|--------|
| T1: Config & Validator Core | None | Foundation — everything depends on config and IP validation |
| T2: SSRF-Safe Transport | T1 | Transport uses Validator for dial-time checks |
| T3: Unit Tests (Validator + Transport) | T1, T2 | Tests validate core logic |
| T4: Integrate into Webhook Worker | T1, T2 | Worker uses SSRF-safe HTTP client |
| T5: Integrate into Webhook Service | T1 | Service uses URL validator at registration |
| T6: Integrate into SendGrid Provider | T1, T2 | Provider uses SSRF-safe HTTP client |
| T7: Integration Config & Startup Wiring | T4, T5, T6 | Wire SSRF config into app init |
| T8: Refactor — Remove Inline SSRF Code | T4, T5, T6 | Remove old `isPrivateIP` and `validateURL` from webhook files |

---

## Parallel Execution Graph

```
Wave 1 (Start Immediately — No Dependencies):
├── T1: Config & Validator Core (internal/ssrf/config.go, validator.go)

Wave 2 (After T1 completes):
├── T2: SSRF-Safe Transport (internal/ssrf/transport.go)
├── T3: Unit Tests (internal/ssrf/*_test.go) — starts alongside T2
└── T5: Integrate into Webhook Service (internal/service/webhook.go)

Wave 3 (After T2 completes):
├── T4: Integrate into Webhook Worker (internal/service/webhook_worker.go)
└── T6: Integrate into SendGrid Provider (internal/service/email_sendgrid_provider.go)

Wave 4 (After Wave 3 completes):
├── T7: Integration Config & Startup Wiring (internal/config/config.go, cmd/api/main.go)
└── T8: Remove Inline SSRF Code (webhook.go, webhook_worker.go)

Critical Path: T1 → T2 → T4/T6 → T7/T8
Estimated Parallel Speedup: ~40% faster than sequential
```

---

## Tasks

### Task 1: SSRF Config & IP Validator Core

**Description**: Create the `internal/ssrf/` package with SSRF configuration struct, default configuration, and the core IP validation logic (including IPv6-mapped IPv4 normalization, blocked CIDR checking, and blocked hostname checking).

**Files to Create/Modify**:
- `internal/ssrf/config.go` — SSRFConfig struct with mapstructure tags, DefaultSSRFConfig(), BlockedCIDRs (parsed net.IPNet), AllowedCIDRs, BlockedHosts, AllowedSchemes, AllowPrivateIPs
- `internal/ssrf/validator.go` — SSRFValidator struct, NewValidator(cfg), ValidateURL(url), IsBlockedIP(ip), IsBlockedHost(host), IsAllowedScheme(scheme), normalizeIP(ip)

**Maps to FRs**: FR-001, FR-002, FR-003, FR-004, FR-005, FR-006, FR-007, FR-008, FR-012, FR-013, FR-014, FR-015

**Delegation Recommendation**:
- Category: `deep` — Core security logic requires thorough understanding of IPv4/IPv6 edge cases
- Skills: [`clean-code-principles`, `security-review`] — Security-critical code needs clean structure and vulnerability review

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: IP validation logic must be clear, testable, and SOLID — no magic numbers
- ✅ INCLUDED `security-review`: SSRF is a security vulnerability (OWASP Top 10 #6), needs expert review mindset
- ❌ OMITTED `postgresql-database-engineering`: No database involvement
- ❌ OMITTED `test-driven-development`: Will be loaded per-task during T3 (test phase)

**Depends On**: None
**Acceptance Criteria**:
1. `SSRFConfig` struct in `internal/ssrf/config.go` with all fields from FR-001 through FR-008, plus a `Timeout time.Duration` field (defaults to 30s, mapped from `SSRF_TIMEOUT` env var). Includes mapstructure tags and `DefaultSSRFConfig()` function
2. `SSRFValidator` in `internal/ssrf/validator.go` with `NewValidator(cfg *SSRFConfig) *SSRFValidator` constructor
3. `ValidateURL(rawURL string) error` — validates scheme, hostname against blocked hosts, resolves DNS, checks all resolved IPs (FR-011 first-layer validation)
4. `IsBlockedIP(ip net.IP) bool` — checks: IsLoopback, IsPrivate, IsLinkLocalUnicast, IsLinkLocalMulticast, blocked CIDRs, minus allowed CIDRs, IPv6-mapped IPv4 normalization (FR-014)
5. `IsBlockedHost(host string) bool` — exact match against cfg.BlockedHosts (case-insensitive)
6. `IsAllowedScheme(scheme string) bool` — checks against cfg.AllowedSchemes
7. DNS resolution failure returns error (fail-closed, FR-013)
8. `AllowPrivateIPs=true` logs warning via `slog.Warn` on validator creation (FR-007)
9. IPv6-mapped IPv4 addresses (`::ffff:127.0.0.1`) are normalized before checking (FR-014)
10. All blocked request checks log structured details: IP, hostname, reason (FR-012)
11. `go vet ./internal/ssrf/...` passes
12. `golangci-lint run ./internal/ssrf/...` passes with no errors

**QA Scenarios (Agent-Executable)**:
```bash
# Verify package compiles
cd /go-api-feature && go build ./internal/ssrf/...

# Verify no lint errors
cd /go-api-feature && golangci-lint run ./internal/ssrf/...

# Verify DefaultSSRFConfig returns production-secure defaults (no private IPs allowed)
cd /go-api-feature && go test -v -run TestDefaultSSRFConfig ./internal/ssrf/...

# Verify IPv6-mapped IPv4 normalization
cd /go-api-feature && go test -v -run TestNormalizeIPv4MappedIPv6 ./internal/ssrf/...

# Verify blocked CIDRs work
cd /go-api-feature && go test -v -run TestBlockedCIDRs ./internal/ssrf/...

# Verify allowed CIDRs override blocked ranges
cd /go-api-feature && go test -v -run TestAllowedCIDRs ./internal/ssrf/...

# Verify DNS failure blocks request (fail-closed)
cd /go-api-feature && go test -v -run TestDNSFailureBlocks ./internal/ssrf/...
```

---

### Task 2: SSRF-Safe HTTP Transport

**Description**: Create the SSRF-safe HTTP transport that wraps `http.Transport` with a custom `DialContext` that resolves the hostname, validates all resolved IPs against the SSRF validator, and blocks private/internal IPs at connection time. This is the critical runtime protection against DNS rebinding.

**Files to Create/Modify**:
- `internal/ssrf/transport.go` — SSRFSafeTransport struct, NewClient(cfg, logger) *http.Client, custom DialContext implementation
- `internal/ssrf/transport_test.go` — Integration-level tests using httptest.NewServer

**Maps to FRs**: FR-004, FR-009, FR-011, FR-013, FR-015, FR-016

**Delegation Recommendation**:
- Category: `deep` — Custom DialContext with DNS resolution is security-critical and has subtle edge cases
- Skills: [`clean-code-principles`, `security-review`] — Security-critical transport layer

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Transport must be clean, well-documented, and SOLID
- ✅ INCLUDED `security-review`: DNS rebinding prevention is the core security mechanism
- ❌ OMITTED `systematic-debugging`: No bug to debug — building new code

**Depends On**: T1 (Config & Validator Core)
**Acceptance Criteria**:
1. `NewSSRFClient(cfg *SSRFConfig, logger *slog.Logger) *http.Client` returns an `*http.Client` with custom Transport and `Timeout` set from `cfg.Timeout` (defaults to 30s if zero)
2. Custom `DialContext` implementation:
   a. Splits host:port from address
   b. If IP literal: validates directly via `validator.IsBlockedIP()`
   c. If hostname: resolves via `net.LookupIP(host)`, validates ALL resolved IPs
   d. DNS resolution failure → return error (fail-closed, FR-013)
   e. If all IPs pass validation: dials the **first valid IP** via `net.JoinHostPort(ip.String(), port)`
   f. If any IP is blocked: returns error and logs blocked attempt (FR-012)
3. Redirect following works correctly — each redirect triggers a new DialContext validation (FR-004)
4. `AllowPrivateIPs=true` mode: DialContext passes through without IP validation
5. Structured logging of all blocked attempts with: blocked IP, hostname, reason, URL context
6. Thread-safe: no shared mutable state in transport
7. `go build ./internal/ssrf/...` compiles
8. `golangci-lint run ./internal/ssrf/...` passes

**QA Scenarios (Agent-Executable)**:
```bash
# Verify transport blocks loopback
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlocksLoopback ./internal/ssrf/...

# Verify transport blocks RFC1918 private IPs  
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlocksPrivateIP ./internal/ssrf/...

# Verify transport blocks cloud metadata (169.254.169.254)
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlocksCloudMetadata ./internal/ssrf/...

# Verify transport blocks link-local
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlocksLinkLocal ./internal/ssrf/...

# Verify IPv6-mapped IPv4 is normalized and blocked
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlocksIPv6MappedIPv4 ./internal/ssrf/...

# Verify AllowPrivateIPs=true allows loopback
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_AllowPrivateIPsMode ./internal/ssrf/...

# Verify blocked hosts list works
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlockedHosts ./internal/ssrf/...

# Verify allowed CIDRs override blocked ranges
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_AllowedCIDRs ./internal/ssrf/...

# Verify default HTTPS-only scheme blocking
cd /go-api-feature && go test -v -run TestSSRFSafeTransport_BlockHTTP ./internal/ssrf/...

# Verify all existing unit tests still pass
cd /go-api-feature && go test -v ./tests/unit/... 2>&1 | tail -5
```

---

### Task 3: Core Unit Tests

**Description**: Write comprehensive unit tests for the SSRF validator and transport, covering all edge cases: IPv4/IPv6 ranges, IPv6-mapped IPv4, DNS failure, blocked hosts, allowed CIDRs, scheme validation, and development mode.

**Files to Create/Modify**:
- `internal/ssrf/validator_test.go` — Table-driven tests for all IP validation scenarios
- `internal/ssrf/config_test.go` — Tests for DefaultSSRFConfig, config parsing

**Maps to FRs**: All FRs (testable assertions)

**Delegation Recommendation**:
- Category: `deep` — Test design for security logic requires understanding of bypass techniques
- Skills: [`test-driven-development`, `security-review`] — Security tests need adversarial thinking

**Skills Evaluation**:
- ✅ INCLUDED `test-driven-development`: Writing tests first/alongside — proper TDD approach
- ✅ INCLUDED `security-review`: Must think like an attacker — test bypass scenarios
- ❌ OMITTED `clean-code-principles`: Applied in T1/T2, not needed for tests

**Depends On**: T1, T2
**Acceptance Criteria**:
1. Test coverage ≥ 90% for `internal/ssrf/` package (`go test -cover ./internal/ssrf/...`)
2. All test cases from spec acceptance scenarios pass:
   - FR-001: Block 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
   - FR-002: Block 127.0.0.0/8, ::1
   - FR-003: Block 169.254.0.0/16, fe80::/10, 169.254.169.254
   - FR-004: DNS rebinding — validate resolved IPs, not hostnames
   - FR-005: Scheme validation — default HTTPS-only
   - FR-006: Blocked hostnames list
   - FR-007: Development mode — AllowPrivateIPs toggle with warning log
   - FR-008: Allowed CIDRs override blocked ranges
   - FR-013: Fail-closed on DNS failure
   - FR-014: IPv6-mapped IPv4 normalization (::ffff:127.0.0.1)
3. Table-driven test patterns used (Go convention)
4. No external dependencies in tests — all DNS resolution mocked
5. `go test -v -race ./internal/ssrf/...` passes

**QA Scenarios (Agent-Executable)**:
```bash
# Run all SSRF tests with verbose output
cd /go-api-feature && go test -v -race -cover ./internal/ssrf/...

# Verify coverage >= 90%
cd /go-api-feature && go test -coverprofile=coverage.out ./internal/ssrf/... && go tool cover -func=coverage.out | grep total
```

---

### Task 4: Integrate SSRF into Webhook Worker

**Description**: Replace the inline `NewHTTPClient()` and `isPrivateIP()` in `webhook_worker.go` with the centralized SSRF package. The worker should use `ssrf.NewClient()` for its HTTP client.

**Files to Modify**:
- `internal/service/webhook_worker.go` — Remove `NewHTTPClient()` method, remove `isPrivateIP()` from this file (it's already in webhook.go; both will be removed in T8). Add `ssrfCfg *ssrf.SSRFConfig` field, use `ssrf.NewClient(ssrfCfg, logger)` in constructor. Preserve `SetHTTPClient()` for testing.
- `internal/config/webhook.go` — No changes needed (SSRF config is separate)

**Maps to FRs**: FR-009, FR-010, FR-016

**Delegation Recommendation**:
- Category: `quick` — Straightforward refactoring, replacing inline code with package call
- Skills: [`clean-code-principles`] — Ensure clean integration patterns

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Integration should follow existing patterns
- ❌ OMITTED `security-review`: Security logic already in ssrf package, not here
- ❌ OMITTED `test-driven-development`: Existing tests preserved, new logic in ssrf package

**Depends On**: T1, T2
**Acceptance Criteria**:
1. `webhook_worker.go` no longer contains `isPrivateIP` reference in DialContext — the inline SSRF logic in `NewHTTPClient()` is replaced by `ssrf.NewClient(cfg, logger)`
2. `WebhookWorker` struct gains an `httpClient *http.Client` field initialized via `ssrf.NewClient(ssrfCfg, slog.Default())` in the constructor. The `WebhookConfig.DeliveryTimeout` is passed as the client timeout (preserving existing behavior). If `ssrfCfg` is nil, fall back to a basic client without SSRF protection (for backward compatibility during migration).
3. The `NewHTTPClient()` method is removed from `WebhookWorker` — its functionality is replaced by `ssrf.NewClient()` in the constructor.
4. `SetHTTPClient()` method still works for testing — tests can inject a mock client that bypasses SSRF.
5. All existing webhook worker tests pass: `go test -v ./tests/unit/webhook_worker_test.go`
6. `go build ./internal/service/...` compiles

**QA Scenarios (Agent-Executable)**:
```bash
# Verify build compiles
cd /go-api-feature && go build ./internal/service/...

# Verify existing tests pass
cd /go-api-feature && go test -v -run TestWebhookWorker ./tests/unit/...

# Verify no leftover isPrivateIP in webhook_worker.go
grep -c "isPrivateIP" internal/service/webhook_worker.go  # Expected: 0
```

---

### Task 5: Integrate SSRF into Webhook Service

**Description**: Replace the inline `validateURL()` and `isPrivateIP()` functions in `webhook.go` with calls to `ssrf.SSRFValidator.ValidateURL()`. The webhook service should use the SSRF validator at URL creation/update time (first layer of defense).

**Files to Modify**:
- `internal/service/webhook.go` — Replace `validateURL()` and `isPrivateIP()` with `ssrf.SSRFValidator.ValidateURL()`. Add `validator *ssrf.SSRFValidator` field. Update `NewWebhookService()` to accept or create validator.

**Maps to FRs**: FR-010, FR-011

**Delegation Recommendation**:
- Category: `quick` — Replace two functions with package call
- Skills: [`clean-code-principles`] — Clean replacement pattern

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Clean integration following existing dependency injection pattern
- ❌ OMITTED `security-review`: Not writing new security logic, just calling the package

**Depends On**: T1 (SSRF validator must exist)
**Acceptance Criteria**:
1. `webhook.go` no longer contains `validateURL` or `isPrivateIP` functions
2. `WebhookService` has `validator *ssrf.SSRFValidator` field
3. All URL validation calls now use `s.validator.ValidateURL(rawURL)`
4. Error codes remain compatible: validation errors still return 422 status code
5. `AllowHTTP` from `WebhookConfig` maps to `SSRFConfig.AllowedSchemes` (if `AllowHTTP=true`, allowed schemes = ["https", "http"]; if `false`, ["https"] only)
6. All existing webhook service unit tests pass: `go test -v ./tests/unit/... -run TestWebhook`
7. `go build ./internal/service/...` compiles

**QA Scenarios (Agent-Executable)**:
```bash
# Verify build compiles
cd /go-api-feature && go build ./internal/service/...

# Verify existing tests pass
cd /go-api-feature && go test -v -run TestWebhook ./tests/unit/...

# Verify no leftover validateURL/isPrivateIP in webhook.go
grep -c "func validateURL\|func isPrivateIP" internal/service/webhook.go  # Expected: 0
```

---

### Task 6: Integrate SSRF into SendGrid Provider

**Description**: Replace the bare `http.Client` in `NewSendGridProvider` with an SSRF-safe client from the `ssrf` package. This closes the known SSRF vulnerability in the SendGrid email provider.

**Files to Modify**:
- `internal/service/email_sendgrid_provider.go` — Replace `&http.Client{Timeout: 15 * time.Second}` with `ssrf.NewClient(ssrfCfg, logger)`. Add `ssrfCfg` to constructor or accept `*http.Client` parameter. Preserve `SetHTTPClient()` for testing.
- `internal/config/email_sendgrid.go` or `internal/config/email.go` — Add SSRF config reference if needed (or pass from main.go)

**Maps to FRs**: FR-010, FR-016

**Delegation Recommendation**:
- Category: `quick` — Minimal change, add SSRF client to existing provider
- Skills: [`clean-code-principles`] — Follow existing patterns

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Follow existing constructor/DI pattern
- ❌ OMITTED `security-review`: Security logic already in ssrf package

**Depends On**: T1, T2
**Acceptance Criteria**:
1. `NewSendGridProvider` creates an SSRF-safe HTTP client via `ssrf.NewClient(cfg, logger)`. The client timeout is set by `SSRFConfig.Timeout` (the existing hardcoded 15s becomes the SSRF config's default timeout for SendGrid, configurable via `SSRF_TIMEOUT` env var).
2. `SetHTTPClient()` still works for testing
3. SendGrid tests passing: `go test -v ./tests/unit/email_sendgrid_provider_test.go`
4. Default SSRF configuration blocks private IPs (SendGrid's api.sendgrid.com resolves to public IPs — will work)
5. `go build ./internal/service/...` compiles

**QA Scenarios (Agent-Executable)**:
```bash
# Verify build compiles
cd /go-api-feature && go build ./internal/service/...

# Verify existing SendGrid tests pass
cd /go-api-feature && go test -v -run TestSendGridProvider ./tests/unit/...

# Verify NewSendGridProvider uses SSRF client (no bare http.Client)
grep -c "http.Client{Timeout" internal/service/email_sendgrid_provider.go  # Expected: 0
```

---

### Task 7: Integration Config & Startup Wiring

**Description**: Add SSRF configuration to the main config struct, parse it from environment variables, wire it into the service constructors in `cmd/api/main.go`, and add startup validation/warnings.

**Files to Modify**:
- `internal/config/ssrf.go` — NEW or INLINE: Add SSRFConfig parsing, `DefaultSSRFConfig()`, `parseSSRFConfig()`
- `internal/config/config.go` — Add `SSRF SSRFConfig` field to Config struct, add `parseSSRFConfig(v, cfg)` call
- `internal/config/validate.go` — Add SSRF config validation (warn on AllowPrivateIPs, validate CIDRs)
- `cmd/api/main.go` — Wire SSRF config into webhook worker, webhook service, and SendGrid provider constructors

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `SSRF_ALLOW_PRIVATE_IPS` | false | Allow private IPs (dev mode) |
| `SSRF_ALLOWED_SCHEMES` | https | Comma-separated allowed schemes |
| `SSRF_BLOCKED_HOSTS` | metadata.internal,metadata.google.internal,instance-data.ec2.internal | Comma-separated blocked hostnames |
| `SSRF_ALLOWED_CIDRS` | (empty) | Comma-separated CIDRs to whitelist within blocked ranges |

**Maps to FRs**: FR-006, FR-007, FR-008

**Delegation Recommendation**:
- Category: `quick` — Follows established config pattern exactly
- Skills: [`clean-code-principles`] — Follow existing config patterns

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Must follow existing config patterns exactly (mapstructure, parse, defaults, validate)
- ❌ OMITTED `security-review`: No security logic here, just configuration

**Depends On**: T4, T5, T6 (all integrations must exist first)
**Acceptance Criteria**:
1. `internal/config/ssrf.go` created with `SSRFConfig` struct, `DefaultSSRFConfig()`, `parseSSRFConfig()`
2. `Config` struct in config.go has `SSRF SSRFConfig` field
3. `setDefaults()` in config.go sets SSRF defaults
4. `loadConfig()` calls `parseSSRFConfig(v, cfg)`
5. `validate()` warns when `AllowPrivateIPs=true` via `slog.Warn`
6. `cmd/api/main.go` creates `ssrfValidator` from config and passes to:
   - `NewWebhookService()` (or its constructor)
   - `NewWebhookWorker()` (directly or via config)
   - `NewSendGridProvider()` (via config or direct parameter)
7. All existing tests pass: `go test ./... 2>&1 | tail -20`
8. `go build ./cmd/api/...` compiles
9. `TestSSRFAllowPrivateIPsWarning` unit test exists in `internal/config/` verifying that `SSRF_ALLOW_PRIVATE_IPS=true` produces `slog.Warn` output

**QA Scenarios (Agent-Executable)**:
```bash
# Verify build compiles
cd /go-api-feature && go build ./cmd/api/...

# Verify SSRF config can be parsed
cd /go-api-feature && SSRF_ALLOW_PRIVATE_IPS=true SSRF_BLOCKED_HOSTS=metadata.internal go build ./cmd/api/...

# Verify all existing tests pass
cd /go-api-feature && go test ./tests/unit/... -count=1 2>&1 | tail -5

# Verify SSRF_ALLOW_PRIVATE_IPS=true produces warning in logs
cd /go-api-feature && go test -v -run TestSSRFAllowPrivateIPsWarning ./internal/config/...
```

---

### Task 8: Remove Inline SSRF Code & Final Cleanup

**Description**: Remove the old inline `isPrivateIP()` and `validateURL()` functions from `webhook.go` and `webhook_worker.go` that have been replaced by the SSRF package. Update any remaining references. Run full test suite to confirm zero regressions.

**Files to Modify**:
- `internal/service/webhook.go` — Remove `isPrivateIP()` and `validateURL()` functions (if not already removed in T5)
- `internal/service/webhook_worker.go` — Remove `isPrivateIP()` and `NewHTTPClient()` (if not already removed in T4)
- `internal/service/webhook.go` — Ensure all imports are clean
- `internal/service/webhook_worker.go` — Ensure all imports are clean

**Maps to FRs**: FR-010 (complete removal of inline SSRF logic)

**Delegation Recommendation**:
- Category: `quick` — Pure cleanup, deleting replaced code
- Skills: [`clean-code-principles`] — Ensure no dangling references

**Skills Evaluation**:
- ✅ INCLUDED `clean-code-principles`: Clean removal, no orphaned references
- ❌ OMITTED `security-review`: No new security logic, just cleanup

**Depends On**: T4, T5, T6
**Acceptance Criteria**:
1. No `isPrivateIP` function exists in `webhook.go` or `webhook_worker.go`
2. No `validateURL` function exists in `webhook.go`
3. No `NewHTTPClient` method on `WebhookWorker` that contains SSRF logic (may still exist as wrapper that calls `ssrf.NewClient()`)
4. All imports in modified files are clean (`goimports`-formatted)
5. Full test suite passes: `go test ./... 2>&1 | tail -5`
6. `golangci-lint run ./...` passes
7. No references to old inline SSRF functions anywhere: `grep -r "func isPrivateIP\|func validateURL" internal/`

**QA Scenarios (Agent-Executable)**:
```bash
# Verify no inline SSRF functions remain
grep -rn "func isPrivateIP\|func validateURL" internal/service/webhook.go internal/service/webhook_worker.go  # Expected: empty

# Verify full build compiles
cd /go-api-feature && go build ./...

# Verify all unit tests pass
cd /go-api-feature && go test -v -race -count=1 ./tests/unit/... 2>&1 | tail -20

# Verify lint passes
cd /go-api-feature && golangci-lint run ./...

# Verify goimports on modified files
cd /go-api-feature && goimports -l internal/service/webhook.go internal/service/webhook_worker.go internal/service/email_sendgrid_provider.go  # Expected: empty output
```

---

## Commit Strategy

### Atomic Commits by Task

| Commit | Files | Message |
|--------|-------|---------|
| 1 | `internal/ssrf/config.go`, `internal/ssrf/validator.go` | `feat(ssrf): add SSRF config and IP validator core` |
| 2 | `internal/ssrf/transport.go` | `feat(ssrf): add SSRF-safe HTTP transport with DNS rebinding prevention` |
| 3 | `internal/ssrf/validator_test.go`, `internal/ssrf/config_test.go`, `internal/ssrf/transport_test.go` | `test(ssrf): comprehensive unit tests for validator and transport` |
| 4 | `internal/service/webhook_worker.go` | `refactor(webhook): replace inline SSRF with centralized ssrf package` |
| 5 | `internal/service/webhook.go` | `refactor(webhook): replace validateURL/isPrivateIP with ssrf.Validator` |
| 6 | `internal/service/email_sendgrid_provider.go` | `fix(email): add SSRF protection to SendGrid HTTP client` |
| 7 | `internal/config/ssrf.go`, `internal/config/config.go`, `internal/config/validate.go`, `cmd/api/main.go` | `feat(config): add SSRF configuration and startup wiring` |
| 8 | `internal/service/webhook.go`, `internal/service/webhook_worker.go` | `chore(ssrf): remove inline SSRF functions, cleanup imports` |

### Branch Strategy
- Work on branch `020-ssrf-protection`
- Each commit should leave the codebase in a compilable, test-passing state
- Squash before merge if desired, but atomic commits during development

---

## Identified Risks & Mitigations

| # | Risk | Severity | Mitigation |
|---|------|----------|------------|
| 1 | **IPv6-mapped IPv4 bypass** | Critical | Always normalize IPs using `ip.To16()` then check `To4()` != nil for 16-byte representations. Test with `::ffff:127.0.0.1`, `::ffff:10.0.0.1`, etc. |
| 2 | **DNS rebinding not caught** | Critical | DialContext MUST resolve hostname, validate ALL resolved IPs, then dial to validated IP. Test with mock DNS that returns different IPs. |
| 3 | **Breaking existing webhook/SendGrid tests** | High | Preserve `SetHTTPClient()` pattern in both WebhookWorker and SendGridProvider. Tests inject mock clients. |
| 4 | **httptest.NewServer uses 127.0.0.1** | High | Existing tests that use `httptest.NewServer` bind to loopback. In test config, set `AllowPrivateIPs=true` or pass `AllowedCIDRs: ["127.0.0.0/8"]` for test-only clients. |
| 5 | **Performance regression on DNS resolution** | Medium | Fresh DNS resolution on every connection is by design (FR-015). Accept the latency trade-off for security. Monitor in production. |
| 6 | **SendGrid connectivity** | Medium | api.sendgrid.com resolves to public IPs — SSRF client won't block it. Verify with integration test. |
| 7 | **Config hot-reload race** | Low | Config is immutable struct — no mutation after creation. New config = new validator + new client. |
| 8 | **Over-blocking in development** | Low | `SSRF_ALLOW_PRIVATE_IPS=true` with `slog.Warn` on startup. Developers can easily enable. |

---

## Success Criteria Verification

| SC ID | Criterion | Verification Method |
|-------|-----------|---------------------|
| SC-001 | All private/IP requests blocked 100% | `go test -v -run "TestSSRFSafeTransport_Blocks" ./internal/ssrf/...` — all block tests pass |
| SC-002 | Legitimate HTTP(S) to public URLs works | `go test -v -run "TestSSRFSafeTransport_AllowsPublic" ./internal/ssrf/...` — allow tests pass |
| SC-003 | Dev mode with single toggle | `SSRF_ALLOW_PRIVATE_IPS=true` in config → warning logged + private IPs allowed |
| SC-004 | Reusable for new features | `ssrf.NewClient(cfg, logger)` returns ready-to-use `*http.Client`; import + call |
| SC-005 | Structured logs for blocked requests | Test assertions on `slog` output containing IP, hostname, reason |
| SC-006 | <5ms latency per delivery | Benchmark test: `go test -bench=BenchmarkSSRFValidation ./internal/ssrf/...` |

---

## Spec Gaps & Concerns

1. **SES Provider out of scope**: The SES provider uses AWS SDK which has its own HTTP client. The spec explicitly excludes it, but future consideration needed.
2. **No migration needed**: This is correct — no DB changes required.
3. **Metrics deferred**: The spec explicitly defers metrics to OpenTelemetry feature. Structured logs only for now.
4. **No redirect policy configuration**: The spec says "block at dial time" — redirects are caught by DialContext validation on each connection. This is sufficient but could be enhanced with redirect count limits in the future.
5. **Test infrastructure gap**: Existing tests use `httptest.NewServer` which binds to 127.0.0.1. The integration approach (test config with AllowPrivateIPs) needs to be well-documented.
