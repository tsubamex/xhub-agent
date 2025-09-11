package monitor

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"xhub-agent/internal/auth"
	"xhub-agent/pkg/logger"
)

// createTestLogger creates a logger for testing
func createTestLogger(t *testing.T) *logger.Logger {
	// Create temporary log file
	tmpDir, err := os.MkdirTemp("", "monitor-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	logFile := filepath.Join(tmpDir, "test.log")
	log, err := logger.NewLogger(logFile, "debug")
	require.NoError(t, err)

	return log
}

func TestMonitorClient_GetServerStatus_Success(t *testing.T) {
	// 模拟3x-ui服务器状态API
	expectedResponse := `{
		"success": true,
		"msg": "",
		"obj": {
			"cpu": 4.020100500542959,
			"cpuCores": 1,
			"logicalPro": 1,
			"cpuSpeedMhz": 1996.249,
			"mem": {
				"current": 131829760,
				"total": 498667520
			},
			"swap": {
				"current": 0,
				"total": 0
			},
			"disk": {
				"current": 1316593664,
				"total": 29416628224
			},
			"xray": {
				"state": "running",
				"errorMsg": "",
				"version": "25.8.3"
			},
			"uptime": 269583,
			"loads": [0.08, 0.02, 0],
			"tcpCount": 412,
			"udpCount": 5,
			"netIO": {
				"up": 94682,
				"down": 101147
			},
			"netTraffic": {
				"sent": 14558825693,
				"recv": 15208860756
			},
			"publicIP": {
				"ipv4": "31.57.172.16",
				"ipv6": "2a12:bec0:689:1154::"
			},
			"appStats": {
				"threads": 73,
				"mem": 53761288,
				"uptime": 226013
			}
		}
	}`

	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和路径
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/server/status", r.URL.Path)

		// 验证认证cookie
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "test-session-token", cookie.Value)

		// 返回服务器状态
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedResponse))
	}))
	defer server.Close()

	// 创建认证客户端并模拟登录
	authClient := auth.NewXUIAuth(server.URL, "admin", "password123")
	// 手动设置session token（跳过实际登录）
	authClient.SetSessionForTesting("test-session-token")

	// 创建测试logger和监控客户端
	testLogger := createTestLogger(t)
	defer testLogger.Close()
	monitorClient := NewMonitorClient(authClient, testLogger)

	// GetServerStatus gets server status
	status, err := monitorClient.GetServerStatus()
	require.NoError(t, err)

	// 验证返回的数据
	assert.NotNil(t, status)
	assert.True(t, status.Success)
	assert.NotNil(t, status.Data)
	assert.Equal(t, 4.020100500542959, status.Data.CPU)
	assert.Equal(t, 1, status.Data.CPUCores)
	assert.Equal(t, 1, status.Data.LogicalPro)
	assert.Equal(t, 1996.249, status.Data.CPUSpeedMhz)
	assert.Equal(t, int64(131829760), status.Data.Memory.Current)
	assert.Equal(t, int64(498667520), status.Data.Memory.Total)
	assert.Equal(t, int64(0), status.Data.Swap.Current)
	assert.Equal(t, int64(0), status.Data.Swap.Total)
	assert.Equal(t, int64(1316593664), status.Data.Disk.Current)
	assert.Equal(t, int64(29416628224), status.Data.Disk.Total)
	assert.Equal(t, 269583, status.Data.Uptime)
	assert.Len(t, status.Data.Loads, 3)
	assert.Equal(t, 0.08, status.Data.Loads[0])
	assert.Equal(t, 412, status.Data.TCPCount)
	assert.Equal(t, 5, status.Data.UDPCount)
	assert.Equal(t, int64(94682), status.Data.NetIO.Up)
	assert.Equal(t, int64(101147), status.Data.NetIO.Down)
	assert.Equal(t, int64(14558825693), status.Data.NetTraffic.Sent)
	assert.Equal(t, int64(15208860756), status.Data.NetTraffic.Recv)
	assert.Equal(t, "31.57.172.16", status.Data.PublicIP.IPv4)
	assert.Equal(t, "2a12:bec0:689:1154::", status.Data.PublicIP.IPv6)
	assert.Equal(t, "running", status.Data.Xray.State)
	assert.Equal(t, "", status.Data.Xray.ErrorMsg)
	assert.Equal(t, "25.8.3", status.Data.Xray.Version)
	assert.Equal(t, 73, status.Data.AppStats.Threads)
	assert.Equal(t, int64(53761288), status.Data.AppStats.Memory)
	assert.Equal(t, 226013, status.Data.AppStats.Uptime)
}

func TestMonitorClient_GetServerStatus_AuthenticationError(t *testing.T) {
	// 模拟认证失败的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success": false, "msg": "未授权"}`))
	}))
	defer server.Close()

	// 创建未认证的客户端
	authClient := auth.NewXUIAuth(server.URL, "admin", "password123")
	testLogger := createTestLogger(t)
	defer testLogger.Close()
	monitorClient := NewMonitorClient(authClient, testLogger)

	// 尝试获取服务器状态
	_, err := monitorClient.GetServerStatus()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}

func TestMonitorClient_GetServerStatus_ServerError(t *testing.T) {
	// 模拟服务器错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// 创建认证客户端
	authClient := auth.NewXUIAuth(server.URL, "admin", "password123")
	authClient.SetSessionForTesting("test-session-token")

	testLogger := createTestLogger(t)
	defer testLogger.Close()
	monitorClient := NewMonitorClient(authClient, testLogger)

	// 尝试获取服务器状态
	_, err := monitorClient.GetServerStatus()
	assert.Error(t, err)
}

func TestMonitorClient_GetServerStatus_InvalidJSON(t *testing.T) {
	// 模拟返回无效JSON的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json content`))
	}))
	defer server.Close()

	// 创建认证客户端
	authClient := auth.NewXUIAuth(server.URL, "admin", "password123")
	authClient.SetSessionForTesting("test-session-token")

	testLogger := createTestLogger(t)
	defer testLogger.Close()
	monitorClient := NewMonitorClient(authClient, testLogger)

	// 尝试获取服务器状态
	_, err := monitorClient.GetServerStatus()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestMonitorClient_GetServerStatus_APIError(t *testing.T) {
	// 模拟API返回错误的情况
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": false, "msg": "获取服务器状态失败"}`))
	}))
	defer server.Close()

	// 创建认证客户端
	authClient := auth.NewXUIAuth(server.URL, "admin", "password123")
	authClient.SetSessionForTesting("test-session-token")

	testLogger := createTestLogger(t)
	defer testLogger.Close()
	monitorClient := NewMonitorClient(authClient, testLogger)

	// 尝试获取服务器状态
	_, err := monitorClient.GetServerStatus()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "获取服务器状态失败")
}
