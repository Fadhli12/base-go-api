# Tasks: Dynamic Configuration System

**Input**: Design documents from `specs/go-api-base/`
**Prerequisites**: plan.md (required), research.md, contracts/env-vars.md
**Branch**: `001-dynamic-config`

**Tests**: Unit and integration tests per phase as specified in plan.md QA scenarios.

**Organization**: Tasks are grouped by implementation phase from plan.md.

## Format: `[ID] [P?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions

- Backend API: `cmd/`, `internal/`, `migrations/`
- Tests: `tests/unit/`, `tests/integration/`
- Config: `.env.example`, `internal/config/`

---

## Phase 1: Config Struct Extensions

**Goal**: Extend configuration structs to support storage, image, cache, and swagger settings.

**Independent Test**: Configuration loads from environment variables with validation.

### Implementation

- [x] T001 [P] Extend `StorageConfig` struct in `internal/config/storage.go` with S3 fields
- [x] T002 [P] Create `ImageConfig` struct in `internal/config/image.go` with compression settings
- [x] T003 [P] Create `SwaggerConfig` struct definition in `internal/config/swagger.go` with Enabled/Path fields
- [x] T004 [P] Create `CacheConfig` struct in `internal/config/cache.go` with driver/TTL settings
- [x] T005 Update top-level `Config` struct in `internal/config/config.go` to include new configs
- [x] T006 Add parser functions for each new config section in `internal/config/config.go`
- [x] T007 Create `internal/config/validate.go` with `Validate()` function and section validators
- [x] T007.5 Wire `Validate()` call into `loadConfig()` in `internal/config/config.go`

### Unit Tests

- [x] T008 [P] Create `tests/unit/config/storage_test.go` with `TestDefaultStorageConfig`, `TestS3ConfigParsing`, `TestInvalidStorageDriver`, `TestMissingS3Bucket`
- [x] T009 [P] Create `tests/unit/config/image_test.go` with `TestImageConfigParsing`, `TestInvalidQuality`, `TestInvalidDimensions`
- [x] T010 [P] Create `tests/unit/config/cache_test.go` with `TestCacheConfigParsing`, `TestInvalidCacheDriver`
- [x] T011 [P] Create `tests/unit/config/swagger_test.go` with `TestSwaggerConfigDefaults` (config-level validation)

### Integration

- [x] T012 Update `.env.example` with all new environment variables from `contracts/env-vars.md`
- [x] T013 Run `go test ./tests/unit/config/...` and verify all config tests pass

**Checkpoint**: Config loads and validates - can proceed to storage driver implementation

---

## Phase 2: S3/MinIO Storage Driver

**Goal**: Implement unified S3 driver for AWS S3 and MinIO storage backends.

**Independent Test**: Files can be uploaded/downloaded to S3/MinIO via configurable driver.

### Implementation

- [ ] T014 Add AWS SDK v2 dependencies to `go.mod` (aws-sdk-go-v2, config, service/s3, credentials)
- [ ] T015 Create `internal/storage/s3.go` with `S3Driver` struct implementing `storage.Driver` interface
- [ ] T016 Implement `NewS3Driver()` factory function with endpoint/path-style configuration
- [ ] T017 Implement `Store()` method for S3 upload with presigned URL support
- [ ] T018 Implement `Get()` method for S3 download
- [ ] T019 Implement `Delete()` method for S3 object removal
- [ ] T020 Implement `URL()` method for presigned URL generation
- [ ] T021 Implement `Exists()` and `Size()` methods for S3 metadata
- [ ] T022 Update `NewDriver()` factory in `internal/storage/storage.go` to handle "s3" and "minio" drivers

### Unit Tests

- [ ] T023 [P] Create `tests/unit/storage/s3_test.go` with `TestPresignedURL`, `TestS3FileNotFound`, `TestS3InvalidCredentials`

### Integration Tests

- [ ] T024 Create `tests/integration/storage_test.go` with LocalStack container setup
- [ ] T025 Add test case for AWS S3 upload/download with LocalStack
- [ ] T026 Add test case for MinIO upload/download with MinIO container
- [ ] T027 Add test case for invalid S3 credentials error handling

**Checkpoint**: S3/MinIO storage driver functional - can proceed to image configuration

---

## Phase 3: Image Compression Configuration

**Goal**: Replace hardcoded image compression constants with configurable values.

**Independent Test**: Image uploads use configurable quality/dimensions from environment.

### Implementation

- [ ] T028 Create `internal/conversion/config.go` with `Config` struct and `DefaultConfig()` function
- [ ] T029 Add `ThumbnailDef()` and `PreviewDef()` methods to `Config` struct
- [ ] T030 Add `GetDefaultConversions()` method using config values
- [ ] T031 Update `internal/conversion/handler.go` to accept `Config` parameter in constructor
- [ ] T032 Modify `CreateConversionJobs()` to use injected config instead of constants
- [ ] T033 Update `cmd/api/main.go` to create `conversion.Config` from `ImageConfig` and inject

### Unit Tests

- [ ] T034 [P] Create `tests/unit/conversion/config_test.go` with `TestDefaultConversionConfig`, `TestCustomThumbnailQuality`, `TestCompressionDisabled`

### Integration Tests

- [ ] T035 Create `tests/integration/conversion_test.go` with `TestImageUploadWithConfig`

**Checkpoint**: Image compression configurable - can proceed to swagger toggle

---

## Phase 4: Swagger Toggle

**Goal**: Allow Swagger documentation to be enabled/disabled via environment variable.

**Independent Test**: Swagger routes return 404 when disabled, content when enabled.

### Implementation

- [ ] T036 Add swagger config parsing in `internal/config/swagger.go`
- [ ] T037 Update `internal/http/server.go` `RegisterRoutes()` to conditionally register swagger routes based on `config.Swagger.Enabled`

### Unit Tests

- [ ] T038 [P] Create `tests/unit/http/swagger_test.go` with `TestSwaggerPathValidation` (HTTP routing validation)

### Integration Tests

- [ ] T039 Create `tests/integration/swagger_test.go` with `TestSwaggerEnabled`, `TestSwaggerDisabled`, `TestCustomSwaggerPath`

**Checkpoint**: Swagger toggle functional - can proceed to cache driver

---

## Phase 5: Cache Driver Configuration

**Goal**: Implement cache driver abstraction supporting Redis, memory, and none backends.

**Independent Test**: Cache operations work with Redis, memory fallback, and no-op modes.

### Implementation

- [ ] T040 Create `internal/cache/cache.go` with `Driver` interface, `Config` struct, and `NewDriver()` factory
- [ ] T041 [P] Create `internal/cache/redis.go` with `RedisCache` implementing `Driver`
- [ ] T043 [P] Create `internal/cache/memory.go` with `MemoryCache` implementing `Driver`
- [ ] T044 [P] Create `internal/cache/noop.go` with `NoopCache` implementing `Driver`
- [x] T045 Update `internal/permission/cache.go` to use `cache.Driver` interface instead of direct Redis
- [x] T046 Update `internal/http/middleware/rate_limit.go` to use `cache.Driver` interface
- [x] T047 Update `cmd/api/main.go` to initialize cache driver from `CacheConfig`

### Unit Tests

- [ ] T048 [P] Create `tests/unit/cache/memory_test.go` with `TestMemoryCache`
- [ ] T049 [P] Create `tests/unit/cache/noop_test.go` with `TestNoopCache`
- [ ] T050 [P] Create `tests/unit/cache/driver_test.go` with `TestInvalidCacheDriver` validation

### Integration Tests

- [ ] T051 Create `tests/integration/cache_test.go` with Redis container setup
- [ ] T052 Add test case for `CACHE_DRIVER=redis`
- [ ] T053 Add test case for `CACHE_DRIVER=memory`
- [ ] T054 Add test case for `CACHE_DRIVER=none`
- [ ] T055 Add test case for permission cache TTL expiration
- [ ] T056 Add test case for permission cache invalidation

**Checkpoint**: Cache driver abstraction complete - can proceed to final phase

---

## Phase 6: Testing & Documentation

**Goal**: Ensure comprehensive test coverage and update documentation.

**Independent Test**: All tests pass, documentation reflects new configuration.

### Tests

- [ ] T057 [P] Create `tests/integration/config_test.go` with `TestFullConfigIntegration`, `TestBackwardCompatibility`, `TestConfigDefaults`
- [ ] T058 Run `make test` to verify all unit tests pass
- [ ] T059 Run `make test-integration` to verify all integration tests pass (requires Docker)

### Documentation

- [ ] T060 [P] Update `README.md` with new environment variables section
- [ ] T061 [P] Verify `.env.example` contains all variables from `contracts/env-vars.md`
- [ ] T062 Create `docs/configuration.md` explaining each config section with examples
- [ ] T063 Update `AGENTS.md` with note about dynamic configuration feature

### Lint & Quality

- [ ] T064 Run `make lint` and fix any issues

**Checkpoint**: All tests pass, documentation complete

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Config Structs)**: No dependencies - can start immediately
- **Phase 2 (S3 Driver)**: Depends on Phase 1 config struct
- **Phase 3 (Image Config)**: Depends on Phase 1 config struct
- **Phase 4 (Swagger)**: Depends on Phase 1 config struct
- **Phase 5 (Cache Driver)**: Depends on Phase 1 config struct
- **Phase 6 (Testing/Docs)**: Depends on Phases 1-5 completion

### Parallel Opportunities

Within each phase, tasks marked [P] can run in parallel:
- T001, T002, T003, T004 (config struct files)
- T008, T009, T010, T011 (unit test files)
- T042, T043, T044 (cache driver implementations)
- T048, T049, T050 (cache unit tests)
- T057 (integration tests)
- T060, T061 (documentation files)

### Minimum Executable Increment (MVP)

1. Complete Phase 1: Config Struct Extensions
2. **STOP and VALIDATE**: Run unit tests, verify config loads
3. Continue with Phase 2-5 based on priority:
   - Phase 5 (Cache) - most critical for performance
   - Phase 2 (S3) - important for production deployments
   - Phase 3 (Image) - useful for media handling
   - Phase 4 (Swagger) - useful for documentation control
4. Complete Phase 6: Testing & Documentation

---

## Parallel Example: Phase 1

```bash
# Launch all config struct files together:
Task: "Extend StorageConfig struct in internal/config/storage.go"
Task: "Create ImageConfig struct in internal/config/image.go"
Task: "Create SwaggerConfig struct in internal/config/swagger.go"
Task: "Create CacheConfig struct in internal/config/cache.go"

# After T005-T006 complete, launch all unit test files together:
Task: "Create tests/unit/config/storage_test.go"
Task: "Create tests/unit/config/image_test.go"
Task: "Create tests/unit/config/cache_test.go"
Task: "Create tests/unit/config/swagger_test.go"
```

---

## Implementation Strategy

### Sequential (Team Size: 1)

1. Phase 1 → validate → Phase 2 → Phase 5 → Phase 3 → Phase 4 → Phase 6
2. Each phase validated by tests before proceeding
3. Cache driver prioritized after S3 for performance

### MVP First

1. Complete Phase 1 (Config Structs)
2. **Deploy/Demo**: Environment variables work, config validates
3. Complete Phase 5 (Cache Driver) - critical infrastructure
4. **Deploy/Demo**: Can switch cache backends without code changes
5. Complete Phase 2 (S3 Driver)
6. **Deploy/Demo**: Can switch storage backends
7. Continue with remaining phases

---

## Notes

- [P] tasks operate on different files with no shared dependencies
- Tests should FAIL before implementation (TDD where specified)
- Each phase has clear checkpoint criteria
- All new environment variables documented in contracts/env-vars.md
- AWS SDK v2 adds ~15MB to binary size (acceptable for production)