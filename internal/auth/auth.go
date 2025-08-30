package auth

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// XUIAuth 3x-ui authentication client
type XUIAuth struct {
	baseURL      string
	username     string
	password     string
	client       *http.Client
	sessionToken string
	cookieName   string // Store the actual cookie name used
	lastLogin    time.Time
	mutex        sync.RWMutex
}

// LoginResponse 3x-ui login response structure
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"msg"`
}

// NewXUIAuth creates a new 3x-ui authentication client
func NewXUIAuth(baseURL, username, password string) *XUIAuth {
	return &XUIAuth{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
			// Skip HTTPS certificate verification (since 3x-ui usually uses self-signed certificates)
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Login performs login operation
func (a *XUIAuth) Login() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Prepare login data
	data := url.Values{}
	data.Set("username", a.username)
	data.Set("password", a.password)

	// Create login request
	req, err := http.NewRequest("POST", a.baseURL+"/login", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed, HTTP status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Parse response
	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	// Check if login was successful
	if !loginResp.Success {
		return fmt.Errorf("login failed: %s", loginResp.Message)
	}

	// Extract session cookie - first try to parse from Set-Cookie header
	setCookies := resp.Header.Values("Set-Cookie")
	for _, setCookie := range setCookies {
		if sessionToken, cookieName := extractSessionFromSetCookie(setCookie); sessionToken != "" {
			a.sessionToken = sessionToken
			a.cookieName = cookieName
			a.lastLogin = time.Now()
			break
		}
	}

	// If Set-Cookie method didn't work, try standard method
	if a.sessionToken == "" {
		for _, cookie := range resp.Cookies() {
			// Check for common session cookie names
			if cookie.Name == "3x-ui" || cookie.Name == "session" {
				a.sessionToken = cookie.Value
				a.cookieName = cookie.Name
				a.lastLogin = time.Now()
				break
			}
		}
	}

	if a.sessionToken == "" {
		// Add debug information
		allHeaders := ""
		for name, values := range resp.Header {
			for _, value := range values {
				allHeaders += fmt.Sprintf("%s: %s; ", name, value)
			}
		}
		return fmt.Errorf("login successful but failed to get session cookie. Response headers: %s", allHeaders)
	}

	return nil
}

// RefreshSession refreshes session
func (a *XUIAuth) RefreshSession() error {
	return a.Login()
}

// IsAuthenticated checks if authenticated
func (a *XUIAuth) IsAuthenticated() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.sessionToken != ""
}

// IsSessionExpired checks if session is expired
func (a *XUIAuth) IsSessionExpired() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if a.sessionToken == "" {
		return true
	}

	// Assume session is valid for 1 hour
	return time.Since(a.lastLogin) > time.Hour
}

// GetSessionToken gets session token
func (a *XUIAuth) GetSessionToken() string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.sessionToken
}

// GetAuthenticatedRequest creates HTTP request with authentication info
func (a *XUIAuth) GetAuthenticatedRequest(method, path string, body io.Reader) (*http.Request, error) {
	a.mutex.RLock()
	sessionToken := a.sessionToken
	cookieName := a.cookieName
	a.mutex.RUnlock()

	if sessionToken == "" {
		return nil, fmt.Errorf("not authenticated, please login first")
	}

	// Create request
	req, err := http.NewRequest(method, a.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add session cookie with the correct name
	if cookieName == "" {
		cookieName = "3x-ui" // Default fallback
	}
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: sessionToken,
	})

	return req, nil
}

// SetSessionForTesting sets session token (for testing only)
func (a *XUIAuth) SetSessionForTesting(token string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.sessionToken = token
	a.cookieName = "session" // Default for testing
	a.lastLogin = time.Now()
}

// extractSessionFromSetCookie extracts session token from Set-Cookie header
func extractSessionFromSetCookie(setCookie string) (string, string) {
	// Set-Cookie may contain multiple cookies, separated by semicolons
	cookies := strings.Split(setCookie, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		// Check for common session cookie names
		if strings.HasPrefix(cookie, "3x-ui=") {
			parts := strings.SplitN(cookie, "=", 2)
			if len(parts) == 2 {
				return parts[1], "3x-ui"
			}
		} else if strings.HasPrefix(cookie, "session=") {
			parts := strings.SplitN(cookie, "=", 2)
			if len(parts) == 2 {
				return parts[1], "session"
			}
		}
	}
	return "", ""
}
