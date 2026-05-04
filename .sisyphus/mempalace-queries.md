# MemPalace Semantic Search Queries

Quick reference for common searches in the Go API project.

## Authentication & JWT

```bash
mempalace search "How do we handle JWT token refresh?"
mempalace search "Where is JWT validation implemented?"
mempalace search "How do we generate access tokens?"
mempalace search "Where are refresh tokens stored?"
mempalace search "How do we handle token expiration?"
mempalace search "JWT middleware chain"
mempalace search "token claims structure"
```

## RBAC & Permissions

```bash
mempalace search "How does Casbin RBAC work?"
mempalace search "Where is permission validation implemented?"
mempalace search "How do we check user permissions?"
mempalace search "Where are roles defined?"
mempalace search "How do we enforce organization scoping?"
mempalace search "Casbin enforcer initialization"
mempalace search "permission cache invalidation"
mempalace search "domain-based access control"
```

## Webhooks

```bash
mempalace search "What's the webhook delivery retry strategy?"
mempalace search "How do we sign webhook payloads?"
mempalace search "Where is webhook rate limiting implemented?"
mempalace search "How do we handle stuck deliveries?"
mempalace search "What's the webhook event dispatch flow?"
mempalace search "webhook worker goroutine pool"
mempalace search "webhook queue Redis implementation"
mempalace search "HMAC-SHA256 webhook signing"
mempalace search "webhook delivery backoff exponential"
```

## Logging & Monitoring

```bash
mempalace search "How do we structure log output?"
mempalace search "Where is request logging implemented?"
mempalace search "How do we propagate context in logs?"
mempalace search "What fields are automatically logged?"
mempalace search "How do we configure log outputs?"
mempalace search "structured logging slog"
mempalace search "request ID context propagation"
mempalace search "log level configuration"
mempalace search "multiple log writers"
```

## Error Handling

```bash
mempalace search "How do we structure error responses?"
mempalace search "Where are custom errors defined?"
mempalace search "How do we handle validation errors?"
mempalace search "Where is error mapping implemented?"
mempalace search "How do we return HTTP error codes?"
mempalace search "error envelope response format"
mempalace search "validation error messages"
mempalace search "HTTP status code mapping"
```

## Database & Repositories

```bash
mempalace search "How do we implement soft deletes?"
mempalace search "Where are repositories implemented?"
mempalace search "How do we structure GORM queries?"
mempalace search "Where are migrations organized?"
mempalace search "How do we handle database transactions?"
mempalace search "soft delete scope GORM"
mempalace search "repository pattern implementation"
mempalace search "UUID primary keys"
mempalace search "database migration golang-migrate"
mempalace search "context propagation GORM"
```

## Testing

```bash
mempalace search "How do we structure integration tests?"
mempalace search "Where are test fixtures defined?"
mempalace search "How do we use testcontainers?"
mempalace search "Where are unit tests organized?"
mempalace search "How do we mock services?"
mempalace search "testcontainers Postgres Redis"
mempalace search "test suite setup teardown"
mempalace search "integration test tags"
mempalace search "mock service implementation"
```

## Architecture & Design

```bash
mempalace search "What's the overall architecture?"
mempalace search "How do we structure domain services?"
mempalace search "Where is the middleware chain defined?"
mempalace search "How do we handle graceful shutdown?"
mempalace search "What's the initialization order?"
mempalace search "Echo server setup"
mempalace search "dependency injection pattern"
mempalace search "service layer architecture"
mempalace search "handler request validation"
mempalace search "graceful shutdown 30 seconds"
```

## Event System

```bash
mempalace search "How does EventBus work?"
mempalace search "Where are events published?"
mempalace search "How do we subscribe to events?"
mempalace search "event-driven architecture"
mempalace search "Go channel event bus"
mempalace search "event payload structure"
mempalace search "event subscriber pattern"
```

## Organization & Multi-Tenancy

```bash
mempalace search "How do we implement organization scoping?"
mempalace search "Where is organization context extracted?"
mempalace search "How do we enforce org-level permissions?"
mempalace search "organization member roles"
mempalace search "X-Organization-ID header"
mempalace search "domain-based organization isolation"
```

## Configuration & Startup

```bash
mempalace search "How do we load configuration?"
mempalace search "Where are environment variables used?"
mempalace search "How do we initialize services?"
mempalace search "startup sequence order"
mempalace search "Viper configuration"
mempalace search "database connection pooling"
mempalace search "Redis connection setup"
```

## Middleware

```bash
mempalace search "Where is middleware chain defined?"
mempalace search "How do we implement custom middleware?"
mempalace search "request recovery middleware"
mempalace search "CORS middleware configuration"
mempalace search "rate limiting middleware"
mempalace search "authentication middleware"
mempalace search "organization context middleware"
```

## Specific Features

### Invoice Module
```bash
mempalace search "invoice service implementation"
mempalace search "invoice status workflow"
mempalace search "invoice repository pattern"
mempalace search "invoice event publishing"
```

### User Management
```bash
mempalace search "user creation validation"
mempalace search "user soft delete"
mempalace search "user repository"
mempalace search "user event publishing"
```

### News/Content
```bash
mempalace search "news service implementation"
mempalace search "news publication workflow"
mempalace search "news event publishing"
```

## Performance & Optimization

```bash
mempalace search "database query optimization"
mempalace search "connection pooling configuration"
mempalace search "caching strategy"
mempalace search "permission cache TTL"
mempalace search "goroutine pool sizing"
mempalace search "webhook worker concurrency"
```

## Security

```bash
mempalace search "JWT secret management"
mempalace search "password hashing"
mempalace search "HMAC signature verification"
mempalace search "SSDRF validation webhook"
mempalace search "permission enforcement"
mempalace search "role-based access control"
```

## Debugging Tips

### Find Implementation
```bash
mempalace search "where is [feature] implemented"
mempalace search "[feature] handler"
mempalace search "[feature] service"
mempalace search "[feature] repository"
```

### Understand Pattern
```bash
mempalace search "how do we [action]"
mempalace search "[pattern] implementation"
mempalace search "[pattern] example"
```

### Find Related Code
```bash
mempalace search "[module] integration"
mempalace search "[module] dependencies"
mempalace search "[module] event flow"
```

---

## Tips for Better Searches

1. **Be Specific**: `"webhook delivery retry exponential backoff"` > `"webhook"`
2. **Use Questions**: `"How do we..."` often returns better results than statements
3. **Include Context**: `"webhook rate limiting per webhook"` > `"rate limiting"`
4. **Search by Pattern**: `"repository pattern"`, `"middleware chain"`, `"service layer"`
5. **Use Room Filters**: `mempalace search "query" --room internal` to narrow results

---

## Common Search Workflows

### Before Implementing a Feature
1. Search for similar features: `"How do we implement [feature]?"`
2. Find related services: `"[feature] service"`
3. Check event publishing: `"[feature] event"`
4. Review tests: `"[feature] test"`

### Before Fixing a Bug
1. Search for error handling: `"[error message]"`
2. Find related code: `"[module] implementation"`
3. Check validation: `"[field] validation"`
4. Review tests: `"[module] test"`

### Before Refactoring
1. Find all usages: `"[function/struct] usage"`
2. Check dependencies: `"[module] dependencies"`
3. Review tests: `"[module] test"`
4. Find related patterns: `"[pattern] implementation"`

---

## Saved Searches (Bookmark These)

```bash
# Architecture overview
mempalace search "overall architecture initialization order"

# Permission system
mempalace search "Casbin RBAC enforcer permission check"

# Event system
mempalace search "EventBus publish subscribe event-driven"

# Webhook system
mempalace search "webhook delivery retry worker queue"

# Logging
mempalace search "structured logging slog context propagation"

# Testing
mempalace search "integration test testcontainers setup"

# Error handling
mempalace search "error response envelope HTTP status"
```
