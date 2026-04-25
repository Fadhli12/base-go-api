package http

import (
	"testing"

	"github.com/example/go-api-base/internal/config"
	"github.com/example/go-api-base/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerWithLogger(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080},
		Log:    config.LogConfig{Level: "info"},
		CORS:   config.CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}},
	}

	// Create a test logger
	log := logger.NewNopLogger()

	// Create server with logger
	server := NewServer(cfg, nil, nil, nil, log)

	// Assert server was created
	require.NotNil(t, server)
	assert.NotNil(t, server.Echo())
	assert.Equal(t, cfg, server.Config())

	// Assert logger was set
	assert.Equal(t, log, server.Logger())
}

func TestNewServerWithNilLogger(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080},
		Log:    config.LogConfig{Level: "info"},
		CORS:   config.CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}},
	}

	// Create server without logger (nil)
	server := NewServer(cfg, nil, nil, nil, nil)

	// Assert server was created
	require.NotNil(t, server)
	assert.NotNil(t, server.Echo())

	// Logger should be nil
	assert.Nil(t, server.Logger())
}

func TestSetLogger(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080},
		Log:    config.LogConfig{Level: "info"},
		CORS:   config.CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}},
	}

	// Create server without logger
	server := NewServer(cfg, nil, nil, nil, nil)
	require.NotNil(t, server)

	// Logger should be nil initially
	assert.Nil(t, server.Logger())

	// Create a test logger and set it
	log := logger.NewNopLogger()
	server.SetLogger(log)

	// Logger should now be set
	assert.Equal(t, log, server.Logger())
}
