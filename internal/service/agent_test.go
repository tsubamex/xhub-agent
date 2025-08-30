package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentService_NewAgentService(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "xhub-agent-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `uuid: test-uuid-123
xui_user: admin
xui_pass: password123
xhub_api_key: abcd1234apikey
grpcServer: xhub.example.com
grpcPort: 9090
rootPath: /test
port: 54321
xui_base_url: 127.0.0.1
poll_interval: 5
log_level: info
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create temporary log file
	logFile := filepath.Join(tmpDir, "agent.log")

	// Create AgentService
	agent, err := NewAgentService(configPath, logFile)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Verify configuration is loaded correctly
	assert.Equal(t, "test-uuid-123", agent.config.UUID)
	assert.Equal(t, 5, agent.config.PollInterval)

	// Close agent
	agent.Close()
}

func TestAgentService_Start_Stop(t *testing.T) {
	t.Skip("Integration test temporarily disabled during gRPC migration")
	// Create temporary config and log directory
	tmpDir, err := os.MkdirTemp("", "xhub-agent-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock server (with login and status endpoints)
	var loginCalled, statusCalled, reportCalled int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test/login":
			atomic.AddInt32(&loginCalled, 1)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "test-session"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "msg": ""}`))

		case "/test/server/status":
			atomic.AddInt32(&statusCalled, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"success": true,
				"msg": "",
				"obj": {
					"cpu": 4.020100500542959,
					"cpuCores": 1,
					"logicalPro": 1,
					"cpuSpeedMhz": 1996.249,
					"mem": {"current": 131829760, "total": 498667520},
					"swap": {"current": 0, "total": 0},
					"disk": {"current": 1316593664, "total": 29416628224},
					"uptime": 269583,
					"loads": [0.08, 0.02, 0],
					"tcpCount": 412,
					"udpCount": 5,
					"netIO": {"up": 94682, "down": 101147},
					"netTraffic": {"sent": 14558825693, "recv": 15208860756},
					"publicIP": {"ipv4": "31.57.172.16", "ipv6": "2a12:bec0:689:1154::"},
					"xray": {"state": "running", "errorMsg": "", "version": "25.8.3"},
					"appStats": {"threads": 73, "mem": 53761288, "uptime": 226013}
				}
			}`))
		}
	}))
	defer server.Close()

	// Create mock xhub server
	xhubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent/report/test-uuid-123" {
			atomic.AddInt32(&reportCalled, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "msg": "data reported successfully"}`))
		}
	}))
	defer xhubServer.Close()

	// Parse mock server URL
	serverURL, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(serverURL.Port())

	// Create config file
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := fmt.Sprintf(`uuid: test-uuid-123
serverId: server-001
xui_user: admin
xui_pass: password123
xhub_api_key: abcd1234apikey
reportUrl: %s/agent/report
rootPath: /test
port: %d
xui_base_url: %s
poll_interval: 1
log_level: debug
`, xhubServer.URL, port, serverURL.Hostname())

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logFile := filepath.Join(tmpDir, "agent.log")

	// Create and start AgentService
	agent, err := NewAgentService(configPath, logFile)
	require.NoError(t, err)

	// Start service
	go agent.Start()

	// Wait for several polling cycles
	time.Sleep(3 * time.Second)

	// Stop service
	agent.Stop()

	// Verify that all endpoints were called
	assert.Greater(t, atomic.LoadInt32(&loginCalled), int32(0), "should have called login endpoint")
	assert.Greater(t, atomic.LoadInt32(&statusCalled), int32(0), "should have called status endpoint")
	assert.Greater(t, atomic.LoadInt32(&reportCalled), int32(0), "should have called report endpoint")

	agent.Close()
}

func TestAgentService_InvalidConfig(t *testing.T) {
	// Test non-existent config file
	_, err := NewAgentService("/non/existent/config.yml", "/tmp/test.log")
	assert.Error(t, err)
}

func TestAgentService_AuthenticationFailure(t *testing.T) {
	t.Skip("Integration test temporarily disabled during gRPC migration")
	tmpDir, err := os.MkdirTemp("", "xhub-agent-service-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock server that returns authentication failure
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": false, "msg": "username or password incorrect"}`))
		}
	}))
	defer server.Close()

	// Parse mock server URL
	serverURL, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(serverURL.Port())

	// Create config file
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := fmt.Sprintf(`uuid: test-uuid-123
serverId: server-001
xui_user: admin
xui_pass: wrongpassword
xhub_api_key: abcd1234apikey
reportUrl: https://xhub.example.com/agent/report
rootPath: /
port: %d
xui_base_url: %s
poll_interval: 1
log_level: debug
`, port, serverURL.Hostname())

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logFile := filepath.Join(tmpDir, "agent.log")

	// Create AgentService
	agent, err := NewAgentService(configPath, logFile)
	require.NoError(t, err)
	defer agent.Close()

	// Start service (should continue retrying after authentication failure)
	go agent.Start()

	// Wait a bit to ensure enough time for authentication attempts
	time.Sleep(2 * time.Second)

	agent.Stop()

	// Verify log file contains authentication error information
	logContent, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "username or password incorrect")
}
