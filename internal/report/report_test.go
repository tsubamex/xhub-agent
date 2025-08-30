package report

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"xhub-agent/pkg/logger"
)

// createTestLogger creates a logger for testing
func createTestLogger(t *testing.T) *logger.Logger {
	tmpDir, err := os.MkdirTemp("", "xhub-agent-report-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	logFile := filepath.Join(tmpDir, "test.log")
	testLogger, err := logger.NewLogger(logFile, "debug")
	require.NoError(t, err)
	t.Cleanup(func() { testLogger.Close() })

	return testLogger
}

// Legacy HTTP tests are now replaced by gRPC tests in grpc_test.go
// This file is kept for the createTestLogger utility function
