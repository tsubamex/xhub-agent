package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the Agent configuration structure
type Config struct {
	// Basic configuration
	UUID           string `yaml:"uuid"`           // Agent unique identifier
	XUIUser        string `yaml:"xui_user"`       // 3x-ui login username
	XUIPass        string `yaml:"xui_pass"`       // 3x-ui login password
	XHubAPIKey     string `yaml:"xhub_api_key"`   // xhub API key
	ResolvedDomain string `yaml:"resolvedDomain"` // DNS resolved domain for subscription reporting
	GRPCServer     string `yaml:"grpcServer"`     // gRPC server address
	GRPCPort       int    `yaml:"grpcPort"`       // gRPC server port

	// 3x-ui connection configuration
	RootPath string `yaml:"rootPath"` // 3x-ui rootPath
	Port     int    `yaml:"port"`     // 3x-ui port number

	// Optional configuration (with default values)
	XUIBaseURL   string `yaml:"xui_base_url"`  // 3x-ui base URL, default 127.0.0.1 (without port)
	PollInterval int    `yaml:"poll_interval"` // Poll interval (seconds), default 2
	LogLevel     string `yaml:"log_level"`     // Log level, default info
}

// LoadFromFile loads configuration from YAML file
func LoadFromFile(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply default values
	config.applyDefaults()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// applyDefaults applies default configuration values
func (c *Config) applyDefaults() {
	if c.XUIBaseURL == "" {
		c.XUIBaseURL = "127.0.0.1"
	}
	if c.PollInterval == 0 {
		c.PollInterval = 2 // gRPC 时代默认 2 秒轮询，提高响应速度
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	// Apply smart gRPC port defaults based on server type and TLS usage
	c.applySmartGRPCPortDefaults()
}

// applySmartGRPCPortDefaults sets intelligent gRPC port defaults
func (c *Config) applySmartGRPCPortDefaults() {
	// Check if this is a local server
	isLocal := c.isLocalServer()

	if isLocal {
		// Local development: keep existing port or default to 9090
		if c.GRPCPort <= 0 {
			c.GRPCPort = 9090
		}
		// For localhost, keep whatever port is configured (could be 9090 or custom)
	} else {
		// Production/remote server: ALWAYS override to use 443 (standard TLS port)
		// This ensures production servers use the secure port regardless of backend config
		c.GRPCPort = 443
	}
}

// isLocalServer checks if the gRPC server is a local development server
func (c *Config) isLocalServer() bool {
	return c.GRPCServer == "localhost" ||
		c.GRPCServer == "127.0.0.1" ||
		c.GRPCServer == "::1"
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.UUID == "" {
		return fmt.Errorf("UUID cannot be empty")
	}

	if c.XUIUser == "" {
		return fmt.Errorf("XUI username cannot be empty")
	}
	if c.XUIPass == "" {
		return fmt.Errorf("XUI password cannot be empty")
	}
	if c.XHubAPIKey == "" {
		return fmt.Errorf("XHub API key cannot be empty")
	}
	if c.GRPCServer == "" {
		return fmt.Errorf("gRPC server cannot be empty")
	}
	if c.GRPCPort <= 0 {
		return fmt.Errorf("gRPC port must be greater than 0")
	}
	if c.RootPath == "" {
		return fmt.Errorf("RootPath cannot be empty")
	}
	if c.Port <= 0 {
		return fmt.Errorf("port must be greater than 0")
	}
	return nil
}

// GetFullXUIURL gets the complete 3x-ui URL
func (c *Config) GetFullXUIURL() string {
	return fmt.Sprintf("https://%s:%d%s", c.XUIBaseURL, c.Port, c.RootPath)
}
