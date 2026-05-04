//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceHandler(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	server := helpers.NewTestServer(t, suite)

	t.Run("POST /api/v1/invoices - create invoice (happy path)", func(t *testing.T) {
		suite.SetupTest(t)
		token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		payload := map[string]interface{}{
			"customer": "Acme Corp",
			"amount":   150.00,
		}

		rec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", payload, token)
		assert.Equal(t, http.StatusCreated, rec.Code, "create invoice should return 201")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotEmpty(t, data["id"], "invoice id should be present")
		assert.Equal(t, "Acme Corp", data["customer"], "customer should match")
		assert.Equal(t, 150.0, data["amount"], "amount should match")
		assert.Equal(t, "draft", data["status"], "status should be draft")
	})

	t.Run("GET /api/v1/invoices - list invoices (happy path)", func(t *testing.T) {
		suite.SetupTest(t)
		token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		createPayload := map[string]interface{}{
			"customer": "List Test Corp",
			"amount":   200.00,
		}
		createRec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", createPayload, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create invoice should succeed")

		rec := makeInvoiceRequest(t, server, http.MethodGet, "/api/v1/invoices", nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "list invoices should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.NotNil(t, data["data"], "response should contain invoices list")
		assert.NotNil(t, data["total"], "response should contain total count")
	})

	t.Run("GET /api/v1/invoices/:id - get invoice by ID (happy path)", func(t *testing.T) {
		suite.SetupTest(t)
		token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		createPayload := map[string]interface{}{
			"customer": "Get Test Corp",
			"amount":   300.00,
		}
		createRec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", createPayload, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create invoice should succeed")
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		invoiceID, ok := createData["id"].(string)
		require.True(t, ok, "invoice id should be a string")

		rec := makeInvoiceRequest(t, server, http.MethodGet, "/api/v1/invoices/"+invoiceID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "get invoice by id should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, invoiceID, data["id"], "invoice id should match")
		assert.Equal(t, "Get Test Corp", data["customer"], "customer should match")
	})

	t.Run("PUT /api/v1/invoices/:id - update invoice (happy path)", func(t *testing.T) {
		suite.SetupTest(t)
		token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		createPayload := map[string]interface{}{
			"customer": "Update Test Corp",
			"amount":   400.00,
		}
		createRec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", createPayload, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create invoice should succeed")
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		invoiceID, ok := createData["id"].(string)
		require.True(t, ok, "invoice id should be a string")

		updatePayload := map[string]interface{}{
			"customer": "Updated Corp",
			"amount":   500.00,
			"status":   "pending",
		}
		rec := makeInvoiceRequest(t, server, http.MethodPut, "/api/v1/invoices/"+invoiceID, updatePayload, token)
		assert.Equal(t, http.StatusOK, rec.Code, "update invoice should return 200")

		env := helpers.ParseEnvelope(t, rec)
		require.NotNil(t, env.Data, "response should have data")

		data, ok := env.Data.(map[string]interface{})
		require.True(t, ok, "data should be a map")
		assert.Equal(t, "Updated Corp", data["customer"], "customer should be updated")
		assert.Equal(t, 500.0, data["amount"], "amount should be updated")
		assert.Equal(t, "pending", data["status"], "status should be updated")
	})

	t.Run("DELETE /api/v1/invoices/:id - soft delete invoice (happy path)", func(t *testing.T) {
		suite.SetupTest(t)
		token, _ := helpers.CreateAdminUser(t, suite, server.Enforcer())

		createPayload := map[string]interface{}{
			"customer": "Delete Test Corp",
			"amount":   600.00,
		}
		createRec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", createPayload, token)
		require.Equal(t, http.StatusCreated, createRec.Code, "setup: create invoice should succeed")
		createEnv := helpers.ParseEnvelope(t, createRec)

		createData, ok := createEnv.Data.(map[string]interface{})
		require.True(t, ok, "create response data should be a map")
		invoiceID, ok := createData["id"].(string)
		require.True(t, ok, "invoice id should be a string")

		rec := makeInvoiceRequest(t, server, http.MethodDelete, "/api/v1/invoices/"+invoiceID, nil, token)
		assert.Equal(t, http.StatusOK, rec.Code, "delete invoice should return 200")

		getRec := makeInvoiceRequest(t, server, http.MethodGet, "/api/v1/invoices/"+invoiceID, nil, token)
		assert.Equal(t, http.StatusNotFound, getRec.Code, "deleted invoice should return 404")
	})

	t.Run("unauthenticated requests return 401", func(t *testing.T) {
		suite.SetupTest(t)

		fakeID := "00000000-0000-0000-0000-000000000001"

		createPayload := map[string]interface{}{
			"customer": "Noauth Corp",
			"amount":   100.00,
		}
		rec := makeInvoiceRequest(t, server, http.MethodPost, "/api/v1/invoices", createPayload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "create invoice without auth should return 401")

		rec = makeInvoiceRequest(t, server, http.MethodGet, "/api/v1/invoices", nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "list invoices without auth should return 401")

		rec = makeInvoiceRequest(t, server, http.MethodGet, "/api/v1/invoices/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "get invoice without auth should return 401")

		updatePayload := map[string]interface{}{
			"customer": "Hacked Corp",
			"amount":   999.00,
		}
		rec = makeInvoiceRequest(t, server, http.MethodPut, "/api/v1/invoices/"+fakeID, updatePayload, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "update invoice without auth should return 401")

		rec = makeInvoiceRequest(t, server, http.MethodDelete, "/api/v1/invoices/"+fakeID, nil, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "delete invoice without auth should return 401")
	})
}

func makeInvoiceRequest(t *testing.T, server *helpers.Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err, "body should marshal to JSON")
		reqBody = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)
	return rec
}
