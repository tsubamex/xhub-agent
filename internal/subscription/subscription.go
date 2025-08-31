package subscription

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xhub-agent/internal/auth"
)

// SubscriptionClient subscription information client
type SubscriptionClient struct {
	auth   *auth.XUIAuth
	client *http.Client
}

// DefaultSettingsResponse default settings response structure
type DefaultSettingsResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"msg"`
	Data    *SettingsData `json:"obj"`
}

// SettingsData settings data
type SettingsData struct {
	SubEnable  bool   `json:"subEnable"`
	SubURI     string `json:"subURI"`
	SubJsonURI string `json:"subJsonURI"`
}

// InboundListResponse inbound list response structure
type InboundListResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"msg"`
	Data    []*InboundInfo `json:"obj"`
}

// InboundInfo inbound information
type InboundInfo struct {
	ID       int    `json:"id"`
	Remark   string `json:"remark"`
	Enable   bool   `json:"enable"`
	Settings string `json:"settings"`
}

// ClientSettings client settings
type ClientSettings struct {
	Clients []ClientInfo `json:"clients"`
}

// ClientInfo client information
type ClientInfo struct {
	Email  string `json:"email"`
	SubID  string `json:"subId"`
	Enable bool   `json:"enable"`
}

// SubscriptionData subscription data
type SubscriptionData struct {
	SubID      string              `json:"subId"`
	Email      string              `json:"email"`
	NodeConfig string              `json:"nodeConfig"` // base64 encoded node configuration
	Headers    SubscriptionHeaders `json:"headers"`    // HTTP response headers
}

// SubscriptionHeaders HTTP response headers information
type SubscriptionHeaders struct {
	ProfileTitle          string `json:"profileTitle"`          // profile-title
	ProfileUpdateInterval string `json:"profileUpdateInterval"` // profile-update-interval
	SubscriptionUserinfo  string `json:"subscriptionUserinfo"`  // subscription-userinfo
}

// NewSubscriptionClient creates a new subscription client
func NewSubscriptionClient(authClient *auth.XUIAuth) *SubscriptionClient {
	return &SubscriptionClient{
		auth: authClient,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// GetDefaultSettings gets default settings
func (s *SubscriptionClient) GetDefaultSettings() (*SettingsData, error) {
	// Check authentication status
	if !s.auth.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated, please login first")
	}

	// Create authenticated request
	req, err := s.auth.GetAuthenticatedRequest("POST", "/panel/setting/defaultSettings", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request default settings: %w", err)
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

	// Parse response
	var settingsResp DefaultSettingsResponse
	if err := json.Unmarshal(body, &settingsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if API response is successful
	if !settingsResp.Success {
		return nil, fmt.Errorf("API error: %s", settingsResp.Message)
	}

	return settingsResp.Data, nil
}

// GetInboundList gets inbound list
func (s *SubscriptionClient) GetInboundList() ([]*InboundInfo, error) {
	// Check authentication status
	if !s.auth.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated, please login first")
	}

	// Create authenticated request
	req, err := s.auth.GetAuthenticatedRequest("POST", "/panel/inbound/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request inbound list: %w", err)
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

	// Parse response
	var inboundResp InboundListResponse
	if err := json.Unmarshal(body, &inboundResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if API response is successful
	if !inboundResp.Success {
		return nil, fmt.Errorf("API error: %s", inboundResp.Message)
	}

	return inboundResp.Data, nil
}

// ExtractUniqueSubIDs extracts unique SubIDs from inbound list
func (s *SubscriptionClient) ExtractUniqueSubIDs(inbounds []*InboundInfo) ([]SubscriptionData, error) {
	subIDMap := make(map[string]SubscriptionData) // Use map for deduplication

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue // Skip disabled inbound
		}

		// Parse settings JSON
		var settings ClientSettings
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue // Skip unparseable settings
		}

		// Extract subID for each client
		for _, client := range settings.Clients {
			if !client.Enable {
				// Debug: log skipped disabled client
				fmt.Printf("[DEBUG] Skipping disabled client: Email=%s, SubID=%s\n", client.Email, client.SubID)
				continue
			}

			if client.SubID == "" {
				// Debug: log skipped client without SubID
				fmt.Printf("[DEBUG] Skipping client without SubID: Email=%s\n", client.Email)
				continue
			}

			// Deduplication: only save first encountered subID
			if _, exists := subIDMap[client.SubID]; !exists {
				subIDMap[client.SubID] = SubscriptionData{
					SubID: client.SubID,
					Email: client.Email,
				}
				fmt.Printf("[DEBUG] Added active client: Email=%s, SubID=%s\n", client.Email, client.SubID)
			}
		}
	}

	// Convert to slice
	var result []SubscriptionData
	for _, data := range subIDMap {
		result = append(result, data)
	}

	return result, nil
}

// GetSubscriptionContent gets subscription content (base64 node configuration) and response headers
func (s *SubscriptionClient) GetSubscriptionContent(baseSubURL, subID string) (string, SubscriptionHeaders, error) {
	var headers SubscriptionHeaders

	// Build subscription URL - replace domain with 127.0.0.1
	subscriptionURL, err := s.replaceHostWithLocalhost(baseSubURL, subID)
	if err != nil {
		return "", headers, fmt.Errorf("failed to build subscription URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", subscriptionURL, nil)
	if err != nil {
		return "", headers, fmt.Errorf("failed to create subscription request: %w", err)
	}

	// Set User-Agent to simulate v2ray client
	req.Header.Set("User-Agent", "v2rayN/6.23")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", headers, fmt.Errorf("failed to request subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", headers, fmt.Errorf("subscription request failed, HTTP status code: %d", resp.StatusCode)
	}

	// Collect response headers
	headers.ProfileTitle = resp.Header.Get("profile-title")
	headers.ProfileUpdateInterval = resp.Header.Get("profile-update-interval")
	headers.SubscriptionUserinfo = resp.Header.Get("subscription-userinfo")

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", headers, fmt.Errorf("failed to read subscription response: %w", err)
	}

	// Response content should already be base64 encoded node configuration
	content := strings.TrimSpace(string(body))

	// If content is empty, return empty content directly (no error)
	if content == "" {
		return "", headers, nil
	}

	// Validate if it's valid base64
	if _, err := base64.StdEncoding.DecodeString(content); err != nil {
		return "", headers, fmt.Errorf("invalid base64 content in subscription response")
	}

	return content, headers, nil
}

// replaceHostWithLocalhost replaces the domain in subscription URL with 127.0.0.1
func (s *SubscriptionClient) replaceHostWithLocalhost(baseURL, subID string) (string, error) {
	// Parse base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Replace host with 127.0.0.1, keep port unchanged
	if parsedURL.Port() != "" {
		parsedURL.Host = "127.0.0.1:" + parsedURL.Port()
	} else {
		parsedURL.Host = "127.0.0.1"
	}

	// Append subID to the path
	finalURL := parsedURL.String()
	if !strings.HasSuffix(finalURL, "/") {
		finalURL += "/"
	}
	finalURL += subID

	return finalURL, nil
}

// GetAllSubscriptionData gets all subscription data
func (s *SubscriptionClient) GetAllSubscriptionData() ([]SubscriptionData, error) {
	// 1. Get default settings
	settings, err := s.GetDefaultSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get default settings: %w", err)
	}

	if !settings.SubEnable || settings.SubURI == "" {
		return nil, fmt.Errorf("subscription is not enabled or SubURI is empty")
	}

	// 2. Get inbound list
	inbounds, err := s.GetInboundList()
	if err != nil {
		return nil, fmt.Errorf("failed to get inbound list: %w", err)
	}

	// 3. Extract unique SubIDs
	subscriptions, err := s.ExtractUniqueSubIDs(inbounds)
	if err != nil {
		return nil, fmt.Errorf("failed to extract SubIDs: %w", err)
	}

	// 4. Get subscription content for each SubID
	var result []SubscriptionData
	for _, sub := range subscriptions {
		content, headers, err := s.GetSubscriptionContent(settings.SubURI, sub.SubID)
		if err != nil {
			// Log error but continue processing other subscriptions
			fmt.Printf("[WARNING] Failed to get subscription content for SubID %s: %v\n", sub.SubID, err)
			continue
		}

		// If content is empty, subscription service may be down, log warning but continue
		if content == "" {
			fmt.Printf("[WARNING] Empty subscription content for SubID %s (subscription service may be down), skipping\n", sub.SubID)
			continue
		}

		sub.NodeConfig = content
		sub.Headers = headers
		result = append(result, sub)
	}

	return result, nil
}
