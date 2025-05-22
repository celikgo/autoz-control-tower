# Multi-Cluster Manager Makefile
# This provides professional build, test, and distribution automation

# Project information
PROJECT_NAME := autoz-control-tower
BINARY_NAME := mcm
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go build information
GO_VERSION := $(shell go version | cut -d ' ' -f 3)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Build flags for embedding version information
LDFLAGS := -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME} -X main.GoVersion=${GO_VERSION}"

# Directories
BUILD_DIR := build
DIST_DIR := dist
DOCS_DIR := docs

# Colors for output formatting
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: help build test clean install docker lint fmt vet deps check release

# Default target
help: ## Display this help message
	@echo "$(BLUE)Multi-Cluster Manager Build System$(NC)"
	@echo "=================================="
	@echo ""
	@echo "$(GREEN)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Build Information:$(NC)"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(COMMIT)"
	@echo "  Go:      $(GO_VERSION)"
	@echo "  OS/Arch: $(GOOS)/$(GOARCH)"

build: ## Build the binary for current platform
	@echo "$(BLUE)Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/mcm
	@echo "$(GREEN)✓ Built $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

build-all: ## Build binaries for all supported platforms
	@echo "$(BLUE)Building $(BINARY_NAME) for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)

	# Linux builds
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/mcm
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/mcm

	# macOS builds
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/mcm
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/mcm

	# Windows builds
	@echo "Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/mcm

	@echo "$(GREEN)✓ Built all platform binaries in $(DIST_DIR)/$(NC)"

test: ## Run unit tests only (safe for CI)
	@echo "$(BLUE)Running unit tests...$(NC)"
	@SKIP_INTEGRATION_TESTS=true go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Unit tests completed$(NC)"

test-coverage: test ## Run tests and generate coverage report
	@echo "$(BLUE)Generating coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

benchmark: ## Run benchmarks
	@echo "$(BLUE)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

lint: ## Run linters
	@echo "$(BLUE)Running linters...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)" && exit 1)
	@golangci-lint run ./...
	@echo "$(GREEN)✓ Linting completed$(NC)"

fmt: ## Format Go code
	@echo "$(BLUE)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet completed$(NC)"

deps: ## Download and tidy dependencies
	@echo "$(BLUE)Managing dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

check: fmt vet lint test ## Run all checks (format, vet, lint, test)
	@echo "$(GREEN)✓ All checks passed$(NC)"

install: build ## Install binary to GOPATH/bin
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	@go install $(LDFLAGS) ./cmd/mcm
	@echo "$(GREEN)✓ Installed $(BINARY_NAME) to $(shell go env GOPATH)/bin/$(NC)"

docker: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	@docker build -t $(PROJECT_NAME):$(VERSION) .
	@docker tag $(PROJECT_NAME):$(VERSION) $(PROJECT_NAME):latest
	@echo "$(GREEN)✓ Built Docker images:$(NC)"
	@echo "  $(PROJECT_NAME):$(VERSION)"
	@echo "  $(PROJECT_NAME):latest"

docker-run: docker ## Build and run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	@docker run --rm -it \
		-v ~/.kube:/root/.kube:ro \
		-v $(PWD)/configs:/app/configs:ro \
		$(PROJECT_NAME):latest

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)✓ Cleaned$(NC)"

release: clean check build-all ## Create a release (clean, check, build all platforms)
	@echo "$(BLUE)Creating release $(VERSION)...$(NC)"
	@mkdir -p $(DIST_DIR)/release

	# Create archives for each platform
	@cd $(DIST_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(DIST_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd $(DIST_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(DIST_DIR) && tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	@cd $(DIST_DIR) && zip -q release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe

	# Generate checksums
	@cd $(DIST_DIR)/release && sha256sum * > checksums.txt

	@echo "$(GREEN)✓ Release $(VERSION) created in $(DIST_DIR)/release/$(NC)"
	@echo "$(GREEN)Release contents:$(NC)"
	@ls -la $(DIST_DIR)/release/

dev-setup: ## Set up development environment
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@go mod download
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)✓ Development environment ready$(NC)"
	@echo "$(YELLOW)Run 'make help' to see available commands$(NC)"

# Example development workflow
dev: ## Development workflow: format, test, build
	@echo "$(BLUE)Running development workflow...$(NC)"
	@$(MAKE) fmt
	@$(MAKE) test
	@$(MAKE) build
	@echo "$(GREEN)✓ Development workflow completed$(NC)"

# Quick start for new users
quickstart: ## Quick start: setup, build, and show usage
	@echo "$(BLUE)Multi-Cluster Manager Quick Start$(NC)"
	@echo "================================="
	@$(MAKE) dev-setup
	@$(MAKE) build
	@echo ""
	@echo "$(GREEN)✓ Setup complete! Try these commands:$(NC)"
	@echo "  ./$(BUILD_DIR)/$(BINARY_NAME) --help"
	@echo "  ./$(BUILD_DIR)/$(BINARY_NAME) config init"
	@echo "  ./$(BUILD_DIR)/$(BINARY_NAME) clusters list"

# Show project status
status: ## Show project and build status
	@echo "$(BLUE)Project Status$(NC)"
	@echo "=============="
	@echo "Project:     $(PROJECT_NAME)"
	@echo "Version:     $(VERSION)"
	@echo "Commit:      $(COMMIT)"
	@echo "Build Time:  $(BUILD_TIME)"
	@echo "Go Version:  $(GO_VERSION)"
	@echo "Platform:    $(GOOS)/$(GOARCH)"
	@echo ""
	@echo "$(GREEN)Dependencies:$(NC)"
	@go list -m all | head -10
	@echo ""
	@echo "$(GREEN)Build Status:$(NC)"
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		echo "✓ Binary exists: $(BUILD_DIR)/$(BINARY_NAME)"; \
		ls -lh $(BUILD_DIR)/$(BINARY_NAME); \
	else \
		echo "✗ Binary not built yet (run 'make build')"; \
	fi