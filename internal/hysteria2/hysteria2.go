package hysteria2

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"xhub-agent/pkg/logger"
)

// Hysteria2Config represents the Hysteria2 server configuration structure
type Hysteria2Config struct {
	Listen string `yaml:"listen"` // e.g., ":443" or "0.0.0.0:443"

	TLS struct {
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
	} `yaml:"tls"`

	ACME struct {
		Domains []string `yaml:"domains"`
		Email   string   `yaml:"email"`
	} `yaml:"acme"`

	Auth struct {
		Type     string `yaml:"type"`     // "password" or "userpass"
		Password string `yaml:"password"` // for password auth
	} `yaml:"auth"`

	Obfs struct {
		Type       string `yaml:"type"` // "salamander"
		Salamander struct {
			Password string `yaml:"password"`
		} `yaml:"salamander"`
	} `yaml:"obfs"`

	Masquerade struct {
		Type string `yaml:"type"`
	} `yaml:"masquerade"`
}

// Client represents the Hysteria2 configuration client
type Client struct {
	logger *logger.Logger

	// Configuration
	enabled    bool
	configPath string
	nodeName   string
	serverAddr string // External server address (domain or IP)
	insecure   bool   // Whether to skip TLS verification
}

// NewClient creates a new Hysteria2 configuration client
func NewClient(logger *logger.Logger) *Client {
	return &Client{
		logger:     logger,
		enabled:    false,
		configPath: "/etc/hysteria/config.yaml",
		nodeName:   "Hysteria2",
		insecure:   false,
	}
}

// Configure sets up the Hysteria2 client with the given parameters
func (c *Client) Configure(enabled bool, configPath, nodeName, serverAddr string, insecure bool) {
	c.enabled = enabled
	if configPath != "" {
		c.configPath = configPath
	}
	if nodeName != "" {
		c.nodeName = nodeName
	}
	c.serverAddr = serverAddr
	c.insecure = insecure
}

// IsEnabled returns whether Hysteria2 support is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// ParseConfig parses the Hysteria2 configuration file
func (c *Client) ParseConfig() (*Hysteria2Config, error) {
	if !c.enabled {
		return nil, fmt.Errorf("hysteria2 support is not enabled")
	}

	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hysteria2 config file %s: %w", c.configPath, err)
	}

	var config Hysteria2Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse hysteria2 config: %w", err)
	}

	return &config, nil
}

// BuildURI constructs a hysteria2:// share link from the configuration
func (c *Client) BuildURI(config *Hysteria2Config) (string, error) {
	if config == nil {
		return "", fmt.Errorf("config is nil")
	}

	// Extract port from listen address
	port := "443" // default
	if config.Listen != "" {
		parts := strings.Split(config.Listen, ":")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			if lastPart != "" {
				port = lastPart
			}
		}
	}

	// Determine server address
	serverAddr := c.serverAddr
	if serverAddr == "" {
		return "", fmt.Errorf("server address is not configured")
	}

	// Build base URI: hysteria2://[auth]@[host]:[port]/
	var uriBuilder strings.Builder
	uriBuilder.WriteString("hysteria2://")

	// Add authentication
	if config.Auth.Password != "" {
		uriBuilder.WriteString(url.PathEscape(config.Auth.Password))
		uriBuilder.WriteString("@")
	}

	// Add host and port
	uriBuilder.WriteString(serverAddr)
	uriBuilder.WriteString(":")
	uriBuilder.WriteString(port)
	uriBuilder.WriteString("/")

	// Build query parameters
	params := url.Values{}

	// Add insecure flag if needed
	if c.insecure {
		params.Set("insecure", "1")
	}

	// Add obfuscation settings
	if config.Obfs.Type != "" {
		params.Set("obfs", config.Obfs.Type)
		if config.Obfs.Type == "salamander" && config.Obfs.Salamander.Password != "" {
			params.Set("obfs-password", config.Obfs.Salamander.Password)
		}
	}

	// Add SNI (use serverAddr as SNI if it's a domain)
	if !isIPAddress(serverAddr) {
		params.Set("sni", serverAddr)
	}

	// Add query string
	if len(params) > 0 {
		uriBuilder.WriteString("?")
		uriBuilder.WriteString(params.Encode())
	}

	// Add node name as fragment
	uriBuilder.WriteString("#")
	uriBuilder.WriteString(url.PathEscape(c.nodeName))

	return uriBuilder.String(), nil
}

// GetNodeConfig returns the Hysteria2 node as a base64-encoded string
// suitable for inclusion in subscription data
func (c *Client) GetNodeConfig() (string, error) {
	config, err := c.ParseConfig()
	if err != nil {
		c.logger.Errorf("Failed to parse Hysteria2 config: %v", err)
		return "", err
	}

	uri, err := c.BuildURI(config)
	if err != nil {
		c.logger.Errorf("Failed to build Hysteria2 URI: %v", err)
		return "", err
	}

	c.logger.Infof("🚀 Generated Hysteria2 node: %s", c.nodeName)
	c.logger.Debugf("   URI: %s", uri)

	// Return base64-encoded URI for consistency with other subscription data
	return base64.StdEncoding.EncodeToString([]byte(uri)), nil
}

// GetNodeConfigRaw returns the raw Hysteria2 URI (not base64-encoded)
func (c *Client) GetNodeConfigRaw() (string, error) {
	config, err := c.ParseConfig()
	if err != nil {
		return "", err
	}
	return c.BuildURI(config)
}

// isIPAddress checks if the given string is an IP address
func isIPAddress(s string) bool {
	// Simple check: if it contains only digits and dots, it's likely an IPv4
	// If it starts with [ or contains :, it's likely an IPv6
	for _, r := range s {
		if r != '.' && (r < '0' || r > '9') && r != ':' && r != '[' && r != ']' {
			return false
		}
	}
	return true
}
