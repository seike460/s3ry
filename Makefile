# S3ry Makefile - Optimized Development Workflow
# Modern high-performance S3 CLI tool

# Variables
BINARY_NAME := s3ry
PACKAGE := github.com/seike460/s3ry
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Build parameters
BUILD_DIR := dist
MAIN_PATH := ./cmd/s3ry

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

# Help target
.PHONY: help
help: ## Show this help message
	@echo "$(BLUE)🚀 S3ry Development Commands$(NC)"
	@echo "=============================="
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(YELLOW)<target>$(NC)\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(BLUE)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: setup
setup: ## Setup development environment
	@echo "$(BLUE)🔧 Setting up development environment...$(NC)"
	$(GOMOD) download
	$(GOMOD) verify
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.62.2; \
	fi
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing govulncheck...$(NC)"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@echo "$(GREEN)✅ Development environment ready!$(NC)"

.PHONY: deps
deps: ## Download and verify dependencies
	@echo "$(BLUE)📦 Managing dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy
	@echo "$(GREEN)✅ Dependencies updated$(NC)"

##@ Building

.PHONY: build
build: ## Build the binary for current platform
	@echo "$(BLUE)🔨 Building $(BINARY_NAME)...$(NC)"
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

.PHONY: build-all
build-all: ## Build binaries for all platforms
	@echo "$(BLUE)🌐 Building for all platforms...$(NC)"
	mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	@echo "$(YELLOW)Building for Linux AMD64...$(NC)"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	
	# Linux ARM64
	@echo "$(YELLOW)Building for Linux ARM64...$(NC)"
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	
	# macOS AMD64
	@echo "$(YELLOW)Building for macOS AMD64...$(NC)"
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	
	# macOS ARM64
	@echo "$(YELLOW)Building for macOS ARM64...$(NC)"
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	# Windows AMD64
	@echo "$(YELLOW)Building for Windows AMD64...$(NC)"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	
	@echo "$(GREEN)✅ All builds completed!$(NC)"
	@ls -la $(BUILD_DIR)/

.PHONY: install
install: build ## Install the binary to GOPATH/bin
	@echo "$(BLUE)📦 Installing $(BINARY_NAME)...$(NC)"
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

##@ Testing

.PHONY: test
test: ## Run unit tests
	@echo "$(BLUE)🧪 Running unit tests...$(NC)"
	$(GOTEST) -v -race -timeout=10m ./...
	@echo "$(GREEN)✅ Unit tests completed$(NC)"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "$(BLUE)📊 Running tests with coverage...$(NC)"
	$(GOTEST) -v -race -timeout=10m -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	$(GOCMD) tool cover -func=coverage.out
	@echo "$(GREEN)✅ Coverage report generated: coverage.html$(NC)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(BLUE)🔗 Running integration tests...$(NC)"
	$(GOTEST) -v -timeout=15m -tags=integration ./test/integration/...
	@echo "$(GREEN)✅ Integration tests completed$(NC)"

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "$(BLUE)🎯 Running e2e tests...$(NC)"
	RUN_E2E_TESTS=1 $(GOTEST) -v -timeout=20m ./test/e2e/...
	@echo "$(GREEN)✅ E2E tests completed$(NC)"

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(BLUE)⚡ Running benchmarks...$(NC)"
	$(GOTEST) -bench=. -benchmem -timeout=10m ./...
	@echo "$(GREEN)✅ Benchmarks completed$(NC)"

##@ Quality

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(BLUE)🎨 Formatting code...$(NC)"
	$(GOFMT) -s -w .
	@echo "$(GREEN)✅ Code formatted$(NC)"

.PHONY: lint
lint: ## Run linter
	@echo "$(BLUE)🔍 Running linter...$(NC)"
	golangci-lint run --timeout=5m
	@echo "$(GREEN)✅ Linting completed$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(BLUE)🔍 Running go vet...$(NC)"
	$(GOCMD) vet ./...
	@echo "$(GREEN)✅ Vet completed$(NC)"

.PHONY: security
security: ## Run security checks
	@echo "$(BLUE)🔒 Running security checks...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)gosec not installed, skipping...$(NC)"; \
	fi
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "$(YELLOW)govulncheck not installed, skipping...$(NC)"; \
	fi
	@echo "$(GREEN)✅ Security checks completed$(NC)"

.PHONY: check
check: fmt vet lint security test ## Run all quality checks
	@echo "$(GREEN)✅ All quality checks completed!$(NC)"

##@ CI/CD

.PHONY: ci-check
ci-check: ## Run CI integration check
	@echo "$(BLUE)🔍 Running CI integration check...$(NC)"
	./build/scripts/integration-check.sh
	@echo "$(GREEN)✅ CI check completed$(NC)"

.PHONY: ci-deps
ci-deps: ## Run dependency check
	@echo "$(BLUE)📦 Running dependency check...$(NC)"
	./build/scripts/dependency-check.sh
	@echo "$(GREEN)✅ Dependency check completed$(NC)"

.PHONY: ci-perf
ci-perf: ## Run performance monitoring
	@echo "$(BLUE)📊 Running performance monitoring...$(NC)"
	./build/scripts/performance-monitor.sh
	@echo "$(GREEN)✅ Performance monitoring completed$(NC)"

.PHONY: ci-all
ci-all: ci-check ci-deps ci-perf check test-integration ## Run all CI checks
	@echo "$(GREEN)🎉 All CI checks completed successfully!$(NC)"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(BLUE)🐳 Building Docker image...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(DATE) \
		--build-arg VCS_REF=$(COMMIT) \
		-t $(BINARY_NAME):$(VERSION) \
		-t $(BINARY_NAME):latest \
		.
	@echo "$(GREEN)✅ Docker image built$(NC)"

.PHONY: docker-run
docker-run: docker-build ## Run Docker container
	@echo "$(BLUE)🐳 Running Docker container...$(NC)"
	docker run --rm -it $(BINARY_NAME):latest

##@ Release

.PHONY: release-dry
release-dry: ## Dry run release with GoReleaser
	@echo "$(BLUE)🚀 Running release dry run...$(NC)"
	goreleaser release --snapshot --clean --skip=publish
	@echo "$(GREEN)✅ Release dry run completed$(NC)"

.PHONY: release
release: ## Create release with GoReleaser
	@echo "$(BLUE)🚀 Creating release...$(NC)"
	goreleaser release --clean
	@echo "$(GREEN)✅ Release completed$(NC)"

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(BLUE)🧹 Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f performance.log build-metrics.json
	rm -f dependency-*.txt dependency-*.md
	@echo "$(GREEN)✅ Cleanup completed$(NC)"

.PHONY: clean-all
clean-all: clean ## Clean everything including caches
	@echo "$(BLUE)🧹 Deep cleaning...$(NC)"
	$(GOCLEAN) -cache
	$(GOCLEAN) -testcache
	$(GOCLEAN) -modcache
	@echo "$(GREEN)✅ Deep cleanup completed$(NC)"

##@ Information

.PHONY: version
version: ## Show version information
	@echo "$(BLUE)📋 Version Information$(NC)"
	@echo "======================"
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
	@echo "Package: $(PACKAGE)"

.PHONY: info
info: ## Show project information
	@echo "$(BLUE)📋 S3ry Project Information$(NC)"
	@echo "============================"
	@echo "Binary:     $(BINARY_NAME)"
	@echo "Package:    $(PACKAGE)"
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(DATE)"
	@echo "Go Version: $$(go version)"
	@echo "Build Dir:  $(BUILD_DIR)"
	@echo ""
	@echo "$(BLUE)📊 Project Stats$(NC)"
	@echo "=================="
	@echo "Go files:   $$(find . -name '*.go' | wc -l)"
	@echo "Test files: $$(find . -name '*_test.go' | wc -l)"
	@echo "Packages:   $$(go list ./... | wc -l)"

# Phony targets
.PHONY: all
all: clean setup check build test-integration ## Run complete build pipeline
	@echo "$(GREEN)🎉 Complete build pipeline finished!$(NC)"
