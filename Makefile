# xhub-agent Makefile

# Variable definitions
APP_NAME := xhub-agent
VERSION := 1.0.0
BUILD_DIR := bin
MAIN_FILE := cmd/main.go

# Go related variables
GOOS ?= linux
GOARCH ?= amd64
GO_BUILD_FLAGS := -ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: clean test build

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@go clean -testcache

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests（简短输出）
.PHONY: test-short
test-short:
	@echo "Running tests (short output)..."
	@go test ./...

# Build application (current platform)
.PHONY: build
build:
	@echo "Building application (current platform)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)

# Build Linux version
.PHONY: build-linux
build-linux:
	@echo "Building Linux version..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 $(MAIN_FILE)

# Build multi-platform versions
.PHONY: build-all
build-all: build-linux build-linux-arm64 build-darwin

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building Linux ARM64 version..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=arm64 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)_linux_arm64 $(MAIN_FILE)

.PHONY: build-darwin
build-darwin:
	@echo "Building MacOS version..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)_darwin_amd64 $(MAIN_FILE)

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Check code
.PHONY: vet
vet:
	@echo "Checking code..."
	@go vet ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Development mode: format, check, test, build
.PHONY: dev
dev: fmt vet test build

# Create release package
.PHONY: release
release: clean test build-all
	@echo "Creating release package..."
	@cd $(BUILD_DIR) && \
	for binary in $(APP_NAME)_*; do \
		tar -czf "$${binary}.tar.gz" "$$binary"; \
	done

# Show help information
.PHONY: help
help:
	@echo "Available make targets:"
	@echo "  all        - Clean, test, build (default)"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  test-short - Run tests (short output)"
	@echo "  build      - Build application (current platform)"
	@echo "  build-linux - Build Linux version"
	@echo "  build-all  - Build all platform versions"
	@echo "  fmt        - Format code"
	@echo "  vet        - Check code"
	@echo "  tidy       - Tidy dependencies"
	@echo "  dev        - Development mode (format, check, test, build)"
	@echo "  release    - Create release package"
	@echo "  help       - Show this help information"
