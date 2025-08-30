package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_LoadFromFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xhub-agent-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "test-config.yml")
	configContent := `uuid: test-uuid-123
xui_user: admin
xui_pass: password123
xhub_api_key: abcd1234apikey
grpcServer: localhost
grpcPort: 9090
rootPath: /wIqhNNPV3lC3ZzAHdd
port: 22799
xui_base_url: 127.0.0.1
poll_interval: 5
log_level: info
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// 测试加载配置文件
	config, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// 验证配置内容
	assert.Equal(t, "test-uuid-123", config.UUID)
	assert.Equal(t, "admin", config.XUIUser)
	assert.Equal(t, "password123", config.XUIPass)
	assert.Equal(t, "abcd1234apikey", config.XHubAPIKey)
	assert.Equal(t, "localhost", config.GRPCServer)
	assert.Equal(t, 9090, config.GRPCPort)
	assert.Equal(t, "/wIqhNNPV3lC3ZzAHdd", config.RootPath)
	assert.Equal(t, 22799, config.Port)
	assert.Equal(t, "127.0.0.1", config.XUIBaseURL)
	assert.Equal(t, 5, config.PollInterval)
	assert.Equal(t, "info", config.LogLevel)
}

func TestConfig_LoadFromFile_WithDefaults(t *testing.T) {
	// 创建临时目录和最小配置文件
	tmpDir, err := os.MkdirTemp("", "xhub-agent-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "minimal-config.yml")
	configContent := `uuid: test-uuid-123
xui_user: admin
xui_pass: password123
xhub_api_key: abcd1234apikey
grpcServer: xhub.example.com
grpcPort: 9090
rootPath: /wIqhNNPV3lC3ZzAHdd
port: 22799
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// 测试加载配置文件（应该应用默认值）
	config, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// 验证默认值
	assert.Equal(t, "127.0.0.1", config.XUIBaseURL)
	assert.Equal(t, 2, config.PollInterval) // gRPC 时代默认 2 秒
	assert.Equal(t, "info", config.LogLevel)
}

func TestConfig_LoadFromFile_NonExistentFile(t *testing.T) {
	_, err := LoadFromFile("/non/existent/file.yml")
	assert.Error(t, err)
}

func TestConfig_LoadFromFile_InvalidYAML(t *testing.T) {
	// 创建临时目录和无效YAML文件
	tmpDir, err := os.MkdirTemp("", "xhub-agent-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid-config.yml")
	invalidContent := `uuid: test-uuid-123
serverId: server-001
invalid yaml content [[[
`

	err = os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
}

func TestConfig_SmartGRPCPortDefaults(t *testing.T) {
	tests := []struct {
		name         string
		grpcServer   string
		grpcPort     int
		expectedPort int
		description  string
	}{
		{
			name:         "localhost_default_port",
			grpcServer:   "localhost",
			grpcPort:     0, // Not set
			expectedPort: 9090,
			description:  "Localhost should default to port 9090",
		},
		{
			name:         "127_0_0_1_default_port",
			grpcServer:   "127.0.0.1",
			grpcPort:     0, // Not set
			expectedPort: 9090,
			description:  "127.0.0.1 should default to port 9090",
		},
		{
			name:         "production_default_port",
			grpcServer:   "api.example.com",
			grpcPort:     0, // Not set
			expectedPort: 443,
			description:  "Production server should default to port 443",
		},
		{
			name:         "production_port_overridden",
			grpcServer:   "api.example.com",
			grpcPort:     9090, // Backend returned port
			expectedPort: 443,  // Should be overridden to 443
			description:  "Production server should override any port to 443",
		},
		{
			name:         "localhost_custom_port_preserved",
			grpcServer:   "localhost",
			grpcPort:     8080, // Custom port
			expectedPort: 8080,
			description:  "Custom localhost port should be preserved",
		},
		{
			name:         "domain_port_overridden",
			grpcServer:   "grpc.example.com",
			grpcPort:     9090, // Backend returned port
			expectedPort: 443,  // Should be overridden to 443
			description:  "Domain server should override backend port to 443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCServer: tt.grpcServer,
				GRPCPort:   tt.grpcPort,
				RootPath:   "/test",
				Port:       2053,
			}

			// Apply defaults
			config.applyDefaults()

			assert.Equal(t, tt.expectedPort, config.GRPCPort, tt.description)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCServer: "example.com",
				GRPCPort:   9090,
				RootPath:   "/wIqhNNPV3lC3ZzAHdd",
				Port:       22799,
				XUIBaseURL: "127.0.0.1",
			},
			wantErr: false,
		},
		{
			name: "missing UUID",
			config: Config{
				XUIUser:    "admin",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCServer: "example.com",
				GRPCPort:   9090,
			},
			wantErr: true,
		},
		{
			name: "missing XUIUser",
			config: Config{
				UUID:       "test-uuid",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCServer: "example.com",
				GRPCPort:   9090,
			},
			wantErr: true,
		},
		{
			name: "missing XUIPass",
			config: Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XHubAPIKey: "api-key",
				GRPCServer: "example.com",
				GRPCPort:   9090,
			},
			wantErr: true,
		},
		{
			name: "missing XHubAPIKey",
			config: Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XUIPass:    "password",
				GRPCServer: "example.com",
				GRPCPort:   9090,
			},
			wantErr: true,
		},
		{
			name: "missing GRPCServer",
			config: Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCPort:   9090,
			},
			wantErr: true,
		},
		{
			name: "invalid GRPCPort",
			config: Config{
				UUID:       "test-uuid",
				XUIUser:    "admin",
				XUIPass:    "password",
				XHubAPIKey: "api-key",
				GRPCServer: "example.com",
				GRPCPort:   0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
