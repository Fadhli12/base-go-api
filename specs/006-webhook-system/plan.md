# Implementation Plan: Webhook System

**Branch**: `006-webhook-system` | **Date**: 2026-05-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `docs/FEATURE_RECOMMENDATIONS.md` Section 1.5

## Summary

Implement an outbound webhook system allowing external integrations to subscribe to application events (user.created, invoice.paid, etc.) and receive HMAC-SHA256 signed HTTP payloads with retry logic, delivery tracking, and per-webhook rate limiting. The system follows existing repository/service/handler patterns, uses the same async worker architecture as the email service, and stores delivery records as immutable audit trail.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Echo v4, GORM, Casbin, Redis (go-redis/v9), PostgreSQL, testcontainers-go  
**Storage**: PostgreSQL (webhooks, webhook_deliveries), Redis (delivery queue, rate limiting)  
**Testing**: testify (require/assert), testcontainers-go for integration, mockery for unit mocks  
**Target Platform**: Linux server (Docker)  
**Project Type**: Web service (REST API)  
**Performance Goals**: 1000 req/s sustained, <200ms p95 webhook creation, async delivery  
**Constraints**: 30s graceful shutdown, immutable delivery records, HMAC-SHA256 signatures, 90-day delivery retention  
**Scale/Scope**: Multi-tenant (organization-scoped), up to 100 webhooks per org, 100 deliverables/min per webhook

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Production-Ready Foundation | ✅ PASS | Async workers, structured logging, graceful shutdown, error handling |
| II. RBAC Runtime-Configurable | ✅ PASS | `webhooks:view`, `webhooks:manage` permissions synced via `permission:sync` |
| III. Soft Deletes for Audit | ✅ PASS | `webhooks` has soft delete; `webhook_deliveries` is immutable (like audit_logs) |
| IV. Stateless JWT + Revocation | ✅ PASS | No JWT changes; webhook endpoints behind JWT middleware |
| V. SQL Migrations (Not AutoMigrate) | ✅ PASS | Migration 000014_webhooks.up.sql / .down.sql |
| VI. Multi-Instance Consistency | ✅ PASS | Redis queue ensures delivery consistency across instances |
| VII. Audit Logging Non-Negotiable | ✅ PASS | All webhook CRUD operations audit-logged |

## Project Structure

### Documentation (this feature)

```text
specs/006-webhook-system/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── api-v1.md        # REST API contracts
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── domain/
│   ├── webhook.go                # Webhook + WebhookDelivery entities, event constants
│   └── webhook_events.go         # Event definitions and publisher interface
├── repository/
│   └── webhook.go                 # WebhookRepository + WebhookDeliveryRepository interfaces + impls
├── service/
│   ├── webhook.go                 # WebhookService (CRUD + event dispatch)
│   └── webhook_worker.go          # WebhookWorker (async delivery)
├── http/
│   ├── handler/
│   │   └── webhook.go             # WebhookHandler (HTTP endpoints)
│   ├── middleware/
│   │   └── webhook_signature.go   # HMAC signature verification (inbound)
│   └── request/
│       └── webhook.go             # Request DTOs with validation
├── config/
│   └── webhook.go                 # WebhookConfig (env vars)
└── permission/
    └── (existing)                  # Add webhooks permissions to manifest

migrations/
├── 000014_webhooks.up.sql         # Create webhooks + webhook_deliveries tables
└── 000014_webhooks.down.sql        # Rollback

config/
└── permissions.yaml               # Add webhooks:view, webhooks:manage

tests/
├── integration/
│   ├── webhook_test.go             # Service-layer integration tests
│   └── webhook_handler_test.go     # HTTP handler integration tests
└── unit/
    ├── webhook_service_test.go      # WebhookService unit tests
    └── webhook_worker_test.go       # Worker retry/delivery tests
```

**Structure Decision**: Follows existing project layout — domain entities in `internal/domain/`, repositories in `internal/repository/`, services in `internal/service/`, handlers in `internal/http/handler/`, request DTOs in `internal/http/request/`. Migration files in `migrations/`.

## Complexity Tracking

> No constitution violations. All patterns follow established conventions.