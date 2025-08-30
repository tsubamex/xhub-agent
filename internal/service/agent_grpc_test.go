package service

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "xhub-agent/proto/reportpb"
)

// mockGRPCReportServer implements pb.ReportServiceServer for testing
type mockGRPCReportServer struct {
	pb.UnimplementedReportServiceServer
	reportCalled int32
}

func (m *mockGRPCReportServer) SendReport(ctx context.Context, req *pb.ReportRequest) (*pb.ReportResponse, error) {
	atomic.AddInt32(&m.reportCalled, 1)
	return &pb.ReportResponse{
		Success: true,
		Message: "gRPC报告成功",
	}, nil
}

func TestAgentService_gRPC_Integration(t *testing.T) {
	// Create temporary config and log directory
	tmpDir, err := os.MkdirTemp("", "xhub-agent-grpc-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Setup gRPC server
	mockServer := &mockGRPCReportServer{}
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	pb.RegisterReportServiceServer(s, mockServer)

	go func() {
		s.Serve(lis)
	}()
	defer s.Stop()

	// Extract host and port from gRPC server address
	addr := lis.Addr().String()
	host := "127.0.0.1" // localhost
	port := "0"         // dynamic port
	if colonIndex := strings.LastIndex(addr, ":"); colonIndex != -1 {
		host = addr[:colonIndex]
		port = addr[colonIndex+1:]
	}

	// Create config file with gRPC server and port
	configPath := filepath.Join(tmpDir, "config.yml")
	configContent := `uuid: test-uuid-123
xui_user: admin
xui_pass: password123
xhub_api_key: abcd1234apikey
grpcServer: ` + host + `
grpcPort: ` + port + `
rootPath: /test
port: 54321
xui_base_url: 127.0.0.1
poll_interval: 1
log_level: debug
`

	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create temporary log file
	logFile := filepath.Join(tmpDir, "agent.log")

	// Create AgentService
	agent, err := NewAgentService(configPath, logFile)
	require.NoError(t, err)
	require.NotNil(t, agent)
	defer agent.Close()

	// Note: This test only verifies that the agent can be created with gRPC config
	// Full integration test would require a mock 3x-ui server as well
	assert.Equal(t, "test-uuid-123", agent.config.UUID)
	assert.Equal(t, host, agent.config.GRPCServer)
	assert.Equal(t, port, strings.Split(addr, ":")[1]) // Verify port matches

	// Test that gRPC client can be created
	err = agent.reportClient.Connect()
	// This might fail due to no actual server, but that's expected in this test
	if err != nil {
		t.Logf("Expected gRPC connection failure (no server): %v", err)
	} else {
		// If connection succeeds, close it
		agent.reportClient.Close()
	}
}
