# Feature Specification: SSRF Protection

**Feature Branch**: `020-ssrf-protection`
**Created**: 2026-05-13
**Status**: Draft
**Input**: User description: "SSRF Protection (020-ssrf-protection) — Webhook worker makes outbound HTTP requests to user-provided URLs. Without URL validation, attackers can force the server to access internal services (Redis, database, cloud metadata endpoints like 169.254.169.254). This is OWASP API Security Top 10 #6."

## User Scenarios & Testing

### User Story 1 - Block Internal Network Access (Priority: P1)

As a platform operator, I need the system to block all outbound HTTP requests from webhook workers that target internal/private network addresses, so that attackers cannot exploit webhook URLs to scan or access internal services like databases, Redis, or cloud metadata endpoints.

**Why this priority**: This is the core security requirement. Without it, the existing webhook system is vulnerable to SSRF attacks that could expose infrastructure details, access cloud metadata credentials, or pivot to internal services.

**Independent Test**: Can be fully tested by creating a webhook pointing to an internal IP (e.g., `http://127.0.0.1:6379` for Redis, `http://169.254.169.254/latest/meta-data/` for AWS metadata) and verifying the request is blocked before any connection is made. Delivers immediate security value.

**Acceptance Scenarios**:

1. **Given** a webhook configured with URL `http://127.0.0.1/admin`, **When** the webhook worker attempts delivery, **Then** the request is blocked with an SSRF error and no connection is made to the loopback address
2. **Given** a webhook configured with URL `http://10.0.0.1:5432/db`, **When** the webhook worker attempts delivery, **Then** the request is blocked with an SSRF error (RFC 1918 private range)
3. **Given** a webhook configured with URL `http://169.254.169.254/latest/meta-data/`, **When** the webhook worker attempts delivery, **Then** the request is blocked with an SSRF error (cloud metadata endpoint)
4. **Given** a webhook configured with URL `http://192.168.1.1/internal`, **When** the webhook worker attempts delivery, **Then** the request is blocked (RFC 1918 private range)
5. **Given** a webhook configured with URL `http://172.20.0.1/health`, **When** the webhook worker attempts delivery, **Then** the request is blocked (172.16.0.0/12 range)

---

### User Story 2 - DNS Rebinding Prevention (Priority: P1)

As a security-conscious operator, I need the system to validate the resolved IP address of a hostname (not just the hostname string), so that attackers cannot bypass IP-based filtering using DNS rebinding attacks (where a domain resolves to a private IP after the initial check passes).

**Why this priority**: Without DNS rebinding protection, the IP blocklist can be trivially bypassed. An attacker points a domain at a public IP initially, then changes DNS to resolve to a private IP. This is a well-known SSRF bypass technique.

**Independent Test**: Can be tested by configuring a domain that resolves to both a public and private IP (or simulating DNS rebinding) and verifying the system checks the resolved IP at connection time, not just at validation time.

**Acceptance Scenarios**:

1. **Given** a webhook URL using a hostname that resolves to both a public and private IP, **When** the system validates the URL, **Then** all resolved IPs are checked and the request is blocked if any resolve to a private/internal range
2. **Given** a webhook URL whose DNS records change between validation and connection, **When** the HTTP client dials the address, **Then** the dial-time check validates the actual connected IP and blocks if it falls in a blocked range
3. **Given** a hostname that resolves to multiple IPs (some public, some private), **When** the system processes the webhook, **Then** the request is blocked because at least one resolution target is private

---

### User Story 3 - Development Mode with Private IP Allowlist (Priority: P2)

As a developer running the API locally, I need a configurable option to allow private IP requests during development, so that I can test webhook deliveries to local services (e.g., `http://localhost:3000/hooks`) without deploying to a production-like environment.

**Why this priority**: Enables developer productivity without compromising production security. The default (block private IPs) is secure-by-default; the override is opt-in for local development.

**Independent Test**: Can be tested by setting `SSRF_ALLOW_PRIVATE_IPS=true` in a development environment and verifying that requests to `http://127.0.0.1:3000/webhook` succeed, then setting it to `false` (default) and verifying the same request is blocked.

**Acceptance Scenarios**:

1. **Given** `SSRF_ALLOW_PRIVATE_IPS=true` in configuration, **When** a webhook delivery targets `http://localhost:3000/hooks`, **Then** the request is allowed to proceed
2. **Given** `SSRF_ALLOW_PRIVATE_IPS=false` (default), **When** a webhook delivery targets `http://localhost:3000/hooks`, **Then** the request is blocked
3. **Given** a specific allowed CIDR like `172.16.0.0/12` configured in `SSRF_ALLOWED_CIDRS`, **When** a webhook delivery targets an IP in that range, **Then** the request is allowed while other private ranges remain blocked
4. **Given** production configuration (default: no private IPs allowed), **When** an operator reviews the configuration, **Then** all private/internal ranges are blocked by default without any explicit configuration

---

### User Story 4 - Configurable Blocked Hosts and Schemes (Priority: P2)

As a platform operator, I need to configure blocked hostnames (e.g., `metadata.internal`, `instance-data.ec2.internal`) and allowed URL schemes (e.g., `https` only by default), so that SSRF protection adapts to my cloud environment and security posture.

**Why this priority**: Cloud-specific metadata endpoints differ across providers (AWS, GCP, Azure). Configurable blocked hosts and schemes let operators tailor protection without code changes.

**Independent Test**: Can be tested by adding `metadata.internal` to the blocked hosts list, attempting a request to that hostname, and verifying it's blocked even if it resolves to a public IP. Similarly, testing that `http://` requests are blocked by default but `https://` requests to public IPs are allowed.

**Acceptance Scenarios**:

1. **Given** `SSRF_BLOCKED_HOSTS` includes `metadata.internal`, **When** a webhook URL is `https://metadata.internal/credentials`, **Then** the request is blocked regardless of IP resolution
2. **Given** default configuration with `SSRF_ALLOWED_SCHEMES=["https"]`, **When** a webhook URL uses `http://` scheme, **Then** the request is blocked
3. **Given** `SSRF_ALLOWED_SCHEMES=["https", "http"]`, **When** a webhook URL uses `http://` scheme to a public IP, **Then** the request is allowed
4. **Given** `SSRF_BLOCKED_HOSTS` includes `instance-data.ec2.internal`, **When** a webhook URL is `https://instance-data.ec2.internal/latest/meta-data/`, **Then** the request is blocked before DNS resolution

---

### User Story 5 - Reusable SSRF-Safe HTTP Client (Priority: P2)

As a developer adding new features that make outbound HTTP requests (e.g., OAuth callbacks, inbound webhook processing), I need a reusable SSRF-safe HTTP client package, so that I don't have to duplicate SSRF validation logic across every feature.

**Why this priority**: The codebase currently has SSRF logic embedded in the webhook worker. Extracting it into a reusable package prevents duplication and ensures all outbound HTTP calls are protected consistently.

**Independent Test**: Can be tested by importing the SSRF protection package in a new service, constructing an HTTP client with it, and verifying that all the same SSRF protections apply.

**Acceptance Scenarios**:

1. **Given** the SSRF protection package is imported in a new service, **When** that service creates an HTTP client via the package, **Then** all outbound requests through that client receive the same SSRF protections as the webhook worker
2. **Given** multiple services create SSRF-safe HTTP clients, **When** configuration for blocked CIDRs is updated, **Then** all clients respect the updated configuration on next client creation
3. **Given** a developer wants to add an OAuth callback handler, **When** they use the SSRF-safe client wrapper, **Then** the callback URL is validated against the same SSRF rules before any outbound request is made

---

### Edge Cases

- What happens when a hostname resolves to both IPv4 and IPv6 addresses, one of which is private? The request is blocked if any resolved IP is in a blocked range.
- What happens when DNS resolution fails entirely? The request is blocked (fail-closed security posture).
- What happens when the SSRF validator encounters an IPv6-mapped IPv4 address (e.g., `::ffff:127.0.0.1`)? It is normalized and checked against the same private/internal ranges.
- What happens when a URL contains a redirect to a private IP? The transport's DialContext check validates each redirected connection automatically — no special redirect handling is needed. Redirects to private IPs are blocked at the dial level.
- What happens when the blocked CIDRs configuration is malformed? The system logs an error on startup and falls back to default blocked ranges.
- What happens when `SSRF_ALLOW_PRIVATE_IPS=true` is set in production? The system logs a warning on startup to alert operators.

## Clarifications

### Session 2026-05-13

- Q: Scope of existing component refactoring — include SendGrid provider (which has NO SSRF protection currently) in scope? → A: Webhook + SendGrid — Include refactoring the SendGrid email provider to use the SSRF-safe client, covering the known vulnerability. SES and S3 (which use AWS SDK) are out of scope.
- Q: DNS resolution caching strategy — should resolved IPs be cached to reduce latency? → A: No DNS cache — resolve fresh on every outbound call at dial time. Maximum security, no cache window for DNS rebinding attacks.
- Q: HTTP redirect policy — what happens when a redirect targets a private IP? → A: Block at dial time — the transport's DialContext check catches redirects to private IPs automatically since each redirect creates a new connection validated through the same dial path. No special redirect handling needed.
- Q: Should fixed-target providers like SendGrid (api.sendgrid.com) be exempt from SSRF validation? → A: Apply uniformly — all outbound HTTP clients use the SSRF-safe transport. No exemptions. Simpler, more secure, negligible performance cost.
- Q: Should SSRF protection expose metrics/counters for blocked requests beyond structured logging? → A: Structured logs only — metrics counters deferred to the OpenTelemetry feature (Tier 3.2) to keep this feature scope tight.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST block all outbound HTTP requests targeting RFC 1918 private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) when SSRF protection is enabled
- **FR-002**: The system MUST block outbound requests targeting loopback addresses (127.0.0.0/8, ::1)
- **FR-003**: The system MUST block outbound requests targeting link-local addresses (169.254.0.0/16, fe80::/10), including cloud metadata endpoints (169.254.169.254)
- **FR-004**: The system MUST perform DNS resolution and validate the resolved IP address, not just the hostname, to prevent DNS rebinding attacks
- **FR-005**: The system MUST validate URL schemes, allowing only configured schemes (default: HTTPS only) for outbound requests
- **FR-006**: The system MUST support a configurable list of blocked hostnames that are denied regardless of their IP resolution (e.g., cloud metadata hostnames)
- **FR-007**: The system MUST support a configurable development mode that allows private IP requests (opt-in via `SSRF_ALLOW_PRIVATE_IPS`)
- **FR-008**: The system MUST support configurable allowed CIDRs that override specific blocked ranges (whitelist within the blocklist)
- **FR-009**: The system MUST provide a reusable HTTP client wrapper that applies SSRF protection at the transport/dial level, ensuring validation happens for every connection (including redirects)
- **FR-010**: The system MUST replace the existing inline SSRF logic in `webhook_worker.go`, `webhook.go`, and the SendGrid email provider (`email_sendgrid_provider.go`) with the new centralized SSRF protection package
- **FR-011**: The system MUST apply SSRF validation at webhook URL creation/update time (early rejection) AND at HTTP dial time (runtime protection against DNS rebinding)
- **FR-012**: The system MUST log all blocked requests with the blocked IP, hostname, and reason (private IP, blocked CIDR, blocked host, blocked scheme)
- **FR-013**: The system MUST use a fail-closed posture: if DNS resolution fails, the request is blocked
- **FR-014**: The system MUST handle IPv6-mapped IPv4 addresses (e.g., `::ffff:127.0.0.1`) by normalizing them before checking against blocked ranges
- **FR-015**: The system MUST NOT cache DNS resolution results — each outbound connection must perform a fresh DNS lookup at dial time to maximize security against DNS rebinding attacks
- **FR-016**: The system MUST apply SSRF protection uniformly to ALL outbound HTTP clients (webhook, SendGrid, and future providers) — no exemptions for fixed-target endpoints

### Key Entities

No database entities required. This is an infrastructure security layer consisting of:

- **SSRFConfig**: Configuration struct with BlockedCIDRs, BlockedHosts, AllowPrivateIPs, AllowedSchemes, DNSRebindProtection settings
- **SSRFValidator**: Validation engine that checks URLs and hostnames against blocked ranges, hosts, and schemes
- **SSRFSafeTransport**: Custom `http.RoundTripper` that intercepts dial operations to validate IPs at connection time
- **SSRFSafeClient**: Factory function that creates `*http.Client` instances with SSRF-protected transport

## Success Criteria

### Measurable Outcomes

- **SC-001**: All outbound HTTP requests from webhook workers targeting private/internal IP ranges are blocked 100% of the time (no bypass via IP address, CIDR range, or DNS rebinding)
- **SC-002**: Legitimate webhook deliveries to public HTTPS URLs continue to work without any degradation in delivery success rate
- **SC-003**: Developers can enable private IP access for local testing with a single configuration toggle (`SSRF_ALLOW_PRIVATE_IPS=true`) and receive a clear warning in logs
- **SC-004**: Adding a new outbound HTTP feature (e.g., OAuth callback) requires only importing the SSRF package and using the factory — no duplicate validation logic
- **SC-005**: All blocked requests produce structured log entries with clear security context (blocked IP, hostname, reason, URL), enabling security monitoring and incident response
- **SC-006**: The SSRF validation overhead adds less than 5ms latency per webhook delivery for the DNS resolution check

## Assumptions

- The existing `isPrivateIP` function and `NewHTTPClient` method in `webhook_worker.go` will be refactored out in favor of the centralized SSRF package
- SSRF protection is enforced at the transport/dial level in the HTTP client, not just at URL validation time, providing defense-in-depth against DNS rebinding
- Configuration follows the existing pattern in `internal/config/` with environment variable mapping via `mapstructure` tags
- Default configuration is production-secure: private IPs blocked, HTTPS-only scheme, cloud metadata hostnames blocked
- No database migration is required — SSRF protection is an infrastructure layer with configuration managed via environment variables
- SSRF protection is applied to webhook outbound calls as the primary use case, but the package design supports future use in OAuth callbacks, inbound webhook processing, and any other outbound HTTP call
- IPv6 support is included (blocking `::1`, `fc00::/7`, `fe80::/10`, etc.) alongside IPv4 ranges
- The `AllowPrivateIPs` development mode is clearly flagged in logs to prevent accidental production use
- Metrics/counters for blocked requests (e.g., `ssrf_blocked_total`) are out of scope for this feature and deferred to the OpenTelemetry integration feature (Tier 3.2)