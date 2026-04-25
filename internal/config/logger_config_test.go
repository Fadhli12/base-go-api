package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogConfigDefaults(t *testing.T) {
	// Clear environment
	os.Clearenv()

	v := viper.New()
	setDefaults(v)

	assert.Equal(t, "info", v.GetString("log.level"))
	assert.Equal(t, "json", v.GetString("log.format"))
	assert.Equal(t, "stdout", v.GetString("log.outputs"))
	assert.Equal(t, "/var/log/api.log", v.GetString("log.file_path"))
	assert.Equal(t, 100, v.GetInt("log.max_size"))
	assert.Equal(t, 10, v.GetInt("log.max_backups"))
	assert.Equal(t, 30, v.GetInt("log.max_age"))
	assert.Equal(t, true, v.GetBool("log.compress"))
	assert.Equal(t, "", v.GetString("log.syslog_network"))
	assert.Equal(t, "", v.GetString("log.syslog_address"))
	assert.Equal(t, "go-api", v.GetString("log.syslog_tag"))
	assert.Equal(t, false, v.GetBool("log.add_source"))
}

func TestLogConfigParsing(t *testing.T) {
	// Clear environment
	os.Clearenv()
	defer os.Clearenv()

	// Set environment variables
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "text")
	os.Setenv("LOG_OUTPUTS", "stdout,file")
	os.Setenv("LOG_FILE_PATH", "/custom/log/path.log")
	os.Setenv("LOG_FILE_MAX_SIZE", "50")
	os.Setenv("LOG_FILE_MAX_BACKUPS", "5")
	os.Setenv("LOG_FILE_MAX_AGE", "7")
	os.Setenv("LOG_FILE_COMPRESS", "false")
	os.Setenv("LOG_SYSLOG_NETWORK", "tcp")
	os.Setenv("LOG_SYSLOG_ADDRESS", "localhost:514")
	os.Setenv("LOG_SYSLOG_TAG", "my-app")
	os.Setenv("LOG_ADD_SOURCE", "true")

	v := viper.New()
	v.SetEnvPrefix("")
	v.AutomaticEnv()
	setDefaults(v)

	cfg := &Config{}
	err := parseLogConfig(v, cfg)
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, "stdout,file", cfg.Log.Outputs)
	assert.Equal(t, "/custom/log/path.log", cfg.Log.FilePath)
	assert.Equal(t, 50, cfg.Log.MaxSize)
	assert.Equal(t, 5, cfg.Log.MaxBackups)
	assert.Equal(t, 7, cfg.Log.MaxAge)
	assert.Equal(t, false, cfg.Log.Compress)
	assert.Equal(t, "tcp", cfg.Log.SyslogNetwork)
	assert.Equal(t, "localhost:514", cfg.Log.SyslogAddress)
	assert.Equal(t, "my-app", cfg.Log.SyslogTag)
	assert.Equal(t, true, cfg.Log.AddSource)
}

func TestLogConfigPartial(t *testing.T) {
	// Clear environment
	os.Clearenv()
	defer os.Clearenv()

	// Set only some environment variables
	os.Setenv("LOG_LEVEL", "warn")
	os.Setenv("LOG_FILE_MAX_SIZE", "200")

	v := viper.New()
	v.SetEnvPrefix("")
	v.AutomaticEnv()
	setDefaults(v)

	cfg := &Config{}
	err := parseLogConfig(v, cfg)
	require.NoError(t, err)

	// Custom values
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, 200, cfg.Log.MaxSize)

	// Defaults should remain
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, "stdout", cfg.Log.Outputs)
	assert.Equal(t, "/var/log/api.log", cfg.Log.FilePath)
	assert.Equal(t, 10, cfg.Log.MaxBackups)
	assert.Equal(t, 30, cfg.Log.MaxAge)
	assert.Equal(t, true, cfg.Log.Compress)
}
