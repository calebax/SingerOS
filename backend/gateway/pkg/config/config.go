// Package config provides gateway configuration loading and types.
package config

import (
	"fmt"
	"os"

	"github.com/insmtx/Leros/backend/gateway/pkg/types"
	"gopkg.in/yaml.v3"
)

// GatewayConfig is the top-level gateway configuration.
type GatewayConfig struct {
	// Server holds the HTTP server settings for webhook-receiving adapters.
	Server ServerConfig `yaml:"server"`

	// NATS configures the connection to the NATS event bus.
	NATS NATSConfig `yaml:"nats"`

	// Auth holds the authorization policies.
	Auth AuthConfig `yaml:"auth,omitempty"`

	// Channels holds per-channel configuration.
	// Keys are channel codes. Adapters read their own config from here.
	Channels map[types.ChannelCode]ChannelConfig `yaml:"channels"`

	// Stream configures streaming response behavior.
	Stream StreamConfig `yaml:"stream,omitempty"`
}

// ServerConfig configures the embedded HTTP server.
type ServerConfig struct {
	// Host is the listen address (e.g., "0.0.0.0" or "127.0.0.1").
	Host string `yaml:"host"`
	// Port is the listen port.
	Port int `yaml:"port"`
}

// NATSConfig configures the NATS connection.
type NATSConfig struct {
	// URL is the NATS server address.
	URL string `yaml:"url"`
}

// AuthConfig holds authorization settings.
type AuthConfig struct {
	// GlobalAllowlist are user IDs permitted across all channels.
	GlobalAllowlist []string `yaml:"global_allowlist,omitempty"`
	// GlobalDenylist are user IDs denied across all channels.
	GlobalDenylist []string `yaml:"global_denylist,omitempty"`
}

// ChannelConfig holds per-channel settings.
//
// Adapters use Enabled to determine activation, and Extra for
// platform-specific parameters. Auth overrides the global auth
// policies for this channel.
type ChannelConfig struct {
	// Enabled controls whether this channel is active.
	Enabled bool `yaml:"enabled"`
	// Auth overrides global auth for this channel.
	Auth *ChannelAuthConfig `yaml:"auth,omitempty"`
	// Extra holds platform-specific settings.
	Extra map[string]any `yaml:"extra,omitempty"`
}

// ChannelAuthConfig holds per-channel auth overrides.
type ChannelAuthConfig struct {
	// Allowlist user IDs for this channel only.
	Allowlist []string `yaml:"allowlist,omitempty"`
	// Denylist user IDs for this channel only.
	Denylist []string `yaml:"denylist,omitempty"`
	// RequireMention forces group messages to contain a mention phrase.
	RequireMention string `yaml:"require_mention,omitempty"`
}

// StreamConfig configures streaming response behavior.
type StreamConfig struct {
	// Enabled toggles token-by-token streaming delivery.
	Enabled bool `yaml:"enabled"`
	// ChunkSize is characters per delivery chunk.
	ChunkSize int `yaml:"chunk_size"`
}

// LoadGatewayConfig reads and parses the gateway YAML configuration file.
func LoadGatewayConfig(path string) (*GatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg GatewayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Channels == nil {
		cfg.Channels = make(map[types.ChannelCode]ChannelConfig)
	}

	return &cfg, nil
}

// Addr returns the HTTP listen address.
func (s ServerConfig) Addr() string {
	if s.Host == "" {
		s.Host = "0.0.0.0"
	}
	if s.Port == 0 {
		s.Port = 8080
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// ChannelCodes returns all configured channel codes (both enabled and disabled).
func (c *GatewayConfig) ChannelCodes() []types.ChannelCode {
	codes := make([]types.ChannelCode, 0, len(c.Channels))
	for code := range c.Channels {
		codes = append(codes, code)
	}
	return codes
}
