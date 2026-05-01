//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/cache"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

// notificationTestSvc builds a NotificationService wired to the test suite DB/cache.
func notificationTestSvc(t *testing.T, suite *TestSuite) *service.NotificationService {
	t.Helper()

	notifRepo := repository.NewNotificationRepository(suite.DB)
	prefRepo := repository.NewNotificationPreferenceRepository(suite.DB)
	userRepo := repository.NewUserRepository(suite.DB)

	cacheDriver, err := cache.NewDriver(cache.Config{
		Driver:        "redis",
		DefaultTTL:    300,
		PermissionTTL: 300,
		RateLimitTTL:  60,
	}, suite.RedisClient)
	require.NoError(t, err, "cache driver must initialise")

	return service.NewNotificationService(
		notifRepo,
		prefRepo,
		nil, // emailService not needed in handler tests
		userRepo,
		cacheDriver,
		slog.Default(),
	)
}

// sendNotification is a helper that creates a notification via the service and fails the test on error.
func sendNotification(t *testing.T, svc *service.NotificationService, userID uuid.UUID, notifType, title, message string) {
	t.Helper()
	err := svc.Send(context.Background(), userID, notifType, title, message, "")
	require.NoError(t, err, "Send notification must not fail")
}

// getUserIDFromToken parses the /api/v1/me endpoint to get the authenticated user's UUID.
func getUserIDFromToken(t *testing.T, server *helpers.Server, token string) uuid.UUID {
	t.Helper()
	rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/me", nil, token)
	require.Equal(t, http.StatusOK, rec.Code, "/api/v1/me should return 200")
	env := helpers.ParseEnvelope(t, rec)
	data, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "me response data should be a map")
	idStr, ok := data["id"].(string)
	require.True(t, ok, "user id should be a string")
	id, err := uuid.Parse(idStr)
	require.NoError(t, err, "user id should be a valid UUID")
	return id
}

// TestNotificationHandler_List tests pagination and filters on GET /api/v1/notifications.
func TestNotificationHandler_List(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("pagination limit and offset", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 25 notifications
		for i := 0; i < 25; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("Title %d", i), "msg")
		}

		// First page: limit=10, offset=0
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications?limit=10&offset=0", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		total := data["total"].(float64)
		items := data["data"].([]interface{})
		assert.Equal(t, float64(25), total, "total should be 25")
		assert.Len(t, items, 10, "should return 10 items")

		// Second page: limit=10, offset=10
		rec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications?limit=10&offset=10", nil, token)
		assert.Equal(t, http.StatusOK, rec2.Code)
		env2 := helpers.ParseEnvelope(t, rec2)
		data2 := env2.Data.(map[string]interface{})
		items2 := data2["data"].([]interface{})
		assert.Len(t, items2, 10, "second page should return 10 items")

		// Last page: limit=10, offset=20
		rec3 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications?limit=10&offset=20", nil, token)
		assert.Equal(t, http.StatusOK, rec3.Code)
		env3 := helpers.ParseEnvelope(t, rec3)
		data3 := env3.Data.(map[string]interface{})
		items3 := data3["data"].([]interface{})
		assert.Len(t, items3, 5, "last page should return 5 items")
	})

	t.Run("unread_only filter", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 3 unread notifications
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("Unread %d", i), "msg")
		}

		// Create 2 notifications, then mark them as read
		for i := 0; i < 2; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("Read %d", i), "msg")
		}

		// Mark all as read first, then re-create 3 unread
		_, err := svc.MarkAllAsRead(context.Background(), userID, nil)
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userID, "mention", fmt.Sprintf("Unread After %d", i), "msg")
		}

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications?unread_only=true", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		total := data["total"].(float64)
		assert.Equal(t, float64(3), total, "unread_only should return only 3 unread notifications")
	})

	t.Run("archived notifications are hidden", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 3 normal + 1 to-be-archived
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("Normal %d", i), "msg")
		}
		sendNotification(t, svc, userID, "system", "To Archive", "msg")

		// Get the notification list to find the one to archive
		notifications, _, err := svc.ListByUser(context.Background(), userID, 10, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(notifications), 1)

		// Archive the last notification
		toArchive := notifications[0] // newest first
		err = svc.ArchiveNotification(context.Background(), toArchive.ID, userID)
		require.NoError(t, err)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		total := data["total"].(float64)
		assert.Equal(t, float64(3), total, "archived notification should be hidden from list")
	})
}

// TestNotificationHandler_CountUnread tests GET /api/v1/notifications/unread-count.
func TestNotificationHandler_CountUnread(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("counts only unread notifications", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 5 notifications, mark 2 as read
		for i := 0; i < 5; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("Notif %d", i), "msg")
		}

		notifications, _, err := svc.ListByUser(context.Background(), userID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifications, 5)

		// Mark 2 as read
		err = svc.MarkAsRead(context.Background(), notifications[0].ID, userID)
		require.NoError(t, err)
		err = svc.MarkAsRead(context.Background(), notifications[1].ID, userID)
		require.NoError(t, err)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/unread-count", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)
		data := env.Data.(map[string]interface{})
		unreadCount := data["unread_count"].(float64)
		assert.Equal(t, float64(3), unreadCount, "unread_count should be 3")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/unread-count", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestNotificationHandler_MarkAsRead tests PATCH /api/v1/notifications/:id/read.
func TestNotificationHandler_MarkAsRead(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("marks notification as read and sets read_at", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		sendNotification(t, svc, userID, "system", "Test Notif", "msg")

		notifications, _, err := svc.ListByUser(context.Background(), userID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifications, 1)
		notifID := notifications[0].ID

		rec := helpers.MakeRequest(t, server, http.MethodPatch, "/api/v1/notifications/"+notifID.String()+"/read", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify read_at is now set via unread count
		count, err := svc.CountUnread(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "unread count should be 0 after marking as read")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodPatch, "/api/v1/notifications/"+fakeID+"/read", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("user cannot mark another user's notification as read (404)", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userAID := getUserIDFromToken(t, server, tokenA)
		svc := notificationTestSvc(t, suite)

		sendNotification(t, svc, userAID, "system", "User A Notif", "msg")
		notifications, _, err := svc.ListByUser(context.Background(), userAID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifications, 1)
		notifID := notifications[0].ID

		// User B tries to mark User A's notification as read
		rec := helpers.MakeRequest(t, server, http.MethodPatch, "/api/v1/notifications/"+notifID.String()+"/read", nil, tokenB)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestNotificationHandler_MarkAllAsRead tests POST /api/v1/notifications/read-all.
func TestNotificationHandler_MarkAllAsRead(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("marks all unread notifications as read", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 5 system + 3 assignment notifications
		for i := 0; i < 5; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("System %d", i), "msg")
		}
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userID, "assignment", fmt.Sprintf("Assignment %d", i), "msg")
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/notifications/read-all", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		markedCount := data["marked_count"].(float64)
		assert.Equal(t, float64(8), markedCount, "marked_count should be 8")
	})

	t.Run("marks only specified type when type filter is provided", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 3 system + 2 assignment notifications
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userID, "system", fmt.Sprintf("System %d", i), "msg")
		}
		for i := 0; i < 2; i++ {
			sendNotification(t, svc, userID, "assignment", fmt.Sprintf("Assignment %d", i), "msg")
		}

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/notifications/read-all?type=assignment", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		markedCount := data["marked_count"].(float64)
		assert.Equal(t, float64(2), markedCount, "only assignment notifications should be marked read")

		// System notifications should still be unread
		count, err := svc.CountUnread(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, int64(3), count, "3 system notifications should still be unread")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/notifications/read-all", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestNotificationHandler_Archive tests DELETE /api/v1/notifications/:id.
func TestNotificationHandler_Archive(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("archives notification and hides from list", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Create 2 notifications, archive one
		sendNotification(t, svc, userID, "system", "Keep This", "msg")
		sendNotification(t, svc, userID, "system", "Archive This", "msg")

		notifications, _, err := svc.ListByUser(context.Background(), userID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifications, 2)

		toArchive := notifications[0] // newest first
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/notifications/"+toArchive.ID.String(), nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, "Notification archived", data["message"])

		// Verify it no longer appears in list
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)
		listEnv := helpers.ParseEnvelope(t, listRec)
		listData := listEnv.Data.(map[string]interface{})
		total := listData["total"].(float64)
		assert.Equal(t, float64(1), total, "archived notification should not appear in list")
	})

	t.Run("user cannot archive another user's notification (404)", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userAID := getUserIDFromToken(t, server, tokenA)
		svc := notificationTestSvc(t, suite)

		sendNotification(t, svc, userAID, "system", "User A Notif", "msg")
		notifications, _, err := svc.ListByUser(context.Background(), userAID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifications, 1)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/notifications/"+notifications[0].ID.String(), nil, tokenB)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/notifications/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestNotificationHandler_GetPreferences tests GET /api/v1/notifications/preferences.
func TestNotificationHandler_GetPreferences(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("returns empty array when no preferences exist", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/preferences", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)
		prefs, ok := env.Data.([]interface{})
		require.True(t, ok, "preferences data should be an array")
		assert.Empty(t, prefs, "preferences should be empty for new user")
	})

	t.Run("returns preferences after they are created", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		// Create a preference via PUT — use email_enabled=false to test non-default bool value
		putBody := map[string]interface{}{
			"notification_type": "system",
			"email_enabled":     false,
			"push_enabled":      true,
		}
		putRec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", putBody, token)
		require.Equal(t, http.StatusOK, putRec.Code)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/preferences", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		prefs, ok := env.Data.([]interface{})
		require.True(t, ok, "preferences should be an array")
		require.Len(t, prefs, 1, "should have one preference")

		pref := prefs[0].(map[string]interface{})
		assert.Equal(t, "system", pref["notification_type"])
		assert.Equal(t, false, pref["email_enabled"])
		assert.Equal(t, true, pref["push_enabled"])
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/preferences", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestNotificationHandler_UpdatePreference tests PUT /api/v1/notifications/preferences.
func TestNotificationHandler_UpdatePreference(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("creates preference and persists it", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		// Use email_enabled=false to verify non-default bool value is persisted
		body := map[string]interface{}{
			"notification_type": "mention",
			"email_enabled":     false,
			"push_enabled":      true,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", body, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, "Notification preference updated", data["message"])

		// Verify via GET
		getRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/preferences", nil, token)
		assert.Equal(t, http.StatusOK, getRec.Code)
		getEnv := helpers.ParseEnvelope(t, getRec)
		prefs := getEnv.Data.([]interface{})
		require.Len(t, prefs, 1)
		pref := prefs[0].(map[string]interface{})
		assert.Equal(t, "mention", pref["notification_type"])
		assert.Equal(t, false, pref["email_enabled"])
		assert.Equal(t, true, pref["push_enabled"])
	})

	t.Run("updates existing preference", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		// Create initial preference
		initial := map[string]interface{}{
			"notification_type": "assignment",
			"email_enabled":     true,
			"push_enabled":      true,
		}
		putRec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", initial, token)
		require.Equal(t, http.StatusOK, putRec.Code)

		// Update: set email_enabled=false to verify ON CONFLICT DO UPDATE sets non-default bool
		updated := map[string]interface{}{
			"notification_type": "assignment",
			"email_enabled":     false,
			"push_enabled":      true,
		}
		putRec2 := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", updated, token)
		assert.Equal(t, http.StatusOK, putRec2.Code)

		// Verify update — only one record should exist, email_enabled should now be false
		getRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications/preferences", nil, token)
		getEnv := helpers.ParseEnvelope(t, getRec)
		prefs := getEnv.Data.([]interface{})
		require.Len(t, prefs, 1, "should still have only one preference for this type")
		pref := prefs[0].(map[string]interface{})
		assert.Equal(t, false, pref["email_enabled"])
		assert.Equal(t, true, pref["push_enabled"])
	})

	t.Run("validation error for invalid type", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		body := map[string]interface{}{
			"notification_type": "invalid_type",
			"email_enabled":     true,
			"push_enabled":      false,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", body, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error)
		assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		body := map[string]interface{}{
			"notification_type": "system",
			"email_enabled":     true,
			"push_enabled":      false,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/notifications/preferences", body, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestNotificationHandler_Unauthorized tests that all notification endpoints return 401 without a JWT token.
func TestNotificationHandler_Unauthorized(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	fakeID := uuid.New().String()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/notifications/unread-count"},
		{http.MethodPatch, "/api/v1/notifications/" + fakeID + "/read"},
		{http.MethodPost, "/api/v1/notifications/read-all"},
		{http.MethodDelete, "/api/v1/notifications/" + fakeID},
		{http.MethodGet, "/api/v1/notifications/preferences"},
		{http.MethodPut, "/api/v1/notifications/preferences"},
	}

	for _, ep := range endpoints {
		ep := ep // capture loop variable
		t.Run(ep.method+" "+ep.path+" returns 401", func(t *testing.T) {
			rec := helpers.MakeRequest(t, server, ep.method, ep.path, nil, "")
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"%s %s should return 401 without auth", ep.method, ep.path)
		})
	}
}

// TestNotificationHandler_UserIsolation tests that users cannot access each other's notifications.
func TestNotificationHandler_UserIsolation(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("user A cannot see user B's notifications", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userAID := getUserIDFromToken(t, server, tokenA)
		userBID := getUserIDFromToken(t, server, tokenB)
		svc := notificationTestSvc(t, suite)

		// Create 3 notifications for A and 2 for B
		for i := 0; i < 3; i++ {
			sendNotification(t, svc, userAID, "system", fmt.Sprintf("A Notif %d", i), "msg")
		}
		for i := 0; i < 2; i++ {
			sendNotification(t, svc, userBID, "system", fmt.Sprintf("B Notif %d", i), "msg")
		}

		// User A should see only 3
		recA := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications", nil, tokenA)
		assert.Equal(t, http.StatusOK, recA.Code)
		envA := helpers.ParseEnvelope(t, recA)
		dataA := envA.Data.(map[string]interface{})
		assert.Equal(t, float64(3), dataA["total"].(float64), "user A should see 3 notifications")

		// User B should see only 2
		recB := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications", nil, tokenB)
		assert.Equal(t, http.StatusOK, recB.Code)
		envB := helpers.ParseEnvelope(t, recB)
		dataB := envB.Data.(map[string]interface{})
		assert.Equal(t, float64(2), dataB["total"].(float64), "user B should see 2 notifications")
	})

	t.Run("user A cannot mark user B's notification as read", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userBID := getUserIDFromToken(t, server, tokenB)
		svc := notificationTestSvc(t, suite)

		sendNotification(t, svc, userBID, "system", "B Only Notif", "msg")
		notifs, _, err := svc.ListByUser(context.Background(), userBID, 10, 0)
		require.NoError(t, err)
		require.Len(t, notifs, 1)

		rec := helpers.MakeRequest(t, server, http.MethodPatch, "/api/v1/notifications/"+notifs[0].ID.String()+"/read", nil, tokenA)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestNotificationHandler_RateLimiting tests that sending more than 100 notifications per hour is rejected.
func TestNotificationHandler_RateLimiting(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("100th notification succeeds and 101st is rate limited", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{})

		userID := getUserIDFromToken(t, server, token)
		svc := notificationTestSvc(t, suite)

		// Send 100 notifications — all should succeed
		for i := 0; i < 100; i++ {
			err := svc.Send(context.Background(), userID, "system", fmt.Sprintf("Notif %d", i), "msg", "")
			require.NoError(t, err, "notification %d should succeed within rate limit", i)
		}

		// 101st should be rejected with a rate limit error
		err := svc.Send(context.Background(), userID, "system", "Over Limit", "msg", "")
		require.Error(t, err, "101st notification should be rejected by rate limiter")

		// Verify via list that exactly 100 notifications exist
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/notifications", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		total := data["total"].(float64)
		assert.Equal(t, float64(100), total, "exactly 100 notifications should exist after rate limit hit")
	})
}
