package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_NewLogger(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "xhub-agent-logger-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")

	// Create logger
	logger, err := NewLogger(logFile, "info")
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Verify that log file was created
	_, err = os.Stat(logFile)
	assert.NoError(t, err)

	// Close logger
	logger.Close()
}

func TestLogger_LogLevels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xhub-agent-logger-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")

	// Test different log levels
	tests := []struct {
		level string
		valid bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"error", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger, err := NewLogger(logFile, tt.level)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
				if logger != nil {
					logger.Close()
				}
			} else {
				assert.Error(t, err)
				assert.Nil(t, logger)
			}
		})
	}
}

func TestLogger_WriteMessages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xhub-agent-logger-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")

	// Create logger
	logger, err := NewLogger(logFile, "debug")
	require.NoError(t, err)
	defer logger.Close()

	// Write different level logs
	logger.Debug("This is a debug message")
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")

	// Force flush buffer
	logger.Sync()

	// Read log file content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	// Verify log content
	logContent := string(content)
	assert.Contains(t, logContent, "This is a debug message")
	assert.Contains(t, logContent, "This is an info message")
	assert.Contains(t, logContent, "This is a warning message")
	assert.Contains(t, logContent, "This is an error message")
	assert.Contains(t, logContent, "[DEBUG]")
	assert.Contains(t, logContent, "[INFO]")
	assert.Contains(t, logContent, "[WARN]")
	assert.Contains(t, logContent, "[ERROR]")
}

func TestLogger_LevelFiltering(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "xhub-agent-logger-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logFile := filepath.Join(tmpDir, "test.log")

	// Create warn level logger
	logger, err := NewLogger(logFile, "warn")
	require.NoError(t, err)
	defer logger.Close()

	// Write different level logs
	logger.Debug("This is a debug message, should not appear")
	logger.Info("This is an info message, should not appear")
	logger.Warn("This is a warning message, should appear")
	logger.Error("This is an error message, should appear")

	// Force flush buffer
	logger.Sync()

	// Read log file content
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logContent := string(content)
	// debug and info level logs should not appear
	assert.NotContains(t, logContent, "This is a debug message, should not appear")
	assert.NotContains(t, logContent, "This is an info message, should not appear")
	// warn and error level logs should appear
	assert.Contains(t, logContent, "This is a warning message, should appear")
	assert.Contains(t, logContent, "This is an error message, should appear")
}

func TestLogger_InvalidLogFile(t *testing.T) {
	// Try to create log file in non-existent directory
	invalidPath := "/non/existent/directory/test.log"

	logger, err := NewLogger(invalidPath, "info")
	assert.Error(t, err)
	assert.Nil(t, logger)
}
