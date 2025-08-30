package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"xhub-agent/internal/service"
)

const (
	defaultConfigPath = "/opt/xhub-agent/config.yml"
	defaultLogPath    = "/opt/xhub-agent/logs/agent.log"
)

func main() {
	// Command line arguments
	var (
		configPath = flag.String("c", defaultConfigPath, "Config file path")
		logPath    = flag.String("l", defaultLogPath, "Log file path")
		version    = flag.Bool("v", false, "Show version information")
		help       = flag.Bool("h", false, "Show help information")
	)
	flag.Parse()

	// Show version information
	if *version {
		fmt.Println("xhub-agent v1.0.0")
		fmt.Println("A monitoring agent for 3x-ui servers")
		return
	}

	// Show help information
	if *help {
		fmt.Println("xhub-agent - 3x-ui monitoring agent")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  xhub-agent [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  xhub-agent -c /path/to/config.yml -l /path/to/agent.log")
		return
	}

	// Check if config file exists
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config file does not exist: %s\n", *configPath)
		fmt.Fprintf(os.Stderr, "Please ensure the config file exists, or use -c parameter to specify the correct config file path\n")
		os.Exit(1)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(*logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Create Agent service
	agent, err := service.NewAgentService(*configPath, *logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create Agent service: %v\n", err)
		os.Exit(1)
	}
	defer agent.Close()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start Agent service (in goroutine)
	go agent.Start()

	// Wait for signal
	sig := <-sigChan
	fmt.Printf("Received signal %v, gracefully shutting down...\n", sig)

	// Stop service
	agent.Stop()
}
