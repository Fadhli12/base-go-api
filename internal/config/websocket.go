// Package config provides application configuration management.
package config

import "time"

// WsConfig holds WebSocket system configuration.
type WsConfig struct {
	Enabled            bool          `mapstructure:"ws_enabled"`
	AllowedOrigins     []string      `mapstructure:"ws_allowed_origins"`
	HeartbeatInterval  time.Duration `mapstructure:"ws_heartbeat_interval"`
	HeartbeatTimeout   time.Duration `mapstructure:"ws_heartbeat_timeout"`
	MaxMessageSize     int64         `mapstructure:"ws_max_message_size"`
	MaxRoomsPerClient  int           `mapstructure:"ws_max_rooms_per_client"`
	WriteTimeout       time.Duration `mapstructure:"ws_write_timeout"`
	ReadTimeout        time.Duration `mapstructure:"ws_read_timeout"`
	TypingTTL          time.Duration `mapstructure:"ws_typing_ttl"`
	PresenceTTL        time.Duration `mapstructure:"ws_presence_ttl"`
	SendChannelBuffer  int           `mapstructure:"ws_send_channel_buffer"`
}

// DefaultWsConfig returns a WsConfig with sensible defaults.
func DefaultWsConfig() WsConfig {
	return WsConfig{
		Enabled:           true,
		AllowedOrigins:     []string{"*"},
		HeartbeatInterval:  30 * time.Second,
		HeartbeatTimeout:    60 * time.Second,
		MaxMessageSize:      65536, // 64KB
		MaxRoomsPerClient:   50,
		WriteTimeout:        10 * time.Second,
		ReadTimeout:         60 * time.Second,
		TypingTTL:           5 * time.Second,
		PresenceTTL:         70 * time.Second,
		SendChannelBuffer:   256,
	}
}