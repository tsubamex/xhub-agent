package hysteria2

import (
	"strings"
	"testing"
)

func TestBuildURI(t *testing.T) {
	// Create a mock logger (nil is acceptable for tests since we don't call logging methods)
	client := &Client{
		enabled:    true,
		configPath: "/etc/hysteria/config.yaml",
		nodeName:   "TestNode",
		serverAddr: "example.com",
		insecure:   true,
	}

	// Test case 1: Basic configuration with password auth
	config := &Hysteria2Config{
		Listen: ":443",
	}
	config.Auth.Password = "testpassword"

	uri, err := client.BuildURI(config)
	if err != nil {
		t.Fatalf("BuildURI failed: %v", err)
	}

	// Verify URI contains expected components
	if !strings.HasPrefix(uri, "hysteria2://") {
		t.Errorf("URI should start with hysteria2://, got: %s", uri)
	}
	if !strings.Contains(uri, "testpassword@") {
		t.Errorf("URI should contain password, got: %s", uri)
	}
	if !strings.Contains(uri, "example.com:443") {
		t.Errorf("URI should contain host:port, got: %s", uri)
	}
	if !strings.Contains(uri, "insecure=1") {
		t.Errorf("URI should contain insecure=1 when insecure is true, got: %s", uri)
	}
	if !strings.Contains(uri, "#TestNode") {
		t.Errorf("URI should contain node name as fragment, got: %s", uri)
	}
	if !strings.Contains(uri, "sni=example.com") {
		t.Errorf("URI should contain SNI for domain, got: %s", uri)
	}

	t.Logf("Generated URI: %s", uri)
}

func TestBuildURIWithObfs(t *testing.T) {
	client := &Client{
		enabled:    true,
		nodeName:   "ObfsNode",
		serverAddr: "test.example.com",
		insecure:   false,
	}

	config := &Hysteria2Config{
		Listen: ":8443",
	}
	config.Auth.Password = "mypass"
	config.Obfs.Type = "salamander"
	config.Obfs.Salamander.Password = "obfspass"

	uri, err := client.BuildURI(config)
	if err != nil {
		t.Fatalf("BuildURI failed: %v", err)
	}

	if !strings.Contains(uri, "test.example.com:8443") {
		t.Errorf("URI should contain correct host:port, got: %s", uri)
	}
	if !strings.Contains(uri, "obfs=salamander") {
		t.Errorf("URI should contain obfs type, got: %s", uri)
	}
	if !strings.Contains(uri, "obfs-password=obfspass") {
		t.Errorf("URI should contain obfs password, got: %s", uri)
	}
	// insecure should not be present when false
	if strings.Contains(uri, "insecure=1") {
		t.Errorf("URI should not contain insecure=1 when insecure is false, got: %s", uri)
	}

	t.Logf("Generated URI with obfs: %s", uri)
}

func TestBuildURIWithIPAddress(t *testing.T) {
	client := &Client{
		enabled:    true,
		nodeName:   "IPNode",
		serverAddr: "192.168.1.1",
		insecure:   true,
	}

	config := &Hysteria2Config{
		Listen: ":443",
	}
	config.Auth.Password = "pass"

	uri, err := client.BuildURI(config)
	if err != nil {
		t.Fatalf("BuildURI failed: %v", err)
	}

	// SNI should not be set for IP addresses
	if strings.Contains(uri, "sni=") {
		t.Errorf("URI should not contain SNI for IP address, got: %s", uri)
	}

	t.Logf("Generated URI for IP: %s", uri)
}

func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"example.com", false},
		{"test-server.example.com", false},
		{"my_server", false},
		{"[::1]", true},
		// Note: IPv6 addresses without brackets containing hex letters are detected as domains
		// This is acceptable as hysteria2 URIs typically use brackets for IPv6
	}

	for _, tt := range tests {
		result := isIPAddress(tt.input)
		if result != tt.expected {
			t.Errorf("isIPAddress(%s) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestBuildURIWithPortHopping(t *testing.T) {
	client := &Client{
		enabled:          true,
		nodeName:         "PortHoppingNode",
		serverAddr:       "example.com",
		insecure:         false,
		portHopping:      true,
		portHoppingRange: "20000-50000",
	}

	config := &Hysteria2Config{
		Listen: ":443",
	}
	config.Auth.Password = "testpass"

	uri, err := client.BuildURI(config)
	if err != nil {
		t.Fatalf("BuildURI failed: %v", err)
	}

	// Verify URI uses ports query parameter (Clash compatible format)
	if !strings.Contains(uri, "example.com:443/") {
		t.Errorf("URI should contain single port in host:port, got: %s", uri)
	}
	if !strings.Contains(uri, "ports=20000-50000") {
		t.Errorf("URI should contain ports query parameter, got: %s", uri)
	}
	if !strings.HasPrefix(uri, "hysteria2://") {
		t.Errorf("URI should start with hysteria2://, got: %s", uri)
	}
	if !strings.Contains(uri, "#PortHoppingNode") {
		t.Errorf("URI should contain node name, got: %s", uri)
	}

	t.Logf("Generated URI with port hopping: %s", uri)
}

func TestBuildURIWithPortHoppingColonFormat(t *testing.T) {
	// Test that colon format "20000:50000" is converted to dash format "20000-50000"
	client := &Client{
		enabled:          true,
		nodeName:         "ColonFormatNode",
		serverAddr:       "test.example.com",
		insecure:         true,
		portHopping:      true,
		portHoppingRange: "28299:60000", // colon format from config
	}

	config := &Hysteria2Config{
		Listen: ":28299",
	}
	config.Auth.Password = "mypass"

	uri, err := client.BuildURI(config)
	if err != nil {
		t.Fatalf("BuildURI failed: %v", err)
	}

	// Verify single port in host:port and ports query parameter
	if !strings.Contains(uri, "test.example.com:28299/") {
		t.Errorf("URI should contain single port in host:port, got: %s", uri)
	}
	// Verify colon is converted to dash in ports parameter
	if !strings.Contains(uri, "ports=28299-60000") {
		t.Errorf("URI should contain ports query parameter with dash format, got: %s", uri)
	}

	t.Logf("Generated URI with colon-to-dash conversion: %s", uri)
}
