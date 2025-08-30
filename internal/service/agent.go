package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"xhub-agent/internal/auth"
	"xhub-agent/internal/config"
	"xhub-agent/internal/monitor"
	"xhub-agent/internal/report"
	"xhub-agent/pkg/logger"
)

// AgentService main Agent service
type AgentService struct {
	config        *config.Config
	logger        *logger.Logger
	authClient    *auth.XUIAuth
	monitorClient *monitor.MonitorClient
	reportClient  *report.ReportClient

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	runningMux sync.RWMutex
}

// NewAgentService creates a new Agent service
func NewAgentService(configPath, logFile string) (*AgentService, error) {
	// Load configuration
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create logger
	log, err := logger.NewLogger(logFile, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create authentication client
	authClient := auth.NewXUIAuth(cfg.GetFullXUIURL(), cfg.XUIUser, cfg.XUIPass)

	// Create monitoring client
	monitorClient := monitor.NewMonitorClient(authClient)

	// Create report client using gRPC server and port
	grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPCServer, cfg.GRPCPort)
	reportClient := report.NewReportClient(grpcAddr, cfg.XHubAPIKey, log)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	return &AgentService{
		config:        cfg,
		logger:        log,
		authClient:    authClient,
		monitorClient: monitorClient,
		reportClient:  reportClient,
		ctx:           ctx,
		cancel:        cancel,
	}, nil
}

// Start starts the Agent service
func (a *AgentService) Start() {
	a.runningMux.Lock()
	if a.running {
		a.runningMux.Unlock()
		return
	}
	a.running = true
	a.runningMux.Unlock()

	a.logger.Info("🚀 Starting xhub-agent service")
	a.logger.Infof("🆔 Agent UUID: %s", a.config.UUID)
	a.logger.Infof("⏱️  Poll interval: %d seconds", a.config.PollInterval)

	// Debug: Log detailed configuration
	a.logger.Debugf("📋 Configuration Details:")
	a.logger.Debugf("   🌐 3x-ui URL: %s", a.config.GetFullXUIURL())
	a.logger.Debugf("   👤 3x-ui User: %s", a.config.XUIUser)
	a.logger.Debugf("   📡 gRPC Server: %s:%d", a.config.GRPCServer, a.config.GRPCPort)
	a.logger.Debugf("   🔑 API Key: %s", a.config.XHubAPIKey)
	a.logger.Debugf("   📊 Log Level: %s", a.config.LogLevel)

	// Start main work loop
	a.wg.Add(1)
	go a.workLoop()

	// Wait for all goroutines to complete
	a.wg.Wait()
	a.logger.Info("🛑 xhub-agent service stopped")
}

// Stop stops the Agent service
func (a *AgentService) Stop() {
	a.runningMux.Lock()
	if !a.running {
		a.runningMux.Unlock()
		return
	}
	a.running = false
	a.runningMux.Unlock()

	a.logger.Info("Stopping xhub-agent service...")
	a.cancel()
}

// Close closes the Agent service and cleans up resources
func (a *AgentService) Close() {
	a.Stop()

	// Close gRPC connection
	if a.reportClient != nil {
		if err := a.reportClient.Close(); err != nil {
			a.logger.Errorf("Failed to close gRPC connection: %v", err)
		}
	}

	if a.logger != nil {
		a.logger.Close()
	}
}

// IsRunning checks if the service is running
func (a *AgentService) IsRunning() bool {
	a.runningMux.RLock()
	defer a.runningMux.RUnlock()
	return a.running
}

// workLoop main work loop
func (a *AgentService) workLoop() {
	defer a.wg.Done()

	// Create ticker
	ticker := time.NewTicker(time.Duration(a.config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Execute immediately once
	a.executeOnce()

	// Execute periodically
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.executeOnce()
		}
	}
}

// executeOnce executes one complete monitoring and reporting cycle
func (a *AgentService) executeOnce() {
	a.logger.Debug("🔄 Starting monitoring and reporting cycle")
	a.logger.Debugf("   🎯 Target gRPC server: %s:%d", a.config.GRPCServer, a.config.GRPCPort)
	a.logger.Debugf("   🆔 Agent UUID: %s", a.config.UUID)

	// Check authentication status, re-login if needed
	if err := a.ensureAuthenticated(); err != nil {
		a.logger.Errorf("❌ Authentication failed: %v", err)
		return
	}

	// Get server status
	a.logger.Debug("📊 Requesting server status from 3x-ui...")
	status, err := a.monitorClient.GetServerStatus()
	if err != nil {
		a.logger.Errorf("❌ Failed to get server status: %v", err)

		// If it's an authentication error, clear auth status for re-login in next cycle
		if isAuthError(err) {
			a.logger.Warn("🔑 Detected authentication error, will re-login in next cycle")
		}
		return
	}

	a.logger.Debug("✅ Successfully retrieved server status from 3x-ui")

	// Print data to be reported
	if statusJSON, err := json.MarshalIndent(status.Data, "", "  "); err == nil {
		a.logger.Debugf("📋 Data to be reported via gRPC: %s", string(statusJSON))
	}

	// Report data to xhub
	a.logger.Debug("📡 Sending data to xhub via gRPC...")
	if err := a.reportClient.SendReport(a.config.UUID, status.Data); err != nil {
		a.logger.Errorf("❌ Failed to report data via gRPC: %v", err)
		a.logger.Errorf("   🎯 Server: %s:%d", a.config.GRPCServer, a.config.GRPCPort)
		a.logger.Errorf("   🆔 UUID: %s", a.config.UUID)
		return
	}

	a.logger.Debug("✅ Successfully reported data to xhub via gRPC")
}

// ensureAuthenticated ensures authentication, attempts login if not authenticated
func (a *AgentService) ensureAuthenticated() error {
	// Check if re-authentication is needed
	if !a.authClient.IsAuthenticated() || a.authClient.IsSessionExpired() {
		a.logger.Info("Logging into 3x-ui...")

		if err := a.authClient.Login(); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		a.logger.Info("Successfully logged into 3x-ui")
	}

	return nil
}

// isAuthError checks if the error is authentication-related
func isAuthError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()
	return contains(errorMsg, "unauthenticated") ||
		contains(errorMsg, "session") ||
		contains(errorMsg, "authentication") ||
		contains(errorMsg, "unauthorized")
}

// contains checks if string contains substring (case insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

// containsSubstring checks if it contains substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
