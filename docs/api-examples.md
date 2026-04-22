# API Usage Examples

This document provides practical examples for using the Go API Base REST API.

## Table of Contents

1. [Authentication Flow](#authentication-flow)
2. [Working with Users](#working-with-users)
3. [Managing Roles and Permissions](#managing-roles-and-permissions)
4. [Invoice Management](#invoice-management)
5. [Common Workflows](#common-workflows)

## Base URL

All API endpoints are available at:

```
http://localhost:8080/api/v1
```

Swagger documentation is available at:

```
http://localhost:8080/swagger/index.html
```

## Authentication Flow

### 1. Register a New User

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePassword123!",
    "password_confirm": "SecurePassword123!"
  }'
```

**Response:**

```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com"
  },
  "meta": {
    "request_id": "abc123",
    "timestamp": "2025-04-22T10:30:00Z"
  }
}
```

### 2. Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePassword123!"
  }'
```

**Response:**

```json
{
  "data": {
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com"
    },
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "meta": {
    "request_id": "abc124",
    "timestamp": "2025-04-22T10:31:00Z"
  }
}
```

### 3. Use Access Token for Authenticated Requests

Include the access token in the `Authorization` header:

```bash
curl -X GET http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 4. Refresh Tokens

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }'
```

### 5. Logout

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "X-User-ID: 550e8400-e29b-41d4-a716-446655440000"
```

## Working with Users

### Get Current User Profile

```bash
curl -X GET http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access_token>"
```

### List All Users (Admin Only)

```bash
curl -X GET http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer <admin_access_token>"
```

### Get User by ID

```bash
curl -X GET http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer <access_token>"
```

### Assign Role to User

```bash
curl -X POST http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000/roles \
  -H "Authorization: Bearer <admin_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7"
  }'
```

### Get User's Roles

```bash
curl -X GET http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000/roles \
  -H "Authorization: Bearer <access_token>"
```

### Get User's Effective Permissions

```bash
curl -X GET http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000/effective-permissions \
  -H "Authorization: Bearer <access_token>"
```

## Managing Roles and Permissions

### Create a Role

```bash
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer <admin_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "editor",
    "description": "Content editor role with limited permissions"
  }'
```

### List All Roles

```bash
curl -X GET http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer <access_token>"
```

### Create a Permission

```bash
curl -X POST http://localhost:8080/api/v1/permissions \
  -H "Authorization: Bearer <admin_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "invoices:approve",
    "resource": "invoices",
    "action": "approve",
    "scope": "all"
  }'
```

**Permission Scopes:**
- `own` - User can only manage their own resources
- `all` - User can manage all resources (admin-level)

### Attach Permission to Role

```bash
curl -X POST http://localhost:8080/api/v1/roles/7c9e6679-7425-40de-944b-e07fc1f90ae7/permissions/3b9e6679-7425-40de-944b-e07fc1f90ae7 \
  -H "Authorization: Bearer <admin_access_token>"
```

### Grant Permission Directly to User

```bash
curl -X POST http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000/permissions \
  -H "Authorization: Bearer <admin_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "permission_id": "3b9e6679-7425-40de-944b-e07fc1f90ae7",
    "effect": "allow"
  }'
```

### Deny Permission to User

```bash
curl -X POST http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000/permissions \
  -H "Authorization: Bearer <admin_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "permission_id": "3b9e6679-7425-40de-944b-e07fc1f90ae7",
    "effect": "deny"
  }'
```

## Invoice Management

### Create an Invoice

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "customer": "Acme Corporation",
    "amount": 1500.00,
    "description": "Consulting services for Q1 2025",
    "items": [
      {
        "description": "Strategy consulting",
        "quantity": 10,
        "unit_price": 150.00
      }
    ]
  }'
```

### List Invoices

Regular users see only their own invoices:

```bash
curl -X GET "http://localhost:8080/api/v1/invoices?limit=20&offset=0" \
  -H "Authorization: Bearer <access_token>"
```

Admins see all invoices:

```bash
curl -X GET "http://localhost:8080/api/v1/invoices?limit=20&offset=0" \
  -H "Authorization: Bearer <admin_access_token>"
```

### Get Invoice by ID

```bash
curl -X GET http://localhost:8080/api/v1/invoices/660e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer <access_token>"
```

### Update an Invoice

```bash
curl -X PUT http://localhost:8080/api/v1/invoices/660e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "customer": "Acme Corporation",
    "amount": 2000.00,
    "description": "Updated consulting services",
    "status": "pending"
  }'
```

### Update Invoice Status (Admin Only)

```bash
curl -X PATCH "http://localhost:8080/api/v1/invoices/660e8400-e29b-41d4-a716-446655440000/status?status=paid" \
  -H "Authorization: Bearer <admin_access_token>"
```

**Invoice Statuses:**
- `draft` - Initial state
- `pending` - Sent to customer
- `paid` - Payment received
- `cancelled` - Invoice cancelled

### Delete an Invoice

```bash
curl -X DELETE http://localhost:8080/api/v1/invoices/660e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer <access_token>"
```

## Common Workflows

### Setup a New Admin User

1. **Register the user:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePassword123!",
    "password_confirm": "SecurePassword123!"
  }'
```

2. **Login as an existing admin:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "superadmin@example.com",
    "password": "AdminPassword123!"
  }'
```

3. **Create necessary permissions (if not exists):**

```bash
# Create users:manage permission
curl -X POST http://localhost:8080/api/v1/permissions \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "users:manage",
    "resource": "users",
    "action": "manage",
    "scope": "all"
  }'
```

4. **Create admin role:**

```bash
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "admin",
    "description": "Administrator with full access"
  }'
```

5. **Attach permissions to role:**

```bash
curl -X POST http://localhost:8080/api/v1/roles/<role_id>/permissions/<permission_id> \
  -H "Authorization: Bearer <admin_token>"
```

6. **Assign role to new user:**

```bash
curl -X POST http://localhost:8080/api/v1/users/<user_id>/roles \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role_id": "<admin_role_id>"
  }'
```

### Complete Invoice Workflow

1. **Create invoice:**

```bash
curl -X POST http://localhost:8080/api/v1/invoices \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "customer": "Client Inc",
    "amount": 5000.00,
    "description": "Project development"
  }'
```

2. **Admin approves invoice:**

```bash
curl -X PATCH "http://localhost:8080/api/v1/invoices/<invoice_id>/status?status=pending" \
  -H "Authorization: Bearer <admin_token>"
```

3. **Mark as paid after payment:**

```bash
curl -X PATCH "http://localhost:8080/api/v1/invoices/<invoice_id>/status?status=paid" \
  -H "Authorization: Bearer <admin_token>"
```

## Error Handling

All errors follow a consistent format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": "email must be a valid email address"
  },
  "meta": {
    "request_id": "abc123",
    "timestamp": "2025-04-22T10:30:00Z"
  }
}
```

**Common Error Codes:**
- `BAD_REQUEST` - Invalid request format
- `VALIDATION_ERROR` - Field validation failed
- `UNAUTHORIZED` - Missing or invalid authentication
- `FORBIDDEN` - Permission denied
- `NOT_FOUND` - Resource not found
- `INTERNAL_ERROR` - Server error

## Health Endpoints

### Liveness Probe

```bash
curl http://localhost:8080/healthz
```

**Response:**

```json
{
  "status": "ok"
}
```

### Readiness Probe

```bash
curl http://localhost:8080/readyz
```

**Response (healthy):**

```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

**Response (unhealthy):**

```json
{
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "message": "Service is not ready",
    "details": {
      "database": "ok",
      "redis": "error: connection refused"
    }
  },
  "meta": {
    "request_id": "abc125",
    "timestamp": "2025-04-22T10:35:00Z"
  }
}
```

## Rate Limiting

The API implements rate limiting. If you exceed the limit, you'll receive:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests, please try again later"
  },
  "meta": {
    "request_id": "abc126",
    "timestamp": "2025-04-22T10:40:00Z"
  }
}
```

Check your rate limit status via response headers:
- `X-RateLimit-Limit` - Maximum requests per window
- `X-RateLimit-Remaining` - Remaining requests
- `X-RateLimit-Reset` - Unix timestamp when limit resets