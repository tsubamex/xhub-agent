package report

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"xhub-agent/internal/monitor"
	pb "xhub-agent/proto/reportpb"
)

// mockReportServer implements pb.ReportServiceServer for testing
type mockReportServer struct {
	pb.UnimplementedReportServiceServer
	receivedRequests []*pb.ReportRequest
	response         *pb.ReportResponse
	shouldError      codes.Code
}

func (m *mockReportServer) SendReport(ctx context.Context, req *pb.ReportRequest) (*pb.ReportResponse, error) {
	m.receivedRequests = append(m.receivedRequests, req)

	if m.shouldError != codes.OK {
		return nil, status.Error(m.shouldError, "test error")
	}

	if m.response != nil {
		return m.response, nil
	}

	return &pb.ReportResponse{
		Success: true,
		Message: "test success",
	}, nil
}

// setupGRPCTestServer creates a test gRPC server
func setupGRPCTestServer(t *testing.T, mock *mockReportServer) (string, func()) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	pb.RegisterReportServiceServer(s, mock)

	go func() {
		s.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		s.Stop()
	}
}

func TestReportClient_TLS_Security_Enforcement(t *testing.T) {
	mockLogger := createTestLogger(t)

	// Test 1: Localhost should allow disabling TLS
	t.Run("Localhost_AllowInsecure", func(t *testing.T) {
		client := NewReportClient("localhost:9090", "test-key", mockLogger)
		assert.False(t, client.IsTLSEnabled(), "Localhost should default to insecure")

		// Should allow enabling TLS for localhost
		client.SetTLS(true)
		assert.True(t, client.IsTLSEnabled(), "Should allow enabling TLS for localhost")

		// Should allow disabling TLS for localhost
		client.SetTLS(false)
		assert.False(t, client.IsTLSEnabled(), "Should allow disabling TLS for localhost")
	})

	// Test 2: Production server should enforce TLS
	t.Run("Production_EnforceTLS", func(t *testing.T) {
		client := NewReportClient("api.example.com:9090", "test-key", mockLogger)
		assert.True(t, client.IsTLSEnabled(), "Production server should default to TLS")

		// Should allow keeping TLS enabled
		client.SetTLS(true)
		assert.True(t, client.IsTLSEnabled(), "Should allow keeping TLS enabled")

		// Should PREVENT disabling TLS for production
		client.SetTLS(false)
		assert.True(t, client.IsTLSEnabled(), "Should PREVENT disabling TLS for production server")
	})

	// Test 3: Security info should be accurate
	t.Run("SecurityInfo", func(t *testing.T) {
		// Local server
		localClient := NewReportClient("127.0.0.1:9090", "test-key", mockLogger)
		localInfo := localClient.GetSecurityInfo()
		assert.False(t, localInfo["tls_enabled"].(bool))
		assert.True(t, localInfo["is_local"].(bool))
		assert.Equal(t, "OK - Local development", localInfo["recommendation"].(string))

		// Production server
		prodClient := NewReportClient("grpc.production.com:443", "test-key", mockLogger)
		prodInfo := prodClient.GetSecurityInfo()
		assert.True(t, prodInfo["tls_enabled"].(bool))
		assert.False(t, prodInfo["is_local"].(bool))
		assert.Equal(t, "SECURE - Production with TLS", prodInfo["recommendation"].(string))
	})
}

func TestReportClient_gRPC_SendReport_Success(t *testing.T) {
	// Create test logger
	testLogger := createTestLogger(t)

	// Setup mock server
	mockServer := &mockReportServer{
		response: &pb.ReportResponse{
			Success: true,
			Message: "报告成功",
		},
	}
	addr, cleanup := setupGRPCTestServer(t, mockServer)
	defer cleanup()

	// Create client
	client := NewReportClient(addr, "test-api-key", testLogger)
	defer client.Close()

	// Test data
	testData := &monitor.ServerStatusData{
		CPU: 25.5,
		Memory: monitor.MemoryInfo{
			Current: 1073741824,
			Total:   8589934592,
		},
		Disk: monitor.DiskInfo{
			Current: 53687091200,
			Total:   107374182400,
		},
		Uptime:   3600,
		Loads:    []float64{1.2, 1.5, 1.8},
		TCPCount: 150,
		UDPCount: 12,
		NetIO: monitor.NetIOInfo{
			Up:   1048576,
			Down: 2097152,
		},
		NetTraffic: monitor.NetTraffic{
			Sent: 10737418240,
			Recv: 21474836480,
		},
		PublicIP: monitor.PublicIPInfo{
			IPv4: "192.168.1.100",
			IPv6: "2001:db8::1",
		},
		Xray: monitor.XrayInfo{
			State:    "running",
			ErrorMsg: "",
			Version:  "1.8.0",
		},
		AppStats: monitor.AppStats{
			Threads: 25,
			Memory:  134217728,
			Uptime:  1800,
		},
	}

	// Send report
	err := client.SendReport("test-uuid-123", testData)

	// Verify
	assert.NoError(t, err)
	require.Len(t, mockServer.receivedRequests, 1)

	req := mockServer.receivedRequests[0]
	assert.Equal(t, "test-uuid-123", req.Uuid)
	assert.NotNil(t, req.Data)
	assert.Equal(t, float64(25.5), req.Data.Cpu)
	assert.Equal(t, int64(1073741824), req.Data.Memory.Current)
}

func TestReportClient_gRPC_SendReport_AuthenticationError(t *testing.T) {
	testLogger := createTestLogger(t)

	mockServer := &mockReportServer{
		shouldError: codes.Unauthenticated,
	}
	addr, cleanup := setupGRPCTestServer(t, mockServer)
	defer cleanup()

	client := NewReportClient(addr, "invalid-api-key", testLogger)
	defer client.Close()

	testData := &monitor.ServerStatusData{CPU: 10.0}

	err := client.SendReport("test-uuid-123", testData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
}

func TestReportClient_gRPC_SendReport_ServerError(t *testing.T) {
	testLogger := createTestLogger(t)

	mockServer := &mockReportServer{
		shouldError: codes.Internal,
	}
	addr, cleanup := setupGRPCTestServer(t, mockServer)
	defer cleanup()

	client := NewReportClient(addr, "test-api-key", testLogger)
	defer client.Close()

	testData := &monitor.ServerStatusData{CPU: 10.0}

	err := client.SendReport("test-uuid-123", testData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}

func TestReportClient_gRPC_Connection_Management(t *testing.T) {
	testLogger := createTestLogger(t)

	mockServer := &mockReportServer{}
	addr, cleanup := setupGRPCTestServer(t, mockServer)
	defer cleanup()

	client := NewReportClient(addr, "test-api-key", testLogger)

	// Test connect
	err := client.Connect()
	assert.NoError(t, err)
	assert.NotNil(t, client.conn)

	// Test close
	err = client.Close()
	assert.NoError(t, err)
	assert.Nil(t, client.conn)
}

func TestConvertToProto(t *testing.T) {
	// Test nil input
	result := ConvertToProto(nil)
	assert.Nil(t, result)

	// Test valid data conversion
	data := &monitor.ServerStatusData{
		CPU:         45.7,
		CPUCores:    8,
		LogicalPro:  16,
		CPUSpeedMhz: 3200.0,
		Memory: monitor.MemoryInfo{
			Current: 8589934592,
			Total:   17179869184,
		},
		Swap: monitor.SwapInfo{
			Current: 1073741824,
			Total:   4294967296,
		},
		Disk: monitor.DiskInfo{
			Current: 214748364800,
			Total:   1099511627776,
		},
		Uptime:   86400,
		Loads:    []float64{2.1, 2.3, 2.5},
		TCPCount: 500,
		UDPCount: 25,
		NetIO: monitor.NetIOInfo{
			Up:   5242880,
			Down: 10485760,
		},
		NetTraffic: monitor.NetTraffic{
			Sent: 53687091200,
			Recv: 107374182400,
		},
		PublicIP: monitor.PublicIPInfo{
			IPv4: "203.0.113.1",
			IPv6: "2001:db8:85a3::8a2e:370:7334",
		},
		Xray: monitor.XrayInfo{
			State:    "running",
			ErrorMsg: "",
			Version:  "1.8.1",
		},
		AppStats: monitor.AppStats{
			Threads: 32,
			Memory:  268435456,
			Uptime:  3600,
		},
	}

	pbData := ConvertToProto(data)
	require.NotNil(t, pbData)

	// Verify conversion
	assert.Equal(t, data.CPU, pbData.Cpu)
	assert.Equal(t, int32(data.CPUCores), pbData.CpuCores)
	assert.Equal(t, int32(data.LogicalPro), pbData.LogicalPro)
	assert.Equal(t, data.CPUSpeedMhz, pbData.CpuSpeedMhz)

	assert.Equal(t, data.Memory.Current, pbData.Memory.Current)
	assert.Equal(t, data.Memory.Total, pbData.Memory.Total)

	assert.Equal(t, data.Swap.Current, pbData.Swap.Current)
	assert.Equal(t, data.Swap.Total, pbData.Swap.Total)

	assert.Equal(t, data.Disk.Current, pbData.Disk.Current)
	assert.Equal(t, data.Disk.Total, pbData.Disk.Total)

	assert.Equal(t, int32(data.Uptime), pbData.Uptime)
	assert.Equal(t, data.Loads, pbData.Loads)
	assert.Equal(t, int32(data.TCPCount), pbData.TcpCount)
	assert.Equal(t, int32(data.UDPCount), pbData.UdpCount)

	assert.Equal(t, data.NetIO.Up, pbData.NetIo.Up)
	assert.Equal(t, data.NetIO.Down, pbData.NetIo.Down)

	assert.Equal(t, data.NetTraffic.Sent, pbData.NetTraffic.Sent)
	assert.Equal(t, data.NetTraffic.Recv, pbData.NetTraffic.Recv)

	assert.Equal(t, data.PublicIP.IPv4, pbData.PublicIp.Ipv4)
	assert.Equal(t, data.PublicIP.IPv6, pbData.PublicIp.Ipv6)

	assert.Equal(t, data.Xray.State, pbData.Xray.State)
	assert.Equal(t, data.Xray.ErrorMsg, pbData.Xray.ErrorMsg)
	assert.Equal(t, data.Xray.Version, pbData.Xray.Version)

	assert.Equal(t, int32(data.AppStats.Threads), pbData.AppStats.Threads)
	assert.Equal(t, data.AppStats.Memory, pbData.AppStats.Memory)
	assert.Equal(t, int32(data.AppStats.Uptime), pbData.AppStats.Uptime)
}
