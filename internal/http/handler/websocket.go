package handler

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/http/middleware"
	"github.com/example/go-api-base/internal/http/response"
	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/internal/permission"
	"github.com/example/go-api-base/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// WebSocketHandler handles WebSocket upgrade and presence REST endpoints.
type WebSocketHandler struct {
	hub      *service.Hub
	presence service.PresenceService
	enforcer *permission.Enforcer
	auditSvc *service.AuditService
	logger   logger.Logger
	config   config.WsConfig
}

// NewWebSocketHandler creates a new WebSocketHandler instance.
func NewWebSocketHandler(
	hub *service.Hub,
	presence service.PresenceService,
	enforcer *permission.Enforcer,
	auditSvc *service.AuditService,
	log logger.Logger,
	config config.WsConfig,
) *WebSocketHandler {
	return &WebSocketHandler{
		hub:      hub,
		presence: presence,
		enforcer: enforcer,
		auditSvc: auditSvc,
		logger:   log,
		config:   config,
	}
}

// HandleUpgrade upgrades an HTTP connection to WebSocket and registers the client with the hub.
func (h *WebSocketHandler) HandleUpgrade(c echo.Context) error {
	ctx := c.Request().Context()

	userID := middleware.GetWSUserID(c)
	if userID == uuid.Nil {
		h.logger.Warn(ctx, "websocket upgrade failed - no user ID")
		return response.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	orgID := middleware.GetWSOrgID(c)

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		OriginPatterns: h.config.AllowedOrigins,
	})
	if err != nil {
		h.logger.Error(ctx, "websocket upgrade failed", logger.Err(err))
		return err
	}

	conn.SetReadLimit(h.config.MaxMessageSize)

	client := service.NewClient(conn, userID, orgID, h.hub, h.logger, h.config)
	h.hub.Register(client)

	client.Run(c.Request().Context())

	h.hub.Unregister(client)

	return nil
}

// GetPresence returns the list of online users for a given organization.
func (h *WebSocketHandler) GetPresence(c echo.Context) error {
	ctx := c.Request().Context()

	orgIDStr := c.Param("orgID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return response.ErrorResponse(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid organization ID")
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		h.logger.Warn(ctx, "get presence failed - not authenticated")
		return response.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	orgIDFromCtx, hasOrgID := middleware.GetOrganizationID(c)
	orgDomain := "default"
	if hasOrgID && orgIDFromCtx != uuid.Nil {
		orgDomain = orgIDFromCtx.String()
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "websocket", "view_presence")
	if err != nil || !allowed {
		h.logger.Warn(ctx, "get presence denied",
			logger.String("user_id", userID.String()),
			logger.String("org_domain", orgDomain),
		)
		return response.ErrorResponse(c, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	}

	onlineUsers, err := h.presence.GetOnlineUsers(ctx, orgID)
	if err != nil {
		h.logger.Error(ctx, "failed to get online users", logger.Err(err))
		return response.ErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve presence data")
	}

	_ = h.auditSvc.LogAction(ctx, userID, "view_presence", "presence", orgID.String(), nil, nil, "", "")

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"org_id":       orgID.String(),
		"online_users": onlineUsers,
		"online_count": len(onlineUsers),
	}))
}

// GetRooms returns the list of WebSocket rooms accessible to the authenticated user.
func (h *WebSocketHandler) GetRooms(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := middleware.GetUserID(c)
	if err != nil {
		h.logger.Warn(ctx, "get rooms failed - not authenticated")
		return response.ErrorResponse(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not authenticated")
	}

	orgID, hasOrgID := middleware.GetOrganizationID(c)

	orgDomain := "default"
	if hasOrgID && orgID != uuid.Nil {
		orgDomain = orgID.String()
	}

	allowed, err := h.enforcer.Enforce(userID.String(), orgDomain, "websocket", "view_rooms")
	if err != nil || !allowed {
		h.logger.Warn(ctx, "get rooms denied",
			logger.String("user_id", userID.String()),
			logger.String("org_domain", orgDomain),
		)
		return response.ErrorResponse(c, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
	}

	rooms := []string{}
	if hasOrgID && orgID != uuid.Nil {
		rooms = append(rooms, "org:"+orgID.String())
	}

	_ = h.auditSvc.LogAction(ctx, userID, "view_rooms", "rooms", userID.String(), nil, nil, "", "")

	return c.JSON(http.StatusOK, response.SuccessWithContext(c, map[string]interface{}{
		"user_id": userID.String(),
		"rooms":   rooms,
	}))
}

// RegisterRoutes registers WebSocket and presence REST routes with the Echo group.
func (h *WebSocketHandler) RegisterRoutes(g *echo.Group, jwtSecret string) {
	g.GET("/ws", h.HandleUpgrade, middleware.WebSocketAuth(jwtSecret, h.enforcer))
	g.GET("/presence/org/:orgID", h.GetPresence, middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))
	g.GET("/ws/rooms", h.GetRooms, middleware.JWT(middleware.JWTConfig{Secret: jwtSecret, ContextKey: "user"}))
}