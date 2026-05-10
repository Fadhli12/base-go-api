//go:build integration
// +build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/example/go-api-base/internal/logger"
	"github.com/example/go-api-base/tests/integration/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwaggerEnabled(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	cfg := helpers.DefaultTestConfig()
	cfg.Swagger.Enabled = true
	cfg.Swagger.Path = "/swagger"

	log, err := logger.NewLogger(logger.Config{Level: "debug", Format: "json", Outputs: "stdout"})
	require.NoError(t, err)

	server := helpers.NewTestServerWithConfig(t, suite, cfg, log)

	rec := helpers.MakeRequest(t, server, http.MethodGet, "/swagger/index.html", nil, "")
	assert.Equal(t, http.StatusOK, rec.Code, "swagger UI should be accessible when enabled")
}

func TestSwaggerDisabled(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	cfg := helpers.DefaultTestConfig()
	cfg.Swagger.Enabled = false

	log, err := logger.NewLogger(logger.Config{Level: "debug", Format: "json", Outputs: "stdout"})
	require.NoError(t, err)

	server := helpers.NewTestServerWithConfig(t, suite, cfg, log)

	rec := helpers.MakeRequest(t, server, http.MethodGet, "/swagger/index.html", nil, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, "swagger UI should return 404 when disabled")
}

func TestCustomSwaggerPath(t *testing.T) {
	suite := NewTestSuite(t)
	defer suite.Cleanup()
	suite.RunMigrations(t)
	suite.SetupTest(t)

	cfg := helpers.DefaultTestConfig()
	cfg.Swagger.Enabled = true
	cfg.Swagger.Path = "/api-docs"

	log, err := logger.NewLogger(logger.Config{Level: "debug", Format: "json", Outputs: "stdout"})
	require.NoError(t, err)

	server := helpers.NewTestServerWithConfig(t, suite, cfg, log)

	rec := helpers.MakeRequest(t, server, http.MethodGet, "/api-docs/index.html", nil, "")
	assert.Equal(t, http.StatusOK, rec.Code, "swagger UI should be accessible at custom path")
}