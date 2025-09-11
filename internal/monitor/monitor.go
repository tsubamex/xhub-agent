package monitor

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"xhub-agent/internal/auth"
	"xhub-agent/pkg/logger"
)

// MonitorClient monitoring data client
type MonitorClient struct {
	auth   *auth.XUIAuth
	client *http.Client
	logger *logger.Logger
}

// ServerStatusResponse server status response structure
type ServerStatusResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"msg"`
	Data    *ServerStatusData `json:"obj"`
}

// ServerStatusData server status data
type ServerStatusData struct {
	CPU         float64      `json:"cpu"`         // CPU usage rate
	CPUCores    int          `json:"cpuCores"`    // CPU core count
	LogicalPro  int          `json:"logicalPro"`  // Logical processor count
	CPUSpeedMhz float64      `json:"cpuSpeedMhz"` // CPU frequency (MHz)
	Memory      MemoryInfo   `json:"mem"`         // Memory information
	Swap        SwapInfo     `json:"swap"`        // Swap space information
	Disk        DiskInfo     `json:"disk"`        // Disk information
	Uptime      int          `json:"uptime"`      // Uptime (seconds)
	Loads       []float64    `json:"loads"`       // System load
	TCPCount    int          `json:"tcpCount"`    // TCP connection count
	UDPCount    int          `json:"udpCount"`    // UDP connection count
	NetIO       NetIOInfo    `json:"netIO"`       // Network IO
	NetTraffic  NetTraffic   `json:"netTraffic"`  // Network traffic
	PublicIP    PublicIPInfo `json:"publicIP"`    // Public IP information
	Xray        XrayInfo     `json:"xray"`        // Xray status
	AppStats    AppStats     `json:"appStats"`    // Application status
}

// MemoryInfo memory information
type MemoryInfo struct {
	Current int64 `json:"current"` // Current memory usage
	Total   int64 `json:"total"`   // Total memory
}

// SwapInfo swap space information
type SwapInfo struct {
	Current int64 `json:"current"` // Current swap usage
	Total   int64 `json:"total"`   // Total swap space
}

// DiskInfo disk information
type DiskInfo struct {
	Current int64 `json:"current"` // Current disk usage
	Total   int64 `json:"total"`   // Total disk space
}

// NetIOInfo network IO information
type NetIOInfo struct {
	Up   int64 `json:"up"`   // Upload traffic
	Down int64 `json:"down"` // Download traffic
}

// NetTraffic network traffic information
type NetTraffic struct {
	Sent int64 `json:"sent"` // Sent traffic
	Recv int64 `json:"recv"` // Received traffic
}

// XrayInfo Xray status information
type XrayInfo struct {
	State    string `json:"state"`    // Running status ("running" or other)
	ErrorMsg string `json:"errorMsg"` // Error message
	Version  string `json:"version"`  // Version
}

// PublicIPInfo public IP information
type PublicIPInfo struct {
	IPv4 string `json:"ipv4"` // IPv4 address
	IPv6 string `json:"ipv6"` // IPv6 address
}

// AppStats application status information
type AppStats struct {
	Threads int   `json:"threads"` // Thread count
	Memory  int64 `json:"mem"`     // Application memory usage
	Uptime  int   `json:"uptime"`  // Application uptime
}

// OnlineUsersResponse online users API response structure
type OnlineUsersResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"msg"`
	Data    []string `json:"obj"` // Array of online user emails
}

// NewMonitorClient creates a new monitoring client
func NewMonitorClient(authClient *auth.XUIAuth, logger *logger.Logger) *MonitorClient {
	return &MonitorClient{
		auth:   authClient,
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second, // 30 second timeout
			// Skip HTTPS certificate verification (since 3x-ui usually uses self-signed certificates)
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// GetServerStatus gets server status
func (m *MonitorClient) GetServerStatus() (*ServerStatusResponse, error) {
	// Check authentication status
	if !m.auth.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated, please login first")
	}

	// Create authenticated request
	req, err := m.auth.GetAuthenticatedRequest("POST", "/server/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request server status: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("not authenticated, session may have expired")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, HTTP status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Print raw response body for debugging
	m.logger.Debugf("3x-ui server status response body: %s", string(body))

	// Parse response
	var statusResp ServerStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if API response is successful
	if !statusResp.Success {
		return nil, fmt.Errorf("API error: %s", statusResp.Message)
	}

	return &statusResp, nil
}

// GetOnlineUsers gets online users from 3x-ui panel
func (m *MonitorClient) GetOnlineUsers() (*OnlineUsersResponse, error) {
	// Check authentication status
	if !m.auth.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated, please login first")
	}

	// Create authenticated request
	req, err := m.auth.GetAuthenticatedRequest("POST", "/panel/inbound/onlines", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers for this specific API
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	// Send request
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request online users: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("not authenticated, session may have expired")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, HTTP status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Print raw response body for debugging
	m.logger.Debugf("3x-ui online users response body: %s", string(body))

	// Parse response
	var onlineResp OnlineUsersResponse
	if err := json.Unmarshal(body, &onlineResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if API response is successful
	if !onlineResp.Success {
		return nil, fmt.Errorf("API error: %s", onlineResp.Message)
	}

	return &onlineResp, nil
}
