package report

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"xhub-agent/internal/monitor"
	"xhub-agent/pkg/logger"
	pb "xhub-agent/proto/reportpb"
)

// ReportClient report data client
type ReportClient struct {
	serverAddr      string
	apiKey          string
	conn            *grpc.ClientConn
	client          pb.ReportServiceClient
	logger          *logger.Logger
	isConnected     bool      // track connection state to avoid repeated logs
	lastConnectTime time.Time // track last successful connection
	useTLS          bool      // whether to use TLS encryption
	// Error state tracking fields
	lastErrorState string // track last error state to avoid duplicate logs
	hasLoggedError bool   // track if error has been logged for current failure
	wasSuccessful  bool   // track if last operation was successful
}

// NewReportClient creates a new report client
func NewReportClient(serverAddr, apiKey string, log *logger.Logger) *ReportClient {
	// Validate gRPC server address format
	if strings.HasPrefix(serverAddr, "http://") || strings.HasPrefix(serverAddr, "https://") {
		log.Warnf("⚠️  gRPC server address contains HTTP protocol: %s", serverAddr)
		log.Warnf("   gRPC only supports 'host:port' format, not HTTP URLs")
		log.Warnf("   Example: 'localhost:9090' instead of 'http://localhost:8080/path'")
	}

	if strings.Contains(serverAddr, "/") && !strings.HasPrefix(serverAddr, "http") {
		log.Warnf("⚠️  gRPC server address contains path: %s", serverAddr)
		log.Warnf("   gRPC doesn't support URL paths, only 'host:port' format")
		log.Warnf("   Example: 'server.com:9090' instead of 'server.com:9090/api'")
	}

	// Auto-detect TLS usage based on common patterns
	useTLS := shouldUseTLS(serverAddr)

	return &ReportClient{
		serverAddr:    serverAddr,
		apiKey:        apiKey,
		logger:        log,
		useTLS:        useTLS,
		wasSuccessful: true, // assume success initially
	}
}

// shouldUseTLS determines if TLS should be used based on server address patterns
func shouldUseTLS(serverAddr string) bool {
	// Only allow insecure connections for local development
	if strings.Contains(serverAddr, "localhost") || strings.Contains(serverAddr, "127.0.0.1") {
		return false // Local development - use insecure
	}

	// For ALL remote servers, FORCE TLS encryption
	return true
}

// isLocalServer checks if the server is a local development server
func isLocalServer(serverAddr string) bool {
	return strings.Contains(serverAddr, "localhost") || strings.Contains(serverAddr, "127.0.0.1")
}

// SetTLS explicitly enables or disables TLS for the connection
// Note: TLS cannot be disabled for production (non-localhost) servers for security reasons
func (r *ReportClient) SetTLS(useTLS bool) {
	// Security check: prevent disabling TLS for production servers
	if !useTLS && !isLocalServer(r.serverAddr) {
		r.logger.Warnf("🚨 Security Warning: Cannot disable TLS for production server: %s", r.serverAddr)
		r.logger.Warnf("   TLS is MANDATORY for all non-localhost connections")
		r.logger.Warnf("   Keeping TLS enabled for security")
		return // Ignore the request to disable TLS
	}

	r.useTLS = useTLS
	// If connection already exists, it will be recreated on next use
	if r.conn != nil {
		r.logger.Debugf("TLS setting changed, will reconnect with %s",
			map[bool]string{true: "TLS enabled", false: "TLS disabled"}[useTLS])
		r.Close()
	}
}

// IsTLSEnabled returns whether TLS is currently enabled
func (r *ReportClient) IsTLSEnabled() bool {
	return r.useTLS
}

// GetSecurityInfo returns security information about the connection
func (r *ReportClient) GetSecurityInfo() map[string]interface{} {
	isLocal := isLocalServer(r.serverAddr)
	return map[string]interface{}{
		"server_address": r.serverAddr,
		"tls_enabled":    r.useTLS,
		"is_local":       isLocal,
		"security_level": map[bool]string{true: "SECURE (TLS)", false: "INSECURE (no TLS)"}[r.useTLS],
		"recommendation": func() string {
			if isLocal && !r.useTLS {
				return "OK - Local development"
			} else if !isLocal && r.useTLS {
				return "SECURE - Production with TLS"
			} else if !isLocal && !r.useTLS {
				return "⚠️ INSECURE - Production without TLS"
			} else {
				return "SECURE - Local with TLS"
			}
		}(),
	}
}

// extractHostname extracts hostname from server address for TLS ServerName
func (r *ReportClient) extractHostname() string {
	// Remove port if present
	if strings.Contains(r.serverAddr, ":") {
		host, _, err := net.SplitHostPort(r.serverAddr)
		if err == nil {
			return host
		}
	}
	return r.serverAddr
}

// Connect establishes gRPC connection
func (r *ReportClient) Connect() error {
	if r.conn != nil {
		r.logger.Debugf("gRPC connection already exists, reusing connection to: %s", r.serverAddr)
		return nil // Already connected
	}

	// Only log connection attempt if not recently connected
	if !r.isConnected || time.Since(r.lastConnectTime) > 5*time.Minute {
		r.logger.Infof("🔗 Attempting to establish gRPC connection...")
		r.logger.Debugf("📡 gRPC Server Address: %s", r.serverAddr)
		r.logger.Debugf("🔑 API Key: %s", r.apiKey)
		r.logger.Debugf("⏱️  Connection Timeout: 10 seconds")
		if r.useTLS {
			r.logger.Debugf("🔒 Transport: Secure (TLS enabled)")
		} else {
			r.logger.Debugf("⚠️  Transport: Insecure (no TLS)")
		}
	}

	r.logger.Debugf("🚀 Creating gRPC client connection to %s...", r.serverAddr)

	// Choose credentials based on TLS setting
	var creds credentials.TransportCredentials
	if r.useTLS {
		// Use TLS with system root CAs
		creds = credentials.NewTLS(&tls.Config{
			ServerName: r.extractHostname(),
		})
	} else {
		// Use insecure credentials for local development
		creds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(r.serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		r.isConnected = false

		// Only log detailed connection error if it should be logged (deduplication check)
		errorKey := fmt.Sprintf("connect_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ gRPC client creation failed!")
			r.logger.Errorf("   Server: %s", r.serverAddr)
			r.logger.Errorf("   Error: %v", err)
			r.logger.Errorf("   Please check:")
			r.logger.Errorf("   1. Server address format is correct (host:port)")
			r.logger.Errorf("   2. No HTTP protocol prefix in address")
			r.logger.Errorf("   3. Address doesn't contain URL paths")
		}
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Check initial connection state
	state := conn.GetState()
	r.logger.Debugf("📊 Initial connection state: %s", state)

	// Note: With grpc.NewClient, the connection is lazy and will be established on first RPC call
	if state == connectivity.Idle {
		r.logger.Debugf("🔄 Connection is idle (will connect on first RPC call)")
	}

	r.conn = conn
	r.client = pb.NewReportServiceClient(conn)

	// Only log success if not recently connected or first time
	if !r.isConnected || time.Since(r.lastConnectTime) > 5*time.Minute {
		r.logger.Infof("✅ Successfully connected to gRPC server: %s", r.serverAddr)
	}
	r.logger.Debugf("🔗 Connection state: %s", conn.GetState())

	r.isConnected = true
	r.lastConnectTime = time.Now()

	// Mark connection success
	r.markSuccess("gRPC连接")
	return nil
}

// Close closes the gRPC connection
func (r *ReportClient) Close() error {
	if r.conn != nil {
		r.logger.Debug("Closing gRPC connection")
		err := r.conn.Close()
		r.conn = nil
		r.client = nil
		r.isConnected = false
		return err
	}
	return nil
}

// shouldLogError determines if an error should be logged
// Returns true only for the first occurrence of an error state
func (r *ReportClient) shouldLogError(errorKey string) bool {
	// If this is a different error state, always log it
	if r.lastErrorState != errorKey {
		r.lastErrorState = errorKey
		r.hasLoggedError = true
		r.wasSuccessful = false
		return true
	}

	// If same error state and we haven't logged it yet, log it
	if !r.hasLoggedError {
		r.hasLoggedError = true
		r.wasSuccessful = false
		return true
	}

	// Don't log repeated errors
	return false
}

// markSuccess marks an operation as successful and logs recovery if needed
func (r *ReportClient) markSuccess(operationType string) {
	// If we were in error state, log the recovery
	if !r.wasSuccessful {
		r.logger.Infof("✅ %s 操作已恢复正常", operationType)
	}

	// Reset error state
	r.lastErrorState = ""
	r.hasLoggedError = false
	r.wasSuccessful = true
}

// SendReport sends monitoring data to xhub via gRPC
func (r *ReportClient) SendReport(uuid string, data *monitor.ServerStatusData) error {
	r.logger.Debugf("📊 Starting gRPC report transmission...")
	r.logger.Debugf("🆔 Agent UUID: %s", uuid)
	r.logger.Debugf("📡 Target Server: %s", r.serverAddr)

	// Ensure connection is established
	if err := r.Connect(); err != nil {
		return fmt.Errorf("failed to establish gRPC connection: %w", err)
	}

	// Convert monitor data to protobuf format
	r.logger.Debugf("🔄 Converting data to protobuf format...")
	pbData := ConvertToProto(data)
	if pbData == nil {
		r.logger.Errorf("❌ Failed to convert data to protobuf format")
		return fmt.Errorf("failed to convert data to protobuf format")
	}
	r.logger.Debugf("✅ Data conversion successful")

	// Create request
	req := &pb.ReportRequest{
		Uuid: uuid,
		Data: pbData,
	}
	r.logger.Debugf("📦 Created gRPC request with UUID: %s", uuid)

	// Create context with timeout and metadata for authentication
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add API key to metadata for authentication
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + r.apiKey,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Debug: Log detailed request information
	r.logger.Debugf("🚀 Sending gRPC request...")
	r.logger.Debugf("   🎯 Server: %s", r.serverAddr)
	r.logger.Debugf("   🆔 UUID: %s", uuid)
	r.logger.Debugf("   🔑 Auth: Bearer %s", r.apiKey)
	r.logger.Debugf("   ⏱️  Timeout: 30 seconds")
	r.logger.Debugf("   📊 Data: CPU=%.1f%%, Memory=%d/%d bytes",
		pbData.Cpu, pbData.Memory.Current, pbData.Memory.Total)

	// Send gRPC request
	resp, err := r.client.SendReport(ctx, req)
	if err != nil {
		// Create error key for deduplication
		var errorKey string
		var errorMsg string

		// Handle gRPC status errors
		if st, ok := status.FromError(err); ok {
			errorKey = fmt.Sprintf("grpc_%s_%s", st.Code(), r.serverAddr)

			switch st.Code() {
			case codes.Unauthenticated:
				errorMsg = "authentication failed: API key invalid or expired"
			case codes.InvalidArgument:
				errorMsg = fmt.Sprintf("request error: invalid data format - %s", st.Message())
			case codes.NotFound:
				errorMsg = fmt.Sprintf("API endpoint not found: check if UUID is registered - %s", st.Message())
			case codes.Internal:
				errorMsg = fmt.Sprintf("server error: %s", st.Message())
			case codes.DeadlineExceeded:
				errorMsg = fmt.Sprintf("request timeout: %s", st.Message())
			case codes.Unavailable:
				errorMsg = fmt.Sprintf("server unavailable: %s", st.Message())
			default:
				errorMsg = fmt.Sprintf("gRPC error [%s]: %s", st.Code(), st.Message())
			}

			// Only log detailed error if it should be logged (deduplication check)
			if r.shouldLogError(errorKey) {
				r.logger.Errorf("❌ gRPC request failed!")
				r.logger.Errorf("   Server: %s", r.serverAddr)
				r.logger.Errorf("   UUID: %s", uuid)
				r.logger.Errorf("   gRPC Status: %s", st.Code())
				r.logger.Errorf("   Error Message: %s", st.Message())

				switch st.Code() {
				case codes.Unauthenticated:
					r.logger.Errorf("   🔑 Authentication failed - check API key")
				case codes.InvalidArgument:
					r.logger.Errorf("   📊 Invalid data format")
				case codes.NotFound:
					r.logger.Errorf("   🔍 Endpoint or UUID not found")
				case codes.Internal:
					r.logger.Errorf("   🔥 Internal server error")
				case codes.DeadlineExceeded:
					r.logger.Errorf("   ⏰ Request timeout exceeded")
				case codes.Unavailable:
					r.logger.Errorf("   🚫 Server unavailable")
				default:
					r.logger.Errorf("   ❓ Unknown gRPC error")
				}
			}

			return fmt.Errorf(errorMsg)
		}

		// Handle non-gRPC errors
		errorKey = fmt.Sprintf("generic_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ gRPC request failed!")
			r.logger.Errorf("   Server: %s", r.serverAddr)
			r.logger.Errorf("   UUID: %s", uuid)
			r.logger.Errorf("   Raw error: %v", err)
		}
		return fmt.Errorf("gRPC request failed: %w", err)
	}

	// Debug: Log response details
	r.logger.Debugf("✅ gRPC response received")
	r.logger.Debugf("   📊 Success: %t", resp.Success)
	r.logger.Debugf("   💬 Message: %s", resp.Message)

	// Check response
	if !resp.Success {
		errorKey := fmt.Sprintf("server_reject_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ Server rejected the report: %s", resp.Message)
		}
		return fmt.Errorf("report failed: %s", resp.Message)
	}

	// Mark success and log recovery if needed
	r.markSuccess("监控数据上报")
	r.logger.Debugf("🎉 Data successfully reported via gRPC!")
	return nil
}

// SendSubscriptionReport sends subscription data to xhub via gRPC
func (r *ReportClient) SendSubscriptionReport(uuid string, subscriptions []SubscriptionData) error {
	r.logger.Debugf("📊 Starting gRPC subscription report transmission...")
	r.logger.Debugf("🆔 Agent UUID: %s", uuid)
	r.logger.Debugf("📡 Target Server: %s", r.serverAddr)
	r.logger.Debugf("📋 Subscription Count: %d", len(subscriptions))

	// Ensure connection is established
	if err := r.Connect(); err != nil {
		return fmt.Errorf("failed to establish gRPC connection: %w", err)
	}

	// Convert subscription data to protobuf format
	r.logger.Debugf("🔄 Converting subscription data to protobuf format...")
	var pbSubscriptions []*pb.SubscriptionData
	for _, sub := range subscriptions {
		pbHeaders := &pb.SubscriptionHeaders{
			ProfileTitle:          sub.Headers.ProfileTitle,
			ProfileUpdateInterval: sub.Headers.ProfileUpdateInterval,
			SubscriptionUserinfo:  sub.Headers.SubscriptionUserinfo,
		}

		pbSub := &pb.SubscriptionData{
			SubId:      sub.SubID,
			Email:      sub.Email,
			NodeConfig: sub.NodeConfig,
			Headers:    pbHeaders,
		}
		pbSubscriptions = append(pbSubscriptions, pbSub)
	}

	// Create request
	req := &pb.SubscriptionReportRequest{
		Uuid:          uuid,
		Subscriptions: pbSubscriptions,
	}
	r.logger.Debugf("📦 Created gRPC subscription request with UUID: %s", uuid)

	// Create context with timeout and metadata for authentication
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add API key to metadata for authentication
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + r.apiKey,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Debug: Log detailed request information
	r.logger.Debugf("🚀 Sending gRPC subscription request...")
	r.logger.Debugf("   🎯 Server: %s", r.serverAddr)
	r.logger.Debugf("   🆔 UUID: %s", uuid)
	r.logger.Debugf("   🔑 Auth: Bearer %s", r.apiKey)
	r.logger.Debugf("   ⏱️  Timeout: 30 seconds")
	r.logger.Debugf("   📋 Subscriptions: %d items", len(pbSubscriptions))

	// Send gRPC request
	resp, err := r.client.SendSubscriptionReport(ctx, req)
	if err != nil {
		// Create error key for deduplication
		var errorKey string
		var errorMsg string

		// Handle gRPC status errors
		if st, ok := status.FromError(err); ok {
			errorKey = fmt.Sprintf("grpc_sub_%s_%s", st.Code(), r.serverAddr)

			switch st.Code() {
			case codes.Unauthenticated:
				errorMsg = "authentication failed: API key invalid or expired"
			case codes.InvalidArgument:
				errorMsg = fmt.Sprintf("request error: invalid subscription data format - %s", st.Message())
			case codes.NotFound:
				errorMsg = fmt.Sprintf("API endpoint not found: check if UUID is registered - %s", st.Message())
			case codes.Internal:
				errorMsg = fmt.Sprintf("server error: %s", st.Message())
			case codes.DeadlineExceeded:
				errorMsg = fmt.Sprintf("request timeout: %s", st.Message())
			case codes.Unavailable:
				errorMsg = fmt.Sprintf("server unavailable: %s", st.Message())
			default:
				errorMsg = fmt.Sprintf("gRPC error [%s]: %s", st.Code(), st.Message())
			}

			// Only log detailed error if it should be logged (deduplication check)
			if r.shouldLogError(errorKey) {
				r.logger.Errorf("❌ gRPC subscription request failed!")
				r.logger.Errorf("   Server: %s", r.serverAddr)
				r.logger.Errorf("   UUID: %s", uuid)
				r.logger.Errorf("   gRPC Status: %s", st.Code())
				r.logger.Errorf("   Error Message: %s", st.Message())

				switch st.Code() {
				case codes.Unauthenticated:
					r.logger.Errorf("   🔑 Authentication failed - check API key")
				case codes.InvalidArgument:
					r.logger.Errorf("   📊 Invalid subscription data format")
				case codes.NotFound:
					r.logger.Errorf("   🔍 Endpoint or UUID not found")
				case codes.Internal:
					r.logger.Errorf("   🔥 Internal server error")
				case codes.DeadlineExceeded:
					r.logger.Errorf("   ⏰ Request timeout exceeded")
				case codes.Unavailable:
					r.logger.Errorf("   🚫 Server unavailable")
				default:
					r.logger.Errorf("   ❓ Unknown gRPC error")
				}
			}

			return fmt.Errorf(errorMsg)
		}

		// Handle non-gRPC errors
		errorKey = fmt.Sprintf("generic_sub_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ gRPC subscription request failed!")
			r.logger.Errorf("   Server: %s", r.serverAddr)
			r.logger.Errorf("   UUID: %s", uuid)
			r.logger.Errorf("   Raw error: %v", err)
		}
		return fmt.Errorf("gRPC subscription request failed: %w", err)
	}

	// Debug: Log response details
	r.logger.Debugf("✅ gRPC subscription response received")
	r.logger.Debugf("   📊 Success: %t", resp.Success)
	r.logger.Debugf("   💬 Message: %s", resp.Message)

	// Check response
	if !resp.Success {
		errorKey := fmt.Sprintf("server_reject_sub_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ Server rejected the subscription report: %s", resp.Message)
		}
		return fmt.Errorf("subscription report failed: %s", resp.Message)
	}

	// Mark success and log recovery if needed
	r.markSuccess("订阅数据上报")
	r.logger.Debugf("🎉 Subscription data successfully reported via gRPC!")
	return nil
}

// SendOnlineUsersReport sends online users data to xhub via gRPC
func (r *ReportClient) SendOnlineUsersReport(uuid string, onlineEmails []string) error {
	r.logger.Debugf("📊 Starting gRPC online users report transmission...")
	r.logger.Debugf("🆔 Agent UUID: %s", uuid)
	r.logger.Debugf("📡 Target Server: %s", r.serverAddr)
	r.logger.Debugf("👥 Online Users Count: %d", len(onlineEmails))

	// Ensure connection is established
	if err := r.Connect(); err != nil {
		return fmt.Errorf("failed to establish gRPC connection: %w", err)
	}

	// Create request
	req := &pb.OnlineUsersReportRequest{
		Uuid:         uuid,
		OnlineEmails: onlineEmails,
	}
	r.logger.Debugf("📦 Created gRPC online users request with UUID: %s", uuid)

	// Create context with timeout and metadata for authentication
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add API key to metadata for authentication
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + r.apiKey,
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Debug: Log detailed request information
	r.logger.Debugf("🚀 Sending gRPC online users request...")
	r.logger.Debugf("   🎯 Server: %s", r.serverAddr)
	r.logger.Debugf("   🆔 UUID: %s", uuid)
	r.logger.Debugf("   🔑 Auth: Bearer %s", r.apiKey)
	r.logger.Debugf("   ⏱️  Timeout: 30 seconds")
	if len(onlineEmails) > 0 {
		r.logger.Debugf("   👥 Online Users: %v", onlineEmails)
	} else {
		r.logger.Debugf("   👥 Online Users: (empty - no users online)")
	}

	// Send gRPC request
	resp, err := r.client.SendOnlineUsersReport(ctx, req)
	if err != nil {
		// Create error key for deduplication
		var errorKey string
		var errorMsg string

		// Handle gRPC status errors
		if st, ok := status.FromError(err); ok {
			errorKey = fmt.Sprintf("grpc_users_%s_%s", st.Code(), r.serverAddr)

			switch st.Code() {
			case codes.Unauthenticated:
				errorMsg = "authentication failed: API key invalid or expired"
			case codes.InvalidArgument:
				errorMsg = fmt.Sprintf("request error: invalid online users data format - %s", st.Message())
			case codes.NotFound:
				errorMsg = fmt.Sprintf("API endpoint not found: check if UUID is registered - %s", st.Message())
			case codes.Internal:
				errorMsg = fmt.Sprintf("server error: %s", st.Message())
			case codes.DeadlineExceeded:
				errorMsg = fmt.Sprintf("request timeout: %s", st.Message())
			case codes.Unavailable:
				errorMsg = fmt.Sprintf("server unavailable: %s", st.Message())
			default:
				errorMsg = fmt.Sprintf("gRPC error [%s]: %s", st.Code(), st.Message())
			}

			// Only log detailed error if it should be logged (deduplication check)
			if r.shouldLogError(errorKey) {
				r.logger.Errorf("❌ gRPC online users request failed!")
				r.logger.Errorf("   Server: %s", r.serverAddr)
				r.logger.Errorf("   UUID: %s", uuid)
				r.logger.Errorf("   gRPC Status: %s", st.Code())
				r.logger.Errorf("   Error Message: %s", st.Message())

				switch st.Code() {
				case codes.Unauthenticated:
					r.logger.Errorf("   🔑 Authentication failed - check API key")
				case codes.InvalidArgument:
					r.logger.Errorf("   📊 Invalid online users data format")
				case codes.NotFound:
					r.logger.Errorf("   🔍 Endpoint or UUID not found")
				case codes.Internal:
					r.logger.Errorf("   🔥 Internal server error")
				case codes.DeadlineExceeded:
					r.logger.Errorf("   ⏰ Request timeout exceeded")
				case codes.Unavailable:
					r.logger.Errorf("   🚫 Server unavailable")
				default:
					r.logger.Errorf("   ❓ Unknown gRPC error")
				}
			}

			return fmt.Errorf(errorMsg)
		}

		// Handle non-gRPC errors
		errorKey = fmt.Sprintf("generic_users_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ gRPC online users request failed!")
			r.logger.Errorf("   Server: %s", r.serverAddr)
			r.logger.Errorf("   UUID: %s", uuid)
			r.logger.Errorf("   Raw error: %v", err)
		}
		return fmt.Errorf("gRPC online users request failed: %w", err)
	}

	// Debug: Log response details
	r.logger.Debugf("✅ gRPC online users response received")
	r.logger.Debugf("   📊 Success: %t", resp.Success)
	r.logger.Debugf("   💬 Message: %s", resp.Message)

	// Check response
	if !resp.Success {
		errorKey := fmt.Sprintf("server_reject_users_%s", r.serverAddr)
		if r.shouldLogError(errorKey) {
			r.logger.Errorf("❌ Server rejected the online users report: %s", resp.Message)
		}
		return fmt.Errorf("online users report failed: %s", resp.Message)
	}

	// Mark success and log recovery if needed
	r.markSuccess("在线用户上报")
	r.logger.Debugf("🎉 Online users data successfully reported via gRPC!")
	return nil
}

// SubscriptionData represents subscription information for reporting
type SubscriptionData struct {
	SubID      string              `json:"subId"`
	Email      string              `json:"email"`
	NodeConfig string              `json:"nodeConfig"` // base64编码的节点配置
	Headers    SubscriptionHeaders `json:"headers"`    // HTTP响应头
}

// SubscriptionHeaders HTTP响应头信息
type SubscriptionHeaders struct {
	ProfileTitle          string `json:"profileTitle"`          // profile-title
	ProfileUpdateInterval string `json:"profileUpdateInterval"` // profile-update-interval
	SubscriptionUserinfo  string `json:"subscriptionUserinfo"`  // subscription-userinfo
}
