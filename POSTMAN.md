# Postman Collection for Go API

## Files

| File | Purpose |
|------|---------|
| `postman_collection.json` | Postman collection with all endpoints |
| `postman_environment.json` | Environment variables for local development |

## Quick Start

### 1. Import into Postman

1. Open Postman
2. Click **Import** (top left)
3. Drag both files into the import window
4. Click **Import**

### 2. Set Up Environment

1. Select **Go API - Local Development** environment (top right dropdown)
2. Verify `base_url` is set to `http://localhost:8080`

### 3. Start the API

```bash
make serve
# or
go run ./cmd/api serve
```

## Testing Flow

### Basic Auth Flow

1. **Register User** → Creates a new user account
2. **Login** → Automatically saves `access_token`, `refresh_token`, and `user_id` to variables
3. **Protected endpoints** → Use `{{access_token}}` automatically (Bearer auth configured at collection level)

### Token Refresh Flow

1. Run **Refresh Token** → Generates new tokens from `{{refresh_token}}`
2. Tokens auto-update in variables

### Permission Testing

1. Run **Login** with admin user (created by `make seed`)
2. Use **List Roles** to find role IDs
3. Use **Assign Role to User** to grant permissions
4. Test permission-protected endpoints

### Session Management

1. Run **Login** on another device/browser
2. Run **List Sessions** to see all active sessions
3. Use **Revoke Session** or **Revoke All Other Sessions** to manage sessions

## Environment Variables

| Variable | Description | Auto-set |
|----------|-------------|----------|
| `base_url` | API base URL | Manual |
| `access_token` | JWT access token | Login/Refresh responses |
| `refresh_token` | JWT refresh token | Login/Refresh responses |
| `user_id` | Current user ID | Login response |
| `role_id` | Role ID for assignment | Manual |
| `permission_id` | Permission ID for grants | Manual |
| `session_id` | Session ID for revocation | Manual |

## Test Scripts

The collection includes automated tests for:

- ✅ Status code validation
- ✅ Response structure validation
- ✅ Token auto-save after login/refresh
- ✅ Token clearing on logout

## Password Requirements

All password fields require:
- 8-72 characters
- At least 1 uppercase letter
- At least 1 lowercase letter
- At least 1 number
- At least 1 special character (!@#$%^&*(),.?":{}|<>)

## Permission-Protected Endpoints

| Endpoint Group | Required Permission |
|----------------|---------------------|
| Users management | `users:manage` |
| Roles management | `roles:manage` |
| Permissions management | `permissions:manage` |

## Common Test Scenarios

### Scenario 1: New User Registration

```
1. POST /api/v1/auth/register
   Body: { "email": "new@example.com", "password": "Password123!" }
   
2. POST /api/v1/auth/login
   Body: { "email": "new@example.com", "password": "Password123!" }
   
3. GET /api/v1/me
   (Uses saved access_token automatically)
```

### Scenario 2: Password Reset

```
1. POST /api/v1/auth/password-reset
   Body: { "email": "user@example.com" }
   (Check server logs for reset token in development)

2. POST /api/v1/auth/password-reset/confirm
   Body: { "token": "reset-token-from-logs", "password": "NewPassword123!" }

3. POST /api/v1/auth/login
   Body: { "email": "user@example.com", "password": "NewPassword123!" }
```

### Scenario 3: Role Assignment

```
1. Login as admin user

2. GET /api/v1/roles
   (Copy a role ID from response)

3. POST /api/v1/users/:id/roles
   Path: {{user_id}}
   Body: { "role_id": "copied-role-id" }

4. GET /api/v1/users/:id/effective-permissions
   Path: {{user_id}}
```

## Troubleshooting

### 401 Unauthorized

- Token expired → Run **Refresh Token** or **Login** again
- Missing permission → Check with admin or assign role

### Validation Errors

- Check password complexity requirements
- Ensure email format is valid
- Verify JSON body structure matches collection examples

### Connection Refused

- Verify API is running: `curl http://localhost:8080/healthz`
- Check `base_url` in environment

## Development Tips

1. **Auto-save tokens**: Login responses automatically update collection variables
2. **Use variables**: Replace manual IDs with `{{user_id}}`, `{{role_id}}`, etc.
3. **Check Tests tab**: Each request has validation tests
4. **View Console**: Postman Console shows request/response details