//go:build integration
// +build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createCommentViaAPI creates a comment via the HTTP endpoint and returns the response data map.
func createCommentViaAPI(t *testing.T, server *helpers.Server, token, commentableType, commentableID string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	path := fmt.Sprintf("/api/v1/%s/%s/comments", commentableType, commentableID)
	rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
	require.Equal(t, http.StatusCreated, rec.Code, "create comment should return 201")
	env := helpers.ParseEnvelope(t, rec)
	require.NotNil(t, env.Data, "create comment response should have data")
	data, ok := env.Data.(map[string]interface{})
	require.True(t, ok, "create comment data should be a map")
	return data
}

// TestCommentHandler_Create tests POST /api/v1/:type/:id/comments.
func TestCommentHandler_Create(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("creates a comment on a news entity", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		body := map[string]interface{}{
			"content": "This is a test comment",
		}

		path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data)
		data := env.Data.(map[string]interface{})

		assert.NotEmpty(t, data["id"], "comment should have an ID")
		assert.Equal(t, "This is a test comment", data["content"])
		assert.Equal(t, "news", data["commentable_type"])
		assert.Equal(t, newsID.String(), data["commentable_id"])
		assert.Equal(t, false, data["is_pinned"])
		assert.Nil(t, data["parent_id"], "top-level comment should have no parent_id")
	})

	t.Run("creates a reply to an existing comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		// Create parent comment first
		parentData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Parent comment",
		})
		parentID := parentData["id"].(string)

		// Create reply
		body := map[string]interface{}{
			"content":   "This is a reply",
			"parent_id": parentID,
		}
		path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
		assert.Equal(t, http.StatusCreated, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})

		assert.NotEmpty(t, data["id"])
		assert.Equal(t, "This is a reply", data["content"])
		// Replies are flattened: parent_id should point to top-level parent
		assert.NotNil(t, data["parent_id"], "reply should have parent_id")
	})

	t.Run("rejects invalid commentable type", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create",
		})

		fakeID := uuid.New()
		body := map[string]interface{}{
			"content": "Test comment",
		}

		path := fmt.Sprintf("/api/v1/invalid_type/%s/comments", fakeID)
		rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Error)
		assert.Equal(t, "VALIDATION_ERROR", env.Error.Code)
	})

	t.Run("rejects empty content", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create",
		})

		newsID := uuid.New()
		body := map[string]interface{}{
			"content": "",
		}

		path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("rejects without comment:create permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User with only view permission, not create
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:view",
		})

		newsID := uuid.New()
		body := map[string]interface{}{
			"content": "Should be forbidden",
		}

		path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestCommentHandler_GetByID tests GET /api/v1/comments/:id.
func TestCommentHandler_GetByID(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("retrieves a comment by ID", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Fetch me",
		})
		commentID := createdData["id"].(string)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+commentID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, commentID, data["id"])
		assert.Equal(t, "Fetch me", data["content"])
	})

	t.Run("returns 404 for nonexistent comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:view",
		})

		fakeID := uuid.New()
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+fakeID.String(), nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// TestCommentHandler_ListByCommentable tests GET /api/v1/:type/:id/comments.
func TestCommentHandler_ListByCommentable(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("lists comments for a commentable entity", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		// Create 3 comments
		for i := 0; i < 3; i++ {
			createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
				"content": fmt.Sprintf("Comment %d", i),
			})
		}

		path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, float64(3), data["total"].(float64))

		comments := data["comments"].([]interface{})
		assert.Len(t, comments, 3)
	})

	t.Run("lists comments with pagination", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		for i := 0; i < 5; i++ {
			createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
				"content": fmt.Sprintf("Comment %d", i),
			})
		}

		path := fmt.Sprintf("/api/v1/news/%s/comments?limit=2&offset=0", newsID)
		rec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, float64(5), data["total"].(float64))

		comments := data["comments"].([]interface{})
		assert.Len(t, comments, 2, "should return 2 comments with limit=2")
	})

	t.Run("rejects invalid commentable type", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:view",
		})

		fakeID := uuid.New()
		path := fmt.Sprintf("/api/v1/invalid/%s/comments", fakeID)
		rec := helpers.MakeRequest(t, server, http.MethodGet, path, nil, token)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

// TestCommentHandler_Update tests PUT /api/v1/comments/:id.
func TestCommentHandler_Update(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("updates comment content", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Original content",
		})
		commentID := createdData["id"].(string)

		updateBody := map[string]interface{}{
			"content": "Updated content",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/comments/"+commentID, updateBody, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, "Updated content", data["content"])
		assert.NotNil(t, data["edited_at"], "edited_at should be set after update")
	})

	t.Run("author can update own comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "My comment",
		})
		commentID := createdData["id"].(string)

		updateBody := map[string]interface{}{
			"content": "Updated by author",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/comments/"+commentID, updateBody, token)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non-author cannot update another user's comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		// User A creates a comment
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, tokenA, "news", newsID.String(), map[string]interface{}{
			"content": "User A's comment",
		})
		commentID := createdData["id"].(string)

		// User B tries to update User A's comment
		updateBody := map[string]interface{}{
			"content": "Hacked by B",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/comments/"+commentID, updateBody, tokenB)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rejects empty content on update", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Original",
		})
		commentID := createdData["id"].(string)

		updateBody := map[string]interface{}{
			"content": "",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/comments/"+commentID, updateBody, token)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestCommentHandler_Delete tests DELETE /api/v1/comments/:id.
func TestCommentHandler_Delete(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("deletes a comment and it becomes inaccessible", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Delete me",
		})
		commentID := createdData["id"].(string)

		// Delete
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/comments/"+commentID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify 404 on GET
		rec = helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+commentID, nil, token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("author can delete own comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "My comment to delete",
		})
		commentID := createdData["id"].(string)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/comments/"+commentID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("non-author without delete_any cannot delete others' comments", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})
		// User B has delete but NOT delete_any
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, tokenA, "news", newsID.String(), map[string]interface{}{
			"content": "User A's comment",
		})
		commentID := createdData["id"].(string)

		// User B tries to delete User A's comment
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/comments/"+commentID, nil, tokenB)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("user with delete_any can delete others' comments", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})
		// Admin with delete_any
		tokenAdmin, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:view", "comment:delete_any",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, tokenA, "news", newsID.String(), map[string]interface{}{
			"content": "User A's comment",
		})
		commentID := createdData["id"].(string)

		// Admin deletes User A's comment
		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/comments/"+commentID, nil, tokenAdmin)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestCommentHandler_PinUnpin tests POST /api/v1/comments/:id/pin and /unpin.
func TestCommentHandler_PinUnpin(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("pins and unpins a comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:manage",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Pin me",
		})
		commentID := createdData["id"].(string)

		// Pin
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/pin", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, true, data["is_pinned"], "comment should be pinned")

		// Verify pinned via GET
		getRec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+commentID, nil, token)
		assert.Equal(t, http.StatusOK, getRec.Code)
		getEnv := helpers.ParseEnvelope(t, getRec)
		getData := getEnv.Data.(map[string]interface{})
		assert.Equal(t, true, getData["is_pinned"], "comment should remain pinned")

		// Unpin
		rec = helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/unpin", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)
		env = helpers.ParseEnvelope(t, rec)
		data = env.Data.(map[string]interface{})
		assert.Equal(t, false, data["is_pinned"], "comment should be unpinned")
	})

	t.Run("pin requires comment:manage permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Try to pin",
		})
		commentID := createdData["id"].(string)

		// Pin without manage permission
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/pin", nil, token)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("pinning already pinned comment returns 422", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:manage",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Already pinned",
		})
		commentID := createdData["id"].(string)

		// Pin once
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/pin", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Pin again - should fail with 422
		rec = helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/pin", nil, token)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("unpinning already unpinned comment returns 422", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:manage",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Not pinned",
		})
		commentID := createdData["id"].(string)

		// Unpin without pinning first - should fail with 422
		rec := helpers.MakeRequest(t, server, http.MethodPost, "/api/v1/comments/"+commentID+"/unpin", nil, token)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

// TestCommentHandler_ListReplies tests GET /api/v1/comments/:id/replies.
func TestCommentHandler_ListReplies(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("lists replies to a parent comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		// Create parent comment
		parentData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "Parent comment",
		})
		parentID := parentData["id"].(string)

		// Create 2 replies
		for i := 0; i < 2; i++ {
			body := map[string]interface{}{
				"content":   fmt.Sprintf("Reply %d", i),
				"parent_id": parentID,
			}
			path := fmt.Sprintf("/api/v1/news/%s/comments", newsID)
			rec := helpers.MakeRequest(t, server, http.MethodPost, path, body, token)
			require.Equal(t, http.StatusCreated, rec.Code)
		}

		// List replies
		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+parentID+"/replies", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, float64(2), data["total"].(float64))

		replies := data["comments"].([]interface{})
		assert.Len(t, replies, 2)
	})

	t.Run("returns empty list for comment without replies", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		parentData := createCommentViaAPI(t, server, token, "news", newsID.String(), map[string]interface{}{
			"content": "No replies yet",
		})
		parentID := parentData["id"].(string)

		rec := helpers.MakeRequest(t, server, http.MethodGet, "/api/v1/comments/"+parentID+"/replies", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code)

		env := helpers.ParseEnvelope(t, rec)
		data := env.Data.(map[string]interface{})
		assert.Equal(t, float64(0), data["total"].(float64))
	})
}

// TestCommentHandler_Unauthorized tests that all comment endpoints return 401 without a JWT token.
func TestCommentHandler_Unauthorized(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	server := helpers.NewTestServer(t, suite)
	fakeID := uuid.New()
	fakeTypeID := fmt.Sprintf("/api/v1/news/%s/comments", uuid.New())

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, fakeTypeID},                   // Create
		{http.MethodGet, fakeTypeID},                   // ListByCommentable
		{http.MethodGet, "/api/v1/comments/" + fakeID.String()},          // GetByID
		{http.MethodGet, "/api/v1/comments/" + fakeID.String() + "/replies"}, // ListReplies
		{http.MethodPut, "/api/v1/comments/" + fakeID.String()},           // Update
		{http.MethodDelete, "/api/v1/comments/" + fakeID.String()},       // Delete
		{http.MethodPost, "/api/v1/comments/" + fakeID.String() + "/pin"},   // Pin
		{http.MethodPost, "/api/v1/comments/" + fakeID.String() + "/unpin"}, // Unpin
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

// TestCommentHandler_UserIsolation tests that non-owners cannot modify others' comments.
func TestCommentHandler_UserIsolation(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)

	t.Run("user cannot update another user's comment", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, tokenA, "news", newsID.String(), map[string]interface{}{
			"content": "User A comment",
		})
		commentID := createdData["id"].(string)

		updateBody := map[string]interface{}{
			"content": "Hacked by B",
		}
		rec := helpers.MakeRequest(t, server, http.MethodPut, "/api/v1/comments/"+commentID, updateBody, tokenB)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("non-owner cannot delete without delete_any permission", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		tokenA, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})
		// User B has delete permission but NOT delete_any
		tokenB, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view", "comment:delete",
		})

		newsID := uuid.New()
		createdData := createCommentViaAPI(t, server, tokenA, "news", newsID.String(), map[string]interface{}{
			"content": "User A comment",
		})
		commentID := createdData["id"].(string)

		rec := helpers.MakeRequest(t, server, http.MethodDelete, "/api/v1/comments/"+commentID, nil, tokenB)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("commentable isolation - comments scoped to entity", func(t *testing.T) {
		suite.SetupTest(t)
		server := helpers.NewTestServer(t, suite)
		token, _ := helpers.CreateUserWithPermissions(t, suite, server.Enforcer(), []string{
			"comment:create", "comment:view",
		})

		newsA := uuid.New()
		newsB := uuid.New()

		// Create comments on different entities
		createCommentViaAPI(t, server, token, "news", newsA.String(), map[string]interface{}{
			"content": "Comment on A",
		})
		createCommentViaAPI(t, server, token, "news", newsB.String(), map[string]interface{}{
			"content": "Comment on B",
		})

		// List for A should only show A's comments
		recA := helpers.MakeRequest(t, server, http.MethodGet, fmt.Sprintf("/api/v1/news/%s/comments", newsA), nil, token)
		assert.Equal(t, http.StatusOK, recA.Code)
		envA := helpers.ParseEnvelope(t, recA)
		dataA := envA.Data.(map[string]interface{})
		assert.Equal(t, float64(1), dataA["total"].(float64), "should only see 1 comment for entity A")

		// List for B should only show B's comments
		recB := helpers.MakeRequest(t, server, http.MethodGet, fmt.Sprintf("/api/v1/news/%s/comments", newsB), nil, token)
		assert.Equal(t, http.StatusOK, recB.Code)
		envB := helpers.ParseEnvelope(t, recB)
		dataB := envB.Data.(map[string]interface{})
		assert.Equal(t, float64(1), dataB["total"].(float64), "should only see 1 comment for entity B")
	})
}

