// Package middleware provides HTTP middleware components.
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	// IdempotencyKeyHeader is the HTTP header for idempotency keys.
	IdempotencyKeyHeader = "Idempotency-Key"
	// IdempotencyReplayedHeader indicates the response was replayed from cache.
	IdempotencyReplayedHeader = "X-Idempotency-Replayed"
	// IdempotencyKeyResponseHeader echoes back the idempotency key in responses.
	IdempotencyKeyResponseHeader = "X-Idempotency-Key"
	// RetryAfterHeader is the HTTP header for retry-after on 409 responses.
	RetryAfterHeader = "Retry-After"
	// IdempotencyKeyPrefix is the Redis key prefix for idempotency entries.
	IdempotencyKeyPrefix = "idem:"
	// IdempotencyLockSuffix is the Redis key suffix for lock entries.
	IdempotencyLockSuffix = ":lock"
	// IdempotencyRecordSuffix is the Redis key suffix for cached response entries.
	IdempotencyRecordSuffix = ":record"
	// defaultGuardTTLOverride is a fallback guard TTL in seconds if config is zero.
	defaultGuardTTLOverride = 300 // 5 minutes
	// defaultRecordTTLOverride is a fallback record TTL in seconds if config is zero.
	defaultRecordTTLOverride = 86400 // 24 hours
)

// IdempotencyMiddleware provides idempotency key processing for mutating endpoints.
// It intercepts requests with an Idempotency-Key header, checks Redis for prior
// completions, and either replays the cached response or processes the request while
// holding a distributed lock.
//
// Fail-open: all Redis errors are logged but do not block the request.
type IdempotencyMiddleware struct {
	cache   cache.Driver       // Cache driver for Redis operations (nil → fail-open)
	idemSvc *service.IdempotencyService
	config  config.IdempotencyConfig
	logger  *slog.Logger
}

// NewIdempotencyMiddleware creates a new idempotency middleware instance.
func NewIdempotencyMiddleware(cache cache.Driver, idemSvc *service.IdempotencyService, cfg config.IdempotencyConfig, logger *slog.Logger) *IdempotencyMiddleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &IdempotencyMiddleware{
		cache:   cache,
		idemSvc: idemSvc,
		config:  cfg,
		logger:  logger,
	}
}

// Middleware returns an echo.MiddlewareFunc that implements idempotency key handling.
func (im *IdempotencyMiddleware) Middleware() echo.MiddlewareFunc {
	keyPattern := regexp.MustCompile(im.config.KeyPattern)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			// 1. Skip conditions — pass through to next handler
			rawKey := c.Request().Header.Get(IdempotencyKeyHeader)
			if rawKey == "" {
				return next(c)
			}

			method := c.Request().Method
			if method == http.MethodGet || method == http.MethodDelete || method == http.MethodOptions || method == http.MethodHead {
				return next(c)
			}

			if !im.config.Enabled {
				return next(c)
			}

			if im.cache == nil {
				im.logger.Warn("idempotency middleware: cache driver nil, skipping")
				return next(c)
			}

			// 2. Key validation
			if !keyPattern.MatchString(rawKey) || len(rawKey) > im.config.MaxKeyLength {
				im.logger.Warn("idempotency key validation failed",
					slog.String("key", rawKey),
					slog.Int("key_length", len(rawKey)),
					slog.Int("max_length", im.config.MaxKeyLength),
				)
				return c.JSON(http.StatusBadRequest, response.ErrorWithContext(c, "INVALID_IDEMPOTENCY_KEY",
					fmt.Sprintf("Key must match pattern %s and be at most %d characters", im.config.KeyPattern, im.config.MaxKeyLength)))
			}

			// 3. User scoping
			userID, err := GetUserID(c)
			if err != nil {
				// No user_id in JWT context — skip (unauthenticated)
				return next(c)
			}

			orgID, hasOrgID := GetOrganizationID(c)
			orgSegment := "default"
			if hasOrgID && orgID != uuid.Nil {
				orgSegment = orgID.String()
			}

			key := fmt.Sprintf("%s%s:%s:%s:%s:%s", IdempotencyKeyPrefix, orgSegment, userID.String(), strings.ToLower(method), c.Request().URL.Path, rawKey)
			lockKey := key + IdempotencyLockSuffix
			recordKey := key + IdempotencyRecordSuffix

			// 4. Redis check flow (fail-open on all errors)
			lockVal, err := im.cache.Get(ctx, lockKey)
			if err != nil {
				im.logger.Warn("idempotency cache read error on lock key, fail-open",
					slog.String("key", lockKey),
					slog.String("error", err.Error()),
				)
				// Fail-open: continue to normal request processing
			} else if lockVal != nil {
				// Lock key exists — check status
				var lockData map[string]string
				if unmarshalErr := json.Unmarshal(lockVal, &lockData); unmarshalErr == nil {
					status := lockData["status"]
					if status == domain.IdempotencyStatusProcessing {
						// Request is still being processed
						c.Response().Header().Set(RetryAfterHeader, "5")
						return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "IDEMPOTENCY_PROCESSING",
							"A request with this idempotency key is currently being processed"))
					}
					if status == domain.IdempotencyStatusCompleted {
						// Check for cached response in Redis record key
						recVal, recErr := im.cache.Get(ctx, recordKey)
						if recErr != nil {
							im.logger.Warn("idempotency cache read error on record key, falling back to DB",
								slog.String("key", recordKey),
								slog.String("error", recErr.Error()),
							)
							// Fall back to DB lookup
						} else if recVal != nil {
							// Replay from Redis cache
							return im.replayFromRedisCache(c, recVal)
						}

						// Record key missing in Redis — fall back to DB
						return im.replayFromDB(c, rawKey, userID, method, c.Request().URL.Path)
					}
				}
				// If unmarshal fails, log and continue (fail-open)
				im.logger.Warn("idempotency lock value unmarshal error, fail-open",
					slog.String("key", lockKey),
					slog.String("error", "failed to unmarshal lock data"),
				)
			}

			// 5. SETNX lock acquisition
			bodyHash := im.computeBodyHash(c)
			guardTTLSeconds := int(im.config.GuardTTL.Seconds())
			if guardTTLSeconds <= 0 {
				guardTTLSeconds = defaultGuardTTLOverride
			}

			lockData := map[string]string{
				"status":      domain.IdempotencyStatusProcessing,
				"request_hash": bodyHash,
			}
			lockJSON, _ := json.Marshal(lockData)

			acquired, setErr := im.cache.SetNX(ctx, lockKey, lockJSON, guardTTLSeconds)
			if setErr != nil {
				im.logger.Warn("idempotency SetNX error, fail-open",
					slog.String("key", lockKey),
					slog.String("error", setErr.Error()),
				)
				// Fail-open: continue to process request
			} else if !acquired {
				// Another request acquired the lock first
				c.Response().Header().Set(RetryAfterHeader, "5")
				return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "IDEMPOTENCY_CONFLICT",
					"Another request with this idempotency key is being processed"))
			}

			// 6. responseWriter wrapper — capture status + body
			// Skip for WebSocket upgrades
			skipCapture := isWebSocketRequest(c.Request())
			var rw *idempotencyResponseWriter
			if !skipCapture {
				rw = &idempotencyResponseWriter{ResponseWriter: c.Response().Writer}
				c.Response().Writer = rw
			}

			// 7. Process the actual request
			handlerErr := next(c)

			// Get captured response
			statusCode := http.StatusOK
			var body []byte
			var capturedHeaders map[string]string
			if rw != nil {
				statusCode = rw.statusCode
				if statusCode == 0 {
					statusCode = http.StatusOK
				}
				body = rw.body.Bytes()
				capturedHeaders = rw.headers
			}

			// 8. Store result in Redis and DB asynchronously
			go func() {
				defer func() {
					if r := recover(); r != nil {
						im.logger.Error("idempotency storeResult panicked",
							slog.Any("panic", r),
							slog.String("key", rawKey),
						)
					}
				}()
				im.storeResult(
					rawKey, userID, hasOrgID, orgID, method, c.Request().URL.Path,
					bodyHash, body, statusCode, capturedHeaders, lockKey, recordKey,
				)
			}()

			// Set response headers
			c.Response().Header().Set(IdempotencyKeyResponseHeader, rawKey)
			c.Response().Header().Set(IdempotencyReplayedHeader, "false")

			return handlerErr
		}
	}
}

// computeBodyHash returns the SHA-256 hex digest of the request body.
// Returns empty string hash if no body or reading fails.
func (im *IdempotencyMiddleware) computeBodyHash(c echo.Context) string {
	if c.Request().Body == nil || c.Request().ContentLength == 0 {
		// No body — hash the empty string
		h := sha256.Sum256(nil)
		return hex.EncodeToString(h[:])
	}
	// Read body into buffer (up to maxCachedResponseSize + 1KB)
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(c.Request().Body); err != nil {
		im.logger.Warn("idempotency: failed to read request body for hashing",
			slog.String("error", err.Error()),
		)
		h := sha256.Sum256(nil)
		return hex.EncodeToString(h[:])
	}
	// Replace body so downstream handlers can read it
	c.Request().Body = io.NopCloser(&buf)
	h := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(h[:])
}

// replayFromRedisCache replays a cached response stored in Redis.
func (im *IdempotencyMiddleware) replayFromRedisCache(c echo.Context, recVal []byte) error {
	var cached cachedResponse
	if err := json.Unmarshal(recVal, &cached); err != nil {
		im.logger.Warn("idempotency: failed to unmarshal cached response from Redis, falling back to DB",
			slog.String("error", err.Error()),
		)
		// Get the user ID from JWT context for fallback lookup
		userID, _ := GetUserID(c)
		method := c.Request().Method
		path := c.Request().URL.Path
		return im.replayFromDB(c, c.Request().Header.Get(IdempotencyKeyHeader), userID, method, path)
	}

	// Write cached response
	for k, v := range cached.Headers {
		c.Response().Header().Set(k, v)
	}
	c.Response().WriteHeader(cached.StatusCode)
	if len(cached.Body) > 0 {
		_, _ = c.Response().Write(cached.Body)
	}

	c.Response().Header().Set(IdempotencyKeyResponseHeader, c.Request().Header.Get(IdempotencyKeyHeader))
	c.Response().Header().Set(IdempotencyReplayedHeader, "true")
	return nil
}

// replayFromDB looks up the record in PostgreSQL via the service and replays the response.
func (im *IdempotencyMiddleware) replayFromDB(c echo.Context, key string, userID uuid.UUID, method, path string) error {
	ctx := c.Request().Context()

	record, err := im.idemSvc.FindRecord(ctx, key, userID, method, path)
	if err != nil {
		im.logger.Warn("idempotency: DB lookup failed, proceeding with request",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		// Fail-open: if we can't find the record, we can't replay. Return error to trigger retry.
		return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "IDEMPOTENCY_CONFLICT",
			"Unable to retrieve idempotency record"))
	}

	if record == nil || !record.IsCompleted() {
		// Record not found or still processing
		c.Response().Header().Set(RetryAfterHeader, "5")
		return c.JSON(http.StatusConflict, response.ErrorWithContext(c, "IDEMPOTENCY_PROCESSING",
			"A request with this idempotency key is currently being processed"))
	}

	// Replay the stored response
	if record.ResponseHeaders != nil {
		for k, v := range record.ResponseHeaders {
			c.Response().Header().Set(k, v)
		}
	}
	c.Response().WriteHeader(record.ResponseStatusCode)
	if record.ResponseBody != "" {
		_, _ = c.Response().Write([]byte(record.ResponseBody))
	}

	c.Response().Header().Set(IdempotencyKeyResponseHeader, key)
	c.Response().Header().Set(IdempotencyReplayedHeader, "true")
	return nil
}

// storeResult records the request result in both Redis and the database asynchronously.
func (im *IdempotencyMiddleware) storeResult(
	key string, userID uuid.UUID, hasOrgID bool, orgID uuid.UUID,
	method, path, bodyHash string, body []byte, statusCode int,
	capturedHeaders map[string]string, lockKey, recordKey string,
) {
	ctx := contextWithoutDeadline()

	// Store completed lock in Redis
	lockData := map[string]string{
		"status": domain.IdempotencyStatusCompleted,
	}
	lockJSON, _ := json.Marshal(lockData)

	guardTTLSeconds := int(im.config.GuardTTL.Seconds())
	if guardTTLSeconds <= 0 {
		guardTTLSeconds = defaultGuardTTLOverride
	}

	if err := im.cache.Set(ctx, lockKey, lockJSON, guardTTLSeconds); err != nil {
		im.logger.Warn("idempotency: failed to update lock key in Redis",
			slog.String("key", lockKey),
			slog.String("error", err.Error()),
		)
	}

	// Store response body in Redis if under size threshold
	maxSize := im.config.MaxCachedResponseSize
	if maxSize <= 0 {
		maxSize = config.DefaultIdempotencyConfig().MaxCachedResponseSize
	}

	headers := capturedHeaders
	if headers == nil {
		headers = make(map[string]string)
	}

	if len(body) <= maxSize {
		// Store full response in Redis
		cached := cachedResponse{
			StatusCode: statusCode,
			Body:       body,
			Headers:    headers,
		}
		cachedJSON, _ := json.Marshal(cached)
		recordTTLSeconds := int(im.config.DefaultTTL.Seconds())
		if recordTTLSeconds <= 0 {
			recordTTLSeconds = defaultRecordTTLOverride
		}
		if err := im.cache.Set(ctx, recordKey, cachedJSON, recordTTLSeconds); err != nil {
			im.logger.Warn("idempotency: failed to store record key in Redis",
				slog.String("key", recordKey),
				slog.String("error", err.Error()),
			)
		}
	}

	// Store in DB asynchronously
	orgIDPtr := &orgID
	if !hasOrgID || orgID == uuid.Nil {
		orgIDPtr = nil
	}

	record := &domain.IdempotencyRecord{
		IdempotencyKey:     key,
		UserID:             userID,
		OrganizationID:     orgIDPtr,
		HTTPMethod:         method,
		RequestPath:        path,
		RequestHash:        bodyHash,
		Status:             domain.IdempotencyStatusProcessing,
		ResponseStatusCode: statusCode,
		ResponseBody:       string(body),
		ResponseBodySize:    len(body),
		ResponseHeaders:    domain.MapStringString(headers),
		ExpiresAt:          time.Now().Add(im.config.DefaultTTL),
	}

	// Create record as processing first
	if createErr := im.idemSvc.CreateRecord(ctx, record); createErr != nil {
		im.logger.Warn("idempotency: failed to create DB record",
			slog.String("key", key),
			slog.String("error", createErr.Error()),
		)
		return
	}

	// Update record to completed
	bodyStr := ""
	if len(body) <= maxSize {
		bodyStr = string(body)
	}
	if updateErr := im.idemSvc.UpdateRecord(ctx, record.ID, domain.IdempotencyStatusCompleted, statusCode, bodyStr, headers); updateErr != nil {
		im.logger.Warn("idempotency: failed to update DB record status",
			slog.String("key", key),
			slog.String("record_id", record.ID.String()),
			slog.String("error", updateErr.Error()),
		)
	}
}

// cachedResponse represents a cached HTTP response stored in Redis.
type cachedResponse struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// idempotencyResponseWriter wraps http.ResponseWriter to capture the status code, response body,
// and relevant response headers for idempotency replay.
type idempotencyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	headers    map[string]string
	maxSize    int
	wrote      bool
}

// Headers that should be captured for response replay fidelity.
var replayHeaders = map[string]bool{
	"Content-Type":     true,
	"Content-Encoding": true,
	"Content-Length":   true,
	"Location":         true,
	"X-Request-Id":     true,
	"Retry-After":      true,
}

// WriteHeader captures the status code, records relevant response headers,
// and delegates to the underlying writer.
func (w *idempotencyResponseWriter) WriteHeader(code int) {
	if !w.wrote {
		w.statusCode = code
		w.wrote = true
		// Capture relevant headers for replay before they're sent
		if w.headers == nil {
			w.headers = make(map[string]string)
		}
		for k, v := range w.ResponseWriter.Header() {
			if replayHeaders[k] {
				if len(v) > 0 {
					w.headers[k] = v[0]
				}
			}
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the response body (up to maxCachedResponseSize + 1KB overhead) and delegates.
// Also captures response headers if WriteHeader was not explicitly called.
func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.statusCode = http.StatusOK
		w.wrote = true
		// Capture headers on implicit WriteHeader call
		if w.headers == nil {
			w.headers = make(map[string]string)
		}
		for k, v := range w.ResponseWriter.Header() {
			if replayHeaders[k] {
				if len(v) > 0 {
					w.headers[k] = v[0]
				}
			}
		}
	}
	// Capture body up to maximum size
	maxCapture := w.maxSize
	if maxCapture <= 0 {
		maxCapture = config.DefaultIdempotencyConfig().MaxCachedResponseSize + 1024
	}
	if w.body.Len()+len(b) <= maxCapture {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap returns the underlying http.ResponseWriter for http.ResponseController.
func (w *idempotencyResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// isWebSocketRequest checks if the request is a WebSocket upgrade.
func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// contextWithoutDeadline returns a context with no deadline for background goroutines.
// This prevents context cancellation from affecting async DB operations.
func contextWithoutDeadline() context.Context {
	return context.Background()
}