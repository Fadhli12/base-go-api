# Tasks: SSRF Protection

**Input**: Design documents from `/specs/020-ssrf-protection/`
**Prerequisites**: plan.md (required), spec.md (required)
**Tests**: Included per plan AC requirements (T003, T008, T013, T018)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the `internal/ssrf/` package foundation — config struct and IP validator core that ALL subsequent tasks depend on.

- [ ] T001 [US1] Create SSRFConfig struct with all FR-001 through FR-008 fields, Timeout field (default 30s), mapstructure tags, and DefaultSSRFConfig() function in `internal/ssrf/config.go`
- [ ] T002 [US1] Create SSRFValidator struct with NewValidator(cfg), ValidateURL(url), IsBlockedIP(ip), IsBlockedHost(host), IsAllowedScheme(scheme), normalizeIP(ip) in `internal/ssrf/validator.go`

**Checkpoint**: Package compiles, `go build ./internal/ssrf/...` succeeds

---

## Phase 2: Foundational (DNS Rebinding Defense + Transport)

**Purpose**: Build the SSRF-safe HTTP transport with DialContext DNS resolution — the critical runtime protection. This MUST be complete before any integration task (T4-T6).

- [ ] T003 [US1] [US2] Create SSRFSafeTransport in `internal/ssrf/transport.go` — NewClient(cfg, logger) *http.Client with custom DialContext that resolves hostnames, validates ALL resolved IPs, and blocks private/internal IPs at connection time. Client.Timeout set from cfg.Timeout (default 30s). AllowPrivateIPs=true mode passes through. Structured logging of all blocked attempts.
- [ ] T004 [P] [US1] Write unit tests for SSRFConfig and DefaultSSRFConfig in `internal/ssrf/config_test.go` — test default values, BlockedCIDRs parsing, AllowedSchemes defaults
- [ ] T005 [P] [US1] Write table-driven unit tests for SSRFValidator in `internal/ssrf/validator_test.go` — cover FR-001 through FR-008, FR-013, FR-014 (IPv6-mapped IPv4 normalization)
- [ ] T006 [P] [US2] Write integration-level tests for SSRFSafeTransport in `internal/ssrf/transport_test.go` — test DialContext blocks loopback, private IPs, cloud metadata, link-local, IPv6-mapped IPv4; test AllowPrivateIPs mode; test blocked hosts; test allowed CIDRs; test HTTPS-only blocking; test redirect validation

**Checkpoint**: All SSRF tests pass: `go test -v -race -cover ./internal/ssrf/...` with ≥90% coverage

---

## Phase 3: User Story 1 + 2 — Block Internal Network Access + DNS Rebinding Prevention (Priority: P1)

**Goal**: Replace inline SSRF logic with centralized package in webhook worker and webhook service. This delivers the core security fix (blocking internal network access + DNS rebinding prevention).

**Independent Test**: Create a webhook pointing to `http://127.0.0.1:6379` (Redis) or `http://169.254.169.254/latest/meta-data/` (AWS metadata) and verify the request is blocked.

- [ ] T007 [US1] Integrate SSRF into webhook worker — replace `NewHTTPClient()` method and inline `isPrivateIP` check with `ssrf.NewClient(ssrfCfg, slog.Default())`. Add `httpClient` field initialized in constructor with `WebhookConfig.DeliveryTimeout` as client timeout. If ssrfCfg is nil, fall back to basic http.Client. Preserve `SetHTTPClient()` for testing. File: `internal/service/webhook_worker.go`
- [ ] T008 [US2] Integrate SSRF into webhook service — replace `validateURL()` and `isPrivateIP()` functions with `ssrf.SSRFValidator.ValidateURL()`. Add `validator *ssrf.SSRFValidator` field to WebhookService. Map `WebhookConfig.AllowHTTP` to `SSRFConfig.AllowedSchemes` (if AllowHTTP=true → ["https","http"], if false → ["https"] only). Preserve error codes (422). File: `internal/service/webhook.go`

**Checkpoint**: Webhook worker blocks internal IPs at dial time. Webhook service validates URLs at creation time. Existing tests still pass.

---

## Phase 4: User Story 5 — Reusable SSRF-Safe HTTP Client (Priority: P2)

**Goal**: Extend SSRF protection to SendGrid provider — the package is now reusable across all outbound HTTP clients.

**Independent Test**: Verify SendGrid HTTP client uses SSRF-safe transport (no bare `http.Client{Timeout: 15 * time.Second}`).

- [ ] T009 [US5] Integrate SSRF into SendGrid provider — replace bare `http.Client{Timeout: 15 * time.Second}` with `ssrf.NewClient(cfg, logger)`. Add ssrfCfg to constructor. Preserve `SetHTTPClient()` for testing. The existing 15s timeout becomes configurable via `SSRFConfig.Timeout`. File: `internal/service/email_sendgrid_provider.go`

**Checkpoint**: All outbound HTTP clients (webhook worker, webhook service, SendGrid) now use centralized SSRF package.

---

## Phase 5: User Story 3 + 4 — Development Mode + Configurable Blocked Hosts & Schemes (Priority: P2)

**Goal**: Wire SSRF configuration from environment variables into all services. This completes the development mode toggle and configurable blocked hosts/schemes.

**Independent Test**: Set `SSRF_ALLOW_PRIVATE_IPS=true` and verify requests to localhost succeed with a warning log. Set `SSRF_BLOCKED_HOSTS=metadata.internal` and verify that hostname is blocked.

- [ ] T010 [US3] [US4] Create `internal/config/ssrf.go` — SSRFConfig struct with mapstructure tags, DefaultSSRFConfig(), parseSSRFConfig(v, cfg). Environment variables: SSRF_ALLOW_PRIVATE_IPS (default: false), SSRF_ALLOWED_SCHEMES (default: "https"), SSRF_BLOCKED_HOSTS (default: "metadata.internal,metadata.google.internal,instance-data.ec2.internal"), SSRF_ALLOWED_CIDRS (default: ""), SSRF_TIMEOUT (default: "30s")
- [ ] T011 [US3] [US4] Add `SSRF SSRFConfig` field to Config struct in `internal/config/config.go`, add parseSSRFConfig(v, cfg) call in loadConfig(), add SSRF defaults in setDefaults()
- [ ] T012 [US3] [US4] Add SSRF config validation in `internal/config/validate.go` — warn when AllowPrivateIPs=true via slog.Warn, validate CIDR formats
- [ ] T013 [US3] Write TestSSRFAllowPrivateIPsWarning unit test in `internal/config/ssrf_test.go` — verify SSRF_ALLOW_PRIVATE_IPS=true produces slog.Warn output
- [ ] T014 [US3] [US4] Wire SSRF config into service constructors in `cmd/api/main.go` — create ssrfValidator from config, pass to NewWebhookService(), NewWebhookWorker() (via config or direct), NewSendGridProvider() (via config or direct parameter). Graceful shutdown order unchanged.

**Checkpoint**: All SSRF env vars parsed. Warning logged when AllowPrivateIPs=true. Services receive SSRF config from main.go.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Remove inline SSRF code, run full test suite, verify lint, ensure zero regressions.

- [ ] T015 Remove `isPrivateIP()` and `validateURL()` functions from `internal/service/webhook.go` (if not already removed in T008). Remove `NewHTTPClient()` and `isPrivateIP()` from `internal/service/webhook_worker.go` (if not already removed in T007). Clean up imports. Verify: `grep -r "func isPrivateIP\|func validateURL" internal/` returns empty.
- [ ] T016 Run full test suite: `go test -v -race -count=1 ./tests/unit/...` — verify zero regressions
- [ ] T017 Run linter: `golangci-lint run ./...` — verify zero errors
- [ ] T018 Run SSRF-specific tests with coverage: `go test -v -race -coverprofile=coverage.out ./internal/ssrf/... && go tool cover -func=coverage.out | grep total` — verify ≥90% coverage

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 (T001, T002) — BLOCKS all integration
- **Phase 3 (US1+US2)**: Depends on T003 (transport) for T007, depends on T001+T002 for T008
- **Phase 4 (US5)**: Depends on T003 (transport) for SendGrid integration
- **Phase 5 (US3+US4)**: Depends on T007, T008, T009 (all integrations exist)
- **Phase 6 (Polish)**: Depends on all previous phases

### Critical Path

```
T001 → T002 → T003 → T007 → T014 → T015 → T016/T017/T018
                              T008 ──→ T014
                 T004 ─┐
                 T005 ─┤ (parallel with T007/T008 after T003)
                 T006 ─┘
                              T009 ──→ T014
```

### Parallel Opportunities

- **Phase 1**: T001 and T002 can be developed in parallel (different files)
- **Phase 2**: T004, T005, T006 can all run in parallel after T003
- **Phase 3**: T007 and T008 can run in parallel (different files, T007 needs T003, T008 needs T001+T002)
- **Phase 4**: T009 can run in parallel with T007/T008 (different files)
- **Phase 5**: T010, T011, T012 can run in parallel (different files), T013 after T010, T014 after T010-T012

### Within-Phase Ordering

- Phase 1: T001 before T002 (validator depends on config struct)
- Phase 2: T003 first (transport), then T004/T005/T006 in parallel
- Phase 3: T007 and T008 in parallel
- Phase 5: T010, T011, T012 first; then T013, T014

---

## Implementation Strategy

### MVP First (User Stories 1+2 Only)

1. Complete Phase 1: Setup (T001, T002)
2. Complete Phase 2: Foundational (T003, T004, T005, T006)
3. Complete Phase 3: Webhook integration (T007, T008)
4. **STOP and VALIDATE**: Test webhook SSRF protection independently
5. Deploy if ready — core security vulnerability is patched

### Incremental Delivery

1. Phases 1-3 → Core SSRF protection for webhooks (MVP)
2. Phase 4 → SendGrid SSRF protection
3. Phase 5 → Configurable blocked hosts/schemes, development mode
4. Phase 6 → Cleanup, full test suite, lint

---

## Notes

- T001 and T002 are in the same package (`internal/ssrf/`) — if parallelizing, coordinate import dependencies
- T007 removes `NewHTTPClient()` and `isPrivateIP()` from webhook_worker.go as part of integration
- T008 removes `validateURL()` and `isPrivateIP()` from webhook.go as part of integration
- T015 is cleanup — may be a no-op if T007/T008 already removed the inline functions
- Test infrastructure: `httptest.NewServer` binds to 127.0.0.1 — use `AllowPrivateIPs=true` or `AllowedCIDRs: ["127.0.0.0/8"]` in test configs
- `SSRFConfig.Timeout` defaults to 30s; SendGrid previously used 15s hardcoded — now configurable
- No database migrations needed for this feature
- Structured logging only (no metrics) — metrics deferred to OpenTelemetry feature