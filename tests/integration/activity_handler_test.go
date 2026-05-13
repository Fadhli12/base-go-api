//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	"github.com/example/go-api-base/internal/service"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"log/slog"
)

// activityTestSvc builds an ActivityService wired to the test suite DB.
func activityTestSvc(t *testing.T, suite *TestSuite) *service.ActivityService {
	t.Helper()

	activityRepo := repository.NewActivityRepository(suite.DB)
	activityReadRepo := repository.NewActivityReadRepository(suite.DB)
	activityFollowRepo := repository.NewActivityFollowRepository(suite.DB)
	userRepo := repository.NewUserRepository(suite.DB)

	return service.NewActivityService(
		activityRepo,
		activityReadRepo,
		activityFollowRepo,
		userRepo,
		nil, // enforcer from server
		nil, // audit service
		slog.Default(),
	)
}

// activityFollowTestSvc builds an ActivityFollowService wired to the test suite DB.
func activityFollowTestSvc(t *testing.T, suite *TestSuite, server *helpers.Server) *service.ActivityFollowService {
	t.Helper()

	activityFollowRepo := repository.NewActivityFollowRepository(suite.DB)

	return service.NewActivityFollowService(
		activityFollowRepo,
		server.Enforcer(),
		nil, // audit service
		slog.Default(),
	)
}

// createActivity is a helper that directly creates an Activity via the repository.
// Creates activities with nil OrganizationID (global scope) to avoid FK violations.
func createActivity(t *testing.T, suite *TestSuite, actorID uuid.UUID, actionType, resourceType, resourceID string) *domain.Activity {
	t.Helper()
	activity := &domain.Activity{
		ActorID:      actorID,
		ActionType:   actionType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     datatypes.JSON([]byte(`{"description":"test activity"}`)),
	}
	repo := repository.NewActivityRepository(suite.DB)
	err := repo.Create(context.Background(), activity)
	require.NoError(t, err, "Create activity must not fail")
	return activity
}

// getUserIDFromToken parses the /api/v1/me endpoint to get the authenticated user's UUID.
// (Reused from notification tests — same pattern.)
func getUserIDFromTokenActivity(t *testing.T, server *helpers.Server, token string) uuid.UUID {
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

// TestActivityHandler_List tests GET /api/v1/activities with pagination and filters.
func TestActivityHandler_List(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("pagination limit and offset", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)
		_ = activityTestSvc(t, suite) // ensure wiring works

		// Create 15 activities
		for i := 0; i < 15; i++ {
			createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		}

		// First page: limit=10, offset=0
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities?limit=10&offset=0", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok)
		total := data["total"].(float64)
		items := data["activities"].([]interface{})
		assert.Equal(t, float64(15), total, "total should be 15")
		assert.Len(t, items, 10, "should return 10 items")

		// Second page: limit=10, offset=10
		rec2 := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities?limit=10&offset=10", nil, token)
		assert.Equal(t, http.StatusOK, rec2.Code)
		env2 := helpers.ParseEnvelope(t, rec2)
		data2 := env2.Data.(map[string]interface{})
		items2 := data2["activities"].([]interface{})
		assert.Len(t, items2, 5, "second page should return 5 items")
	})

	t.Run("filter by action_type", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)

		// Create activities with different action types
		createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceInvoice, uuid.New().String())
		createActivity(t, suite, userID, domain.ActivityActionDeleted, domain.ActivityResourceUser, uuid.New().String())

		// Filter by "created" action type
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities?action_type=created", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		items := data["activities"].([]interface{})
		assert.Equal(t, float64(2), data["total"].(float64), "should return 2 created activities")
		assert.Len(t, items, 2, "should return 2 created activities")
	})

	t.Run("filter by resource_type", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)

		createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceInvoice, uuid.New().String())
		createActivity(t, suite, userID, domain.ActivityActionPaid, domain.ActivityResourceInvoice, uuid.New().String())

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities?resource_type=invoice", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, float64(2), data["total"].(float64), "should return 2 invoice activities")
	})
}

// TestActivityHandler_CountUnread tests GET /api/v1/activities/count-unread.
func TestActivityHandler_CountUnread(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("counts only unread activities", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)
		svc := activityTestSvc(t, suite)

		// Create 5 activities
		var activityIDs []uuid.UUID
		for i := 0; i < 5; i++ {
			a := createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
			activityIDs = append(activityIDs, a.ID)
		}

		// Mark 2 as read
		err := svc.MarkAsRead(context.Background(), userID, activityIDs[0])
		require.NoError(t, err)
		err = svc.MarkAsRead(context.Background(), userID, activityIDs[1])
		require.NoError(t, err)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/count-unread", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)
		data := env.Data.(map[string]interface{})
		unreadCount := data["unread_count"].(float64)
		assert.Equal(t, float64(3), unreadCount, "unread_count should be 3 (5 created - 2 read)")
	})

	t.Run("count drops to zero after marking all read", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)
		svc := activityTestSvc(t, suite)

		// Create 3 activities
		for i := 0; i < 3; i++ {
			createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		}

		// Mark all as read via service
		marked, err := svc.MarkAllRead(context.Background(), userID, false, uuid.Nil)
		require.NoError(t, err)
		require.Equal(t, int64(3), marked)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/count-unread", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		unreadCount := data["unread_count"].(float64)
		assert.Equal(t, float64(0), unreadCount, "unread_count should be 0 after marking all read")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/count-unread", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestActivityHandler_MarkAllRead tests PUT /api/v1/activities/read-all.
func TestActivityHandler_MarkAllRead(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("marks all activities as read", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)

		// Create 5 activities
		for i := 0; i < 5; i++ {
			createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		}

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/read-all", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		markedCount := data["marked_count"].(float64)
		assert.Equal(t, float64(5), markedCount, "marked_count should be 5")

		// Verify unread count is now 0
		countRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/count-unread", nil, token)
		assert.Equal(t, http.StatusOK, countRec.Code)
		countEnv := helpers.ParseEnvelope(t, countRec)
		countData := countEnv.Data.(map[string]interface{})
		assert.Equal(t, float64(0), countData["unread_count"].(float64), "unread count should be 0 after read-all")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/read-all", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestActivityHandler_MarkAsRead tests PUT /api/v1/activities/:id/read.
func TestActivityHandler_MarkAsRead(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("marks single activity as read", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)

		activity := createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/"+activity.ID.String()+"/read", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, activity.ID.String(), data["activity_id"], "activity_id should match")
		assert.NotEmpty(t, data["read_at"], "read_at should be set")

		// Verify via count-unread that it's now 0
		countRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/count-unread", nil, token)
		assert.Equal(t, http.StatusOK, countRec.Code)
		countEnv := helpers.ParseEnvelope(t, countRec)
		countData := countEnv.Data.(map[string]interface{})
		assert.Equal(t, float64(0), countData["unread_count"].(float64), "unread count should be 0 after marking read")
	})

	t.Run("invalid activity ID returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/not-a-uuid/read", nil, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("nonexistent activity ID returns 404", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/"+fakeID+"/read", nil, token)
		// ActivityService.MarkAsRead verifies activity exists first
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/activities/"+fakeID+"/read", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestActivityHandler_SoftDelete tests DELETE /api/v1/activities/:id.
func TestActivityHandler_SoftDelete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("soft-deletes an activity with manage permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// Need both activity:view and activity:manage permissions
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view", "activity:manage"})

		userID := getUserIDFromTokenActivity(t, server, token)

		// Create 2 activities, delete one
		createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())
		toDelete := createActivity(t, suite, userID, domain.ActivityActionDeleted, domain.ActivityResourceUser, uuid.New().String())

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/activities/"+toDelete.ID.String(), nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, "deleted", data["status"], "should return status deleted")

		// Verify the activity no longer appears in list (soft-deleted)
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)
		listEnv := helpers.ParseEnvelope(t, listRec)
		listData := listEnv.Data.(map[string]interface{})
		total := listData["total"].(float64)
		assert.Equal(t, float64(1), total, "only 1 activity should remain after soft delete")
	})

	t.Run("delete without manage permission returns 403", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// Only have view permission, not manage
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		userID := getUserIDFromTokenActivity(t, server, token)

		activity := createActivity(t, suite, userID, domain.ActivityActionCreated, domain.ActivityResourceUser, uuid.New().String())

		// Note: The DELETE route is registered in the activities group which has activity:view middleware,
		// but the handler also checks activity:manage permission
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/activities/"+activity.ID.String(), nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("invalid activity ID returns 400", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view", "activity:manage"})

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/activities/not-a-uuid", nil, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)

		fakeID := uuid.New().String()
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/activities/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// TestActivityHandler_Unauthorized tests that all activity endpoints return 401 without a JWT token.
func TestActivityHandler_Unauthorized(t *testing.T) {
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
		{http.MethodGet, "/api/v1/activities"},
		{http.MethodGet, "/api/v1/activities/count-unread"},
		{http.MethodPut, "/api/v1/activities/read-all"},
		{http.MethodPut, "/api/v1/activities/" + fakeID + "/read"},
		{http.MethodDelete, "/api/v1/activities/" + fakeID},
		{http.MethodPost, "/api/v1/activities/follow"},
		{http.MethodGet, "/api/v1/activities/follows"},
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

// TestActivityHandler_Follow tests POST /api/v1/activities/follow and related follow endpoints.
func TestActivityHandler_Follow(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("follow a resource and list follows", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		resourceID := uuid.New().String()

		// Follow a resource
		followBody := map[string]interface{}{
			"resource_type": "user",
			"resource_id":   resourceID,
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/activities/follow", followBody, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, "user", data["resource_type"], "resource_type should be user")
		assert.Equal(t, resourceID, data["resource_id"], "resource_id should match")

		// List follows
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/follows", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)
		listEnv := helpers.ParseEnvelope(t, listRec)
		listData := listEnv.Data.(map[string]interface{})
		follows := listData["follows"].([]interface{})
		total := listData["total"].(float64)
		assert.Equal(t, float64(1), total, "should have 1 follow")
		assert.Len(t, follows, 1, "should return 1 follow")
	})

	t.Run("follow with invalid resource_type returns 422", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		followBody := map[string]interface{}{
			"resource_type": "invalid_type",
			"resource_id":   uuid.New().String(),
		}
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/activities/follow", followBody, token)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("unfollow a resource", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{"activity:view"})

		resourceID := uuid.New().String()

		// First, follow
		followBody := map[string]interface{}{
			"resource_type": "invoice",
			"resource_id":   resourceID,
		}
		followRec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/activities/follow", followBody, token)
		require.Equal(t, http.StatusCreated, followRec.Code)
		followEnv := helpers.ParseEnvelope(t, followRec)
		followData := followEnv.Data.(map[string]interface{})
		followID := followData["id"].(string)

		// Then, unfollow
		unfollowRec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/activities/follow/"+followID, nil, token)
		assert.Equal(t, http.StatusOK, unfollowRec.Code)

		// Verify follows list is empty
		listRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/activities/follows", nil, token)
		assert.Equal(t, http.StatusOK, listRec.Code)
		listEnv := helpers.ParseEnvelope(t, listRec)
		listData := listEnv.Data.(map[string]interface{})
		total := listData["total"].(float64)
		assert.Equal(t, float64(0), total, "should have 0 follows after unfollow")
	})
}