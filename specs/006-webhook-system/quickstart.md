# Quickstart: Webhook System

**Feature**: 006-webhook-system | **Last Updated**: 2026-05-02

---

## Prerequisites

Webhooks require the base API running with the webhook migration applied. See main [quickstart](../../README.md) for base setup.

---

## Step 1: Run Webhook Migration

```bash
make migrate
# Creates: webhooks, webhook_deliveries tables
```

---

## Step 2: Sync Permissions

```bash
go run ./cmd/api permission:sync
# Adds: webhooks:view, webhooks:manage
```

---

## Step 3: Create a Webhook

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invoice Notifications",
    "url": "https://example.com/webhooks/invoices",
    "events": ["invoice.created", "invoice.paid"]
  }'
```

**Response** (201):
```json
{
  "data": {
    "id": "a1b2c3d4-...",
    "name": "Invoice Notifications",
    "url": "https://example.com/webhooks/invoices",
    "secret": "whsec_abc123...",
    "events": ["invoice.created", "invoice.paid"],
    "active": true,
    "rate_limit": 100,
    "created_at": "2026-05-02T10:00:00Z"
  },
  "meta": { "request_id": "...", "timestamp": "..." }
}
```

**Important**: Save the `secret` — it's only shown once on creation. Use it to verify webhook signatures on your endpoint.

---

## Step 4: Verify Webhook Signatures (Receiver Side)

When your endpoint receives a webhook:

```go
// Go example
func verifyWebhook(body []byte, sigHeader string, secret string) bool {
    // Parse: t=1683456789,v1=abcdef...
    parts := strings.Split(sigHeader, ",")
    var timestamp, signature string
    for _, p := range parts {
        if strings.HasPrefix(p, "t=") {
            timestamp = strings.TrimPrefix(p, "t=")
        } else if strings.HasPrefix(p, "v1=") {
            signature = strings.TrimPrefix(p, "v1=")
        }
    }

// Compute expected signature
// NOTE: Strip the "whsec_" prefix from the secret before using as HMAC key
secret = strings.TrimPrefix(secret, "whsec_")
signedPayload := timestamp + "." + string(body)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(signedPayload))
    expected := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(expected))
}
```

---

## Step 5: List Webhooks

```bash
curl -X GET http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

---

## Step 6: View Delivery History

```bash
curl -X GET "http://localhost:8080/api/v1/webhooks/{id}/deliveries?limit=20" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

---

## Step 7: Replay a Failed Delivery

```bash
curl -X POST "http://localhost:8080/api/v1/webhooks/{id}/deliveries/{delivery_id}/replay" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_WORKER_CONCURRENCY` | 5 | Number of delivery workers |
| `WEBHOOK_RETRY_MAX` | 3 | Max delivery attempts |
| `WEBHOOK_RATE_LIMIT` | 100 | Default rate limit (deliveries/min) |
| `WEBHOOK_ALLOW_HTTP` | false | Allow HTTP URLs (dev only) |
| `WEBHOOK_DELIVERY_TIMEOUT` | 10s | HTTP client timeout |
| `WEBHOOK_DELIVERY_RETENTION_DAYS` | 90 | Delivery record retention |
| `WEBHOOK_MAX_PAYLOAD_SIZE` | 1048576 | Max payload size in bytes (1MB) |

---

## Webhook Payload Format

```json
{
  "id": "a1b2c3d4-...",
  "event": "invoice.created",
  "timestamp": "2026-05-02T10:00:00Z",
  "data": {
    "id": "invoice-uuid",
    "amount": 15000,
    "status": "draft"
  }
}
```

**Request Headers**:
```
Content-Type: application/json
X-Webhook-ID: a1b2c3d4-...
X-Webhook-Event: invoice.created
X-Webhook-Signature: t=1683456789,v1=abcdef...
X-Webhook-Attempt: 1
User-Agent: Go-API-Webhook/1.0
```

---

**Questions?** See [contracts/api-v1.md](./contracts/api-v1.md) for full endpoint specs.