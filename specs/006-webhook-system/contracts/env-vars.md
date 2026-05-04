# Environment Variables: Webhook System

**Feature**: 006-webhook-system | **Date**: 2026-05-02

---

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_WORKER_CONCURRENCY` | `5` | Number of goroutine workers for delivery |
| `WEBHOOK_RETRY_MAX` | `3` | Maximum retry attempts per delivery |
| `WEBHOOK_RATE_LIMIT` | `100` | Default per-webhook rate limit (deliveries/min) |
| `WEBHOOK_ALLOW_HTTP` | `false` | Allow non-HTTPS webhook URLs (dev only) |
| `WEBHOOK_DELIVERY_TIMEOUT` | `10s` | HTTP client timeout for outbound deliveries |
| `WEBHOOK_DELIVERY_RETENTION_DAYS` | `90` | Days to retain delivery records |
| `WEBHOOK_MAX_PAYLOAD_SIZE` | `1048576` | Max payload size in bytes (1MB) |

---

All variables loaded via Viper (existing config pattern). Added to `internal/config/webhook.go`.