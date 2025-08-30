package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogLevel represents log level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

const (
	// MaxLogFileSize maximum log file size in bytes (10MB)
	MaxLogFileSize = 10 * 1024 * 1024
)

// Logger represents a logger instance
type Logger struct {
	file     *os.File
	logger   *log.Logger
	level    LogLevel
	logFile  string
	fileSize int64
}

// NewLogger creates a new logger instance
func NewLogger(logFile, level string) (*Logger, error) {
	// Parse log level
	logLevel, err := parseLogLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %s", level)
	}

	// Create log directory
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Get current file size
	fileInfo, err := os.Stat(logFile)
	var currentSize int64 = 0
	if err == nil {
		currentSize = fileInfo.Size()
		// If file is too large, truncate it
		if currentSize > MaxLogFileSize {
			// Truncate the file (overwrite)
			file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to truncate log file: %w", err)
			}
			file.Close()
			currentSize = 0
		}
	}

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create logger that outputs to both file and stdout
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger := log.New(multiWriter, "", 0) // No default prefix, we format ourselves

	return &Logger{
		file:     file,
		logger:   logger,
		level:    logLevel,
		logFile:  logFile,
		fileSize: currentSize,
	}, nil
}

// parseLogLevel parses log level string
func parseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn", "warning":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return INFO, fmt.Errorf("unknown log level: %s", level)
	}
}

// String returns string representation of log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// log writes a log message
func (l *Logger) log(level LogLevel, message string) {
	// Check log level
	if level < l.level {
		return
	}

	// Format timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Build log message
	logMessage := fmt.Sprintf("[%s] [%s] %s", timestamp, level.String(), message)

	// Check file size before writing
	messageSize := int64(len(logMessage) + 1) // +1 for newline
	if l.fileSize+messageSize > MaxLogFileSize {
		l.truncateLogFile()
	}

	// Write log
	l.logger.Println(logMessage)
	l.fileSize += messageSize
}

// truncateLogFile truncates the log file when it becomes too large
func (l *Logger) truncateLogFile() {
	if l.file != nil {
		l.file.Close()
	}

	// Truncate the file (overwrite)
	file, err := os.OpenFile(l.logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// If we can't truncate, just continue with the old file
		return
	}

	// Create new multiwriter
	multiWriter := io.MultiWriter(file, os.Stdout)
	l.logger = log.New(multiWriter, "", 0)
	l.file = file
	l.fileSize = 0

	// Log truncation message
	truncateMsg := fmt.Sprintf("[%s] [INFO] Log file truncated due to size limit (%d bytes)",
		time.Now().Format("2006-01-02 15:04:05"), MaxLogFileSize)
	l.logger.Println(truncateMsg)
	l.fileSize = int64(len(truncateMsg) + 1)
}

// Debug logs a debug level message
func (l *Logger) Debug(message string) {
	l.log(DEBUG, message)
}

// Info logs an info level message
func (l *Logger) Info(message string) {
	l.log(INFO, message)
}

// Warn logs a warning level message
func (l *Logger) Warn(message string) {
	l.log(WARN, message)
}

// Error logs an error level message
func (l *Logger) Error(message string) {
	l.log(ERROR, message)
}

// Debugf logs a formatted debug level message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted info level message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning level message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error level message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Sync flushes the buffer
func (l *Logger) Sync() {
	if l.file != nil {
		l.file.Sync()
	}
}

// Close closes the logger
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
