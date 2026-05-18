# Feature Specification: Idempotency Keys

**Feature Branch**: `019-idempotency-keys`  
**Created**: 2026-05-13  
**Status**: Draft  
**Input**: User description: "Idempotency Keys - Prevents duplicate operations from retry mechanisms, network failures, and double-clicks. RFC 9110 standard. Uses Idempotency-Key header. Redis-backed key store with TTL. Returns cached response for duplicate requests with same key. Guards against key reuse with different payloads (409 Conflict). Middleware pattern that applies to all POST, PUT, PATCH endpoints."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Duplicate Request Protection (Priority: P1)

An API consumer's network connection drops after sending a POST request to create an invoice. The consumer's HTTP client automatically retries the same request. Without idempotency, this would create a duplicate invoice. With idempotency keys, the consumer sends an `Idempotency-Key` header. On retry, the API recognizes the key and returns the original response without creating a duplicate.

**Why this priority**: This is the core value proposition — preventing duplicate side effects from retry mechanisms. Without this, the entire feature has no value. This is the minimum viable implementation.

**Independent Test**: Send two identical POST requests with the same `Idempotency-Key` header. The second request must return the same response as the first without executing the business logic again.

**Acceptance Scenarios**:

1. **Given** a consumer sends `POST /api/v1/invoices` with `Idempotency-Key: abc-123` and valid payload, **When** the request succeeds and the same consumer retries with the same `Idempotency-Key: abc-123` and same payload, **Then** the API returns the original response (same status code and body) without creating a duplicate resource.

2. **Given** a consumer sends `POST /api/v1/invoices` with `Idempotency-Key: abc-123`, **When** the first request is still processing (in-flight), **Then** the second request with the same key waits or returns a 409 Conflict indicating the request is already being processed.

3. **Given** a consumer sends `POST /api/v1/invoices` with `Idempotency-Key: abc-123` and payload A, **When** the same consumer retries with `Idempotency-Key: abc-123` but a different payload B, **Then** the API returns 409 Conflict indicating the key is associated with a different request.

---

### User Story 2 - Automatic Key Expiration and Cleanup (Priority: P2)

Over time, idempotency records accumulate in Redis and the database. The system automatically expires old records after a configurable TTL (default 24 hours) and cleans them up via a background reaper, preventing unbounded storage growth.

**Why this priority**: Storage and performance management is essential for production but not required for the core idempotency mechanism to function.

**Independent Test**: Create an idempotency record, wait for the TTL to expire (or set a short TTL for testing), then send a request with the same key. The system should treat it as a new request rather than returning a cached response.

**Acceptance Scenarios**:

1. **Given** an idempotency record with a 24-hour TTL, **When** 24 hours have passed, **Then** the record is automatically expired and a new request with the same key is treated as a fresh request.

2. **Given** a background reaper configured with a 1-hour cleanup interval, **When** the reaper runs, **Then** all records past their TTL are removed from both Redis and the database.

3. **Given** a configurable TTL (minimum 1 hour, maximum 72 hours), **When** an administrator sets `IDEMPOTENCY_MAX_TTL=72h`, **Then** records created with that TTL expire after 72 hours.

---

### User Story 3 - Admin Visibility and Monitoring (Priority: P3)

An administrator needs to monitor idempotency key usage — how many keys are active, hit rates, expired counts — to understand API consumer behavior and detect potential abuse or misconfiguration.

**Why this priority**: Observability is important for production operations but doesn't affect the core idempotency mechanism. Can be added after the feature is working.

**Independent Test**: Make several idempotent requests, then query the monitoring endpoint to verify the counts and statuses match expectations.

**Acceptance Scenarios**:

1. **Given** 50 idempotent requests have been made in the last hour, **When** an administrator queries the idempotency stats endpoint, **Then** the response shows 50 total requests, with counts broken down by status (cache hit, new, conflict).

2. **Given** a consumer has sent 200 requests with idempotency keys in the last 24 hours, **When** an administrator queries for a specific consumer's usage, **Then** the response shows the consumer's key count, hit rate, and conflict rate.

---

### Edge Cases

- What happens when an idempotency key is reused after expiration? → Treated as a new request; processed normally.
- What happens when the Redis store is unavailable? → Fail open: process the request normally without idempotency protection, log a warning.
- What happens when a request with an idempotency key times out on the server side? → The record is stored as "in-flight" with a guard TTL (5 minutes, shorter than the main TTL). Subsequent requests with the same key receive 409 Conflict with a `Retry-After` header. The client must retry after the indicated time.
- What happens with very large request payloads? → Request body hash uses SHA-256 (fixed 64-char size) regardless of payload size.
- What happens when two different consumers use the same idempotency key? → Keys are scoped per consumer (user ID). Different consumers can use the same key without conflict.
- What happens when the key exceeds the maximum length? → Return 400 Bad Request with a descriptive error message.
- What happens with GET/DELETE requests? → Idempotency keys are ignored for GET and DELETE (these methods are already idempotent by HTTP specification).
- What happens with PATCH requests that have partial payload differences? → SHA-256 hash comparison catches any payload difference; 409 Conflict returned if payloads don't match.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept an `Idempotency-Key` HTTP header on POST, PUT, and PATCH requests per RFC 9110.
- **FR-002**: System MUST return the same response (status code and body) for subsequent requests with the same idempotency key and identical payload, without re-executing business logic. For responses ≤ 4KB, the full response body is cached in Redis. For responses larger than 4KB, only the status code and a reference identifier are cached in Redis; the response body is re-fetched from the database on replay.
- **FR-003**: System MUST reject requests that reuse an idempotency key with a different payload by returning 409 Conflict with an error message indicating the payload mismatch.
- **FR-004**: System MUST scope idempotency keys per user (authenticated user ID) so different users can safely use the same key without conflict.
- **FR-005**: System MUST store idempotency records with a configurable TTL (default 24 hours, minimum 1 hour, maximum 72 hours) after which the record expires and the key can be reused.
- **FR-006**: System MUST store idempotency records in Redis for fast lookup and optionally in the database for durability and auditing.
- **FR-007**: System MUST compute and store a SHA-256 hash of the request body to detect payload mismatches, regardless of payload size.
- **FR-008**: System MUST handle in-flight requests by storing a "processing" status with a shorter guard TTL (5 minutes). On receiving a concurrent request with the same key while the original is still processing, the system MUST return 409 Conflict immediately with a `Retry-After` response header indicating the approximate remaining processing time. The client is responsible for retrying. No connection holding or polling is required.
- **FR-009**: System MUST validate the idempotency key format: maximum 128 characters, alphanumeric plus hyphens and underscores (`^[a-zA-Z0-9_-]+$`), and return 400 Bad Request for invalid keys.
- **FR-010**: System MUST ignore the `Idempotency-Key` header for GET and DELETE requests since these methods are inherently idempotent.
- **FR-011**: System MUST clean up expired idempotency records via a background reaper running at a configurable interval (default 1 hour).
- **FR-012**: System MUST fail open if the Redis store is unavailable — process the request normally without idempotency protection and log a warning.
- **FR-013**: System MUST include `X-Idempotency-Key` and `X-Idempotency-Replayed: true` response headers when a cached response is returned, so clients can distinguish between fresh and replayed responses.
- **FR-014**: System MUST record audit logs when idempotency keys are created, replayed, or conflict with mismatched payloads.
- **FR-015**: System MUST enforce a maximum response body cache size of 4KB in Redis. Responses exceeding this threshold are stored with status code and a reference ID only; the response body is reconstructed from the database on replay.

### Key Entities *(include if feature involves data)*

- **IdempotencyRecord**: Stores the mapping between an idempotency key, request metadata (method, path, payload hash), response (status code, body or reference), and expiration timestamp. Key attributes: key (unique, per-user), user ID, request hash, HTTP method, path, cached status code, cached response body (up to 4KB, or reference ID for larger responses), response body size indicator, expiration time. Redis stores fast-lookup records with a 24-hour TTL; PostgreSQL stores full records for durability and reaper cleanup.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Duplicate requests with the same idempotency key return identical responses within 10ms of cache lookup, without re-executing business logic.
- **SC-002**: 100% of requests with mismatched payloads (same key, different body) receive a 409 Conflict response with a clear error message.
- **SC-003**: Idempotency protection adds no more than 5ms latency overhead to requests that do not include an `Idempotency-Key` header (pass-through path).
- **SC-004**: Expired records are automatically cleaned up within 2 hours of their TTL, keeping storage bounded.
- **SC-005**: When Redis is unavailable, 100% of requests continue to be processed normally without idempotency protection (fail-open, zero downtime).

## Clarifications

### Session 2026-05-13

- Q: How should the system handle a second request arriving while the first is still processing? → A: Return 409 Conflict immediately with a `Retry-After` header indicating approximate processing time. Client must retry. No connection holding or polling required.
- Q: How should the system handle caching of large response bodies? → A: Cache full response in Redis only for responses ≤ 4KB. For larger responses, store only the status code in Redis and re-fetch the result from the database when replaying.

## Assumptions

- Users have stable internet connectivity (not relevant to idempotency design, but standard web API assumption).
- The existing Redis infrastructure will be used for fast idempotency key lookups, with PostgreSQL as a durability layer for audit and reaper cleanup.
- Idempotency keys are scoped per authenticated user — unauthenticated endpoints are not covered by this feature.
- GET and DELETE methods do not need idempotency key support (they are already idempotent per HTTP spec).
- Maximum key length of 128 characters is sufficient (aligns with common API gateway conventions).
- The default 24-hour TTL balances storage cost with practical retry windows; most network retries happen within minutes, not days.
- The existing EventBus pattern will be used for audit logging of idempotency events, following the project's established conventions.
- The feature follows the existing middleware pattern established by `rate_limit.go` and `jwt.go`.
- Request body hashing uses SHA-256 which is collision-resistant for this use case (not a security hash, just a comparison mechanism).
- The in-flight guard TTL (5 minutes) is separate from and shorter than the main record TTL (24 hours).