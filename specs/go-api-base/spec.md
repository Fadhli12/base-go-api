# Feature Specification: Team/Organization Support

**Version**: 1.0 | **Date**: 2026-04-24 | **Status**: Draft

---

## Overview

### Problem Statement

The Go API Base currently operates in a single-tenant mode, where all users have equal access to all resources. This limitation prevents organizations from having isolated workspaces with their own members, roles, and resource scopes. Multi-tenant support is essential for SaaS products, enterprise offerings, and B2B use cases.

### Proposed Solution

Add comprehensive multi-tenant organization support that:
- Allows users to create and join organizations
- Scopes all resources (news, invoices, media) to organizations
- Integrates with existing Casbin RBAC using domain-based permissions
- Provides organization-level member management (owner, admin, member)
- Maintains full audit trail compliance

### Success Criteria

- ✅ Users can create organizations with unique slugs
- ✅ Organization owners can invite/remove members
- ✅ All resources are scoped to organization via `X-Organization-ID` header
- ✅ Casbin enforces organization-scoped permissions
- ✅ 100% backward compatible with existing single-tenant behavior
- ✅ Full audit trail for all organization operations
- ✅ Integration tests pass with testcontainers

---

## Requirements

### Functional Requirements

#### FR-1: Organization Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | Users can create organizations with name and unique slug | MUST |
| FR-1.2 | Organization creators become owners automatically | MUST |
| FR-1.3 | Owners can update organization name and settings | MUST |
| FR-1.4 | Owners can delete (soft delete) organizations | MUST |
| FR-1.5 | Organization slugs must be unique across the platform | MUST |
| FR-1.6 | Organizations store owner_id, name, slug, and JSONB settings | MUST |

#### FR-2: Membership Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | Users can join organizations via invitation | MUST |
| FR-2.2 | Owners and admins can invite new members | MUST |
| FR-2.3 | Members have roles: owner, admin, or member | MUST |
| FR-2.4 | Owners can promote members to admin | MUST |
| FR-2.5 | Owners and admins can remove members | MUST |
| FR-2.6 | Owners cannot be removed from their own organization | MUST |
| FR-2.7 | Each user can belong to multiple organizations | MUST |

#### FR-3: Resource Scoping

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | All existing resources (news, invoices, media) gain organization_id FK | MUST |
| FR-3.2 | Resources created without organization_id are globally accessible | MUST |
| FR-3.3 | Resources scoped to organization only visible to members | MUST |
| FR-3.4 | X-Organization-ID header sets tenant context for requests | MUST |
| FR-3.5 | Missing X-Organization-ID header treated as global context | MUST |

#### FR-4: Permission System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | organization:view - View organization details | MUST |
| FR-4.2 | organization:manage - Update organization settings | MUST |
| FR-4.3 | organization:invite - Invite new members | MUST |
| FR-4.4 | organization:remove - Remove members | MUST |
| FR-4.5 | Permissions enforced via Casbin with domain = organization_id | MUST |
| FR-4.6 | Role hierarchy: owner > admin > member | MUST |

### Non-Functional Requirements

#### NFR-1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | Organization context extraction | < 5ms overhead |
| NFR-1.2 | Membership validation | < 10ms with caching |
| NFR-1.3 | Permission check (Casbin) | < 15ms with Redis cache |

#### NFR-2: Security

| ID | Requirement |
|----|-------------|
| NFR-2.1 | Organization ID validation (UUID format) |
| NFR-2.2 | Membership verification before any org operation |
| NFR-2.3 | Soft deletes prevent accidental data loss |
| NFR-2.4 | Audit log captures all org mutations with actor |

#### NFR-3: Compatibility

| ID | Requirement |
|----|-------------|
| NFR-3.1 | Existing API endpoints continue working without organization context |
| NFR-3.2 | No breaking changes to existing entities (backward compatible) |
| NFR-3.3 | Existing permissions still work in global context |

---

## Data Model

### Entities

#### Organization

```go
type Organization struct {
    ID        uuid.UUID      // Primary key
    Name      string         // Organization name (1-255 chars)
    Slug      string         // Unique slug for URLs (1-100 chars)
    OwnerID   uuid.UUID      // Creator/owner reference
    Settings  JSONB          // Flexible settings storage
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt // Soft delete
}
```

#### OrganizationMember

```go
type OrganizationMember struct {
    OrganizationID uuid.UUID  // FK to Organization
    UserID         uuid.UUID  // FK to User
    Role           string     // 'owner', 'admin', 'member'
    JoinedAt       time.Time  // Membership timestamp
    
    // Composite PK: (organization_id, user_id)
}
```

### Relationships

```
User (1) ─────< (N) OrganizationMember (N) >───── (1) Organization
                      │
                   Role: owner/admin/member

Organization (1) ────< (N) Resources (News, Invoice, Media)
                            │
                    organization_id FK
```

### Indexes

- `organizations(slug)` - UNIQUE for slug lookups
- `organizations(owner_id)` - For owner queries
- `organizations(deleted_at)` - Soft delete index
- `organization_members(user_id)` - User's memberships
- `organization_members(role)` - Role filtering
- `{news,invoices,media}(organization_id)` - Resource scoping

---

## API Endpoints

### Organization Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/organizations` | Create organization |
| GET | `/api/organizations` | List user's organizations |
| GET | `/api/organizations/:id` | Get organization details |
| PUT | `/api/organizations/:id` | Update organization |
| DELETE | `/api/organizations/:id` | Delete organization |

### Member Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/organizations/:id/members` | Add member to organization |
| GET | `/api/organizations/:id/members` | List organization members |
| DELETE | `/api/organizations/:id/members/:user_id` | Remove member |

### Request/Response Examples

#### Create Organization

```http
POST /api/organizations
Content-Type: application/json
Authorization: Bearer {token}

{
  "name": "Acme Corporation",
  "slug": "acme-corp",
  "settings": {
    "timezone": "UTC",
    "features": ["invoice", "news"]
  }
}

Response 201:
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corporation",
    "slug": "acme-corp",
    "owner_id": "123e4567-e89b-12d3-a456-426614174000",
    "settings": {...},
    "created_at": "2026-04-24T10:00:00Z"
  }
}
```

#### Add Member

```http
POST /api/organizations/:id/members
Content-Type: application/json
Authorization: Bearer {token}

{
  "user_id": "user-uuid",
  "role": "member"
}

Response 201:
{
  "data": {
    "organization_id": "org-uuid",
    "user_id": "user-uuid",
    "role": "member",
    "joined_at": "2026-04-24T10:05:00Z"
  }
}
```

---

## Permission Model

### Casbin Integration

The organization feature extends the existing Casbin RBAC with domain support:

```
Model:
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

### Permissions

| Permission | Resource | Action | Description |
|------------|----------|--------|-------------|
| organization:view | organization | view | View organization details |
| organization:manage | organization | manage | Update organization settings |
| organization:invite | organization | invite | Invite new members |
| organization:remove | organization | remove | Remove members |

### Permission Grants

```
# Owner has all permissions
g, {user_id}, owner, {org_id}
p, owner, {org_id}, organization, view
p, owner, {org_id}, organization, manage
p, owner, {org_id}, organization, invite
p, owner, {org_id}, organization, remove

# Admin has management and invite permissions
g, {user_id}, admin, {org_id}
p, admin, {org_id}, organization, view
p, admin, {org_id}, organization, manage
p, admin, {org_id}, organization, invite

# Member has view permission
g, {user_id}, member, {org_id}
p, member, {org_id}, organization, view
```

---

## Implementation Phases

### Phase 1: Database Schema (Day 1 AM)

- Create migration `000003_organizations.up.sql`
- Add organizations table with soft deletes
- Add organization_members table
- Add organization_id to news, invoices, media
- Create indexes for performance
- Test rollback migration

### Phase 2: Domain Models (Day 1 PM)

- Define Organization entity with validation
- Define OrganizationMember entity
- Implement ToResponse() methods
- Add helper methods (IsOwner, IsAdmin)

### Phase 3: Repository Layer (Day 1 PM)

- OrganizationRepository interface
- OrganizationMemberRepository interface
- Implement with GORM contexts
- Add member role queries

### Phase 4: Service Layer (Day 2 AM)

- OrganizationService with business logic
- Permission checks via Casbin enforcer
- Audit logging for all mutations
- Member management methods

### Phase 5: HTTP Handlers (Day 2 PM)

- Create OrganizationHandler
- CRUD endpoints for organizations
- Member management endpoints
- Request/response DTOs

### Phase 6: Middleware (Day 2 PM)

- OrganizationContext middleware for header extraction
- RequireOrganization middleware
- Membership validation middleware
- Integration into existing middleware chain

### Phase 7: Permission Seeding (Day 3 AM)

- Add organization permissions to seed data
- Create organization role definitions
- Add default permissions to permission:sync

### Phase 8: Scoping Existing Resources (Day 3 AM)

- Update News entity queries for organization filtering
- Update Invoice entity queries
- Update Media entity queries
- Ensure backward compatibility

### Phase 9: Integration Tests (Day 3 PM)

- Test organization CRUD
- Test membership management
- Test permission enforcement
- Test resource scoping
- Achieve 80%+ coverage

### Phase 10: Documentation (Day 3 PM)

- Swagger annotations for all endpoints
- Update AGENTS.md with organization patterns
- Document permission model
- Add examples to quickstart guide

---

## Architecture Patterns

All implementations MUST follow existing patterns:

### Repository Pattern

```go
type Repository interface {
    Create(ctx context.Context, entity *Entity) error
    FindByID(ctx context.Context, id uuid.UUID) (*Entity, error)
    Update(ctx context.Context, entity *Entity) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

### Service Pattern

```go
type Service struct {
    repo     Repository
    enforcer *permission.Enforcer
    audit    *AuditService
}

func (s *Service) Operation(ctx context.Context, userID uuid.UUID, req Request) (*Entity, error) {
    // 1. Validate input
    // 2. Check permissions
    // 3. Execute business logic
    // 4. Log audit
    // 5. Return result
}
```

### Handler Pattern

```go
func (h *Handler) Operation(c echo.Context) error {
    // 1. Get user context
    // 2. Parse request
    // 3. Validate
    // 4. Call service
    // 5. Handle errors
    // 6. Return response
}
```

---

## Testing Strategy

### Unit Tests

- Repository operations with mock DB
- Service layer business logic
- Permission checking logic
- Middleware context extraction

### Integration Tests

- Full CRUD flow with testcontainers
- Permission enforcement scenarios
- Casbin policy operations
- Resource scoping queries

### Test Coverage Requirements

- Minimum 80% coverage on business logic
- All repository methods tested
- All service methods tested
- All handlers tested

---

## Migration Path

### For Existing Installations

1. Migration adds `organization_id` columns as NULLABLE
2. Existing resources remain globally accessible (NULL organization_id)
3. No breaking changes to existing queries
4. Users can opt into organization scoping gradually

### Migration Steps

```bash
# 1. Run migration
make migrate

# 2. Verify schema
psql -d go_api -c "\d organizations"
psql -d go_api -c "\d organization_members"

# 3. Seed permissions
go run ./cmd/api permission:sync

# 4. Create first organization (optional)
# Via API POST /api/organizations
```

---

## Success Metrics

- [ ] All 5 organization permissions seeded in database
- [ ] Organization CRUD operations logged in audit trail
- [ ] Resource queries properly scoped by organization_id
- [ ] Backward compatibility maintained (existing tests pass)
- [ ] Integration test suite passes with 80%+ coverage
- [ ] Linting passes (golangci-lint)
- [ ] Documentation complete (Swagger + AGENTS.md)

---

## Dependencies

### Internal Dependencies

- `internal/permission` - Casbin enforcer and cache
- `internal/domain` - Entity definitions
- `internal/repository` - Data access pattern
- `internal/service` - Business logic layer
- `internal/http` - HTTP handlers and middleware
- `internal/audit` - Audit logging

### External Dependencies

- Casbin (RBAC with domains)
- GORM (PostgreSQL)
- Echo (HTTP framework)
- UUID (primary keys)

---

## Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Performance degradation with org scoping | Medium | High | Add indexes, use Redis cache for membership |
| Breaking existing API consumers | Low | High | NULL organization_id for global, gradual rollout |
| Permission check latency | Medium | Medium | Redis cache with pub/sub invalidation |
| Data migration issues | Low | High | Tested rollback migration, backup before deploy |

---

## Timeline

**Total Effort**: 2-3 days

| Phase | Duration | Dependencies |
|-------|-----------|--------------|
| Phase 1: Database Schema | 4 hours | None |
| Phase 2: Domain Models | 2 hours | Phase 1 |
| Phase 3: Repository Layer | 2 hours | Phase 2 |
| Phase 4: Service Layer | 4 hours | Phase 3 |
| Phase 5: HTTP Handlers | 3 hours | Phase 4 |
| Phase 6: Middleware | 2 hours | Phase 5 |
| Phase 7: Permission Seeding | 1 hour | Phase 4 |
| Phase 8: Resource Scoping | 3 hours | Phase 7 |
| Phase 9: Integration Tests | 4 hours | All phases |
| Phase 10: Documentation | 2 hours | All phases |

---

## References

- [FEATURE_RECOMMENDATIONS.md](../../docs/FEATURE_RECOMMENDATIONS.md)
- [AGENTS.md](../../AGENTS.md)
- [IMPLEMENTATION_PLAN.md](../../IMPLEMENTATION_PLAN.md)
- [Casbin RBAC with Domains](https://casbin.org/docs/en/rbac-with-domains)