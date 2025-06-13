# S3ry Makefile - Modern build system
.PHONY: help build build-all install test test-integration lint fmt clean release setup dev
.DEFAULT_GOAL := help

# Variables
BINARY_NAME := s3ry
CMD_PATH := ./cmd/s3ry
BUILD_DIR := bin
DIST_DIR := dist
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build flags
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BUILD_FLAGS := -ldflags="$(LDFLAGS)"

# Go environment
export CGO_ENABLED=0
export GOFLAGS=-mod=readonly

help: ## Show this help message
	@echo "S3ry Build System"
	@echo "=================="
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

setup: ## Setup development environment using mise
	@echo "🚀 Setting up development environment..."
	@if command -v mise >/dev/null 2>&1; then \
		mise install; \
		mise run setup; \
	else \
		echo "❌ mise not found. Please install mise first: https://mise.jdx.dev/"; \
		exit 1; \
	fi

install-deps: ## Install Go dependencies
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy

build: ## Build the binary for current platform
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "✅ Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## Build for all supported platforms
	@echo "🔨 Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@echo "  • Building for Darwin AMD64..."
	@GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	@echo "  • Building for Darwin ARM64..."
	@GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	@echo "  • Building for Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	@echo "  • Building for Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	@echo "  • Building for Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@echo "✅ All platforms built in $(DIST_DIR)/"

dev: ## Run in development mode
	@echo "🚀 Starting development server..."
	@go run $(CMD_PATH)

test: ## Run all tests
	@echo "🧪 Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests completed"

test-integration: ## Run integration tests
	@echo "🧪 Running integration tests..."
	@go test -v -tags=integration ./test/integration/...

test-coverage: test ## Generate test coverage report
	@echo "📊 Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

lint: ## Run linting
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix; \
	else \
		echo "❌ golangci-lint not found. Install with: mise install"; \
	fi

fmt: ## Format code
	@echo "🎨 Formatting code..."
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -l -w .; \
	else \
		go fmt ./...; \
	fi

check: fmt lint test ## Run all checks (format, lint, test)

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)/ $(DIST_DIR)/ coverage.out coverage.html
	@echo "✅ Cleaned"

install: build ## Install binary to GOPATH/bin
	@echo "📦 Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(shell go env GOPATH)/bin/
	@echo "✅ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

release: ## Create a new release using goreleaser
	@echo "🚀 Creating release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --clean; \
	else \
		echo "❌ goreleaser not found. Install with: mise install"; \
	fi

release-snapshot: ## Create a snapshot release
	@echo "📸 Creating snapshot release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "❌ goreleaser not found. Install with: mise install"; \
	fi

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

# Docker targets
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker build -t $(BINARY_NAME):$(VERSION) .

docker-run: ## Run Docker container
	@echo "🐳 Running Docker container..."
	@docker run --rm -it $(BINARY_NAME):$(VERSION)

# CI targets
ci-setup: ## Setup CI environment
	@echo "🤖 Setting up CI environment..."
	@go mod download
	@mkdir -p $(BUILD_DIR) $(DIST_DIR)

ci-test: ## Run tests in CI
	@echo "🤖 Running CI tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

ci-build: ## Build in CI
	@echo "🤖 Building in CI..."
	@$(MAKE) build-all

# Development utilities
watch: ## Watch for changes and rebuild
	@echo "👀 Watching for changes..."
	@if command -v fswatch >/dev/null 2>&1; then \
		fswatch -o . -e ".*" -i "\\.go$$" | xargs -n1 -I{} make build; \
	else \
		echo "❌ fswatch not found. Install with: brew install fswatch"; \
	fi

deps-update: ## Update dependencies to latest versions
	@echo "⬆️  Updating dependencies..."
	@build/scripts/update-deps.sh patch

deps-update-minor: ## Update dependencies to latest minor versions
	@echo "⬆️  Updating dependencies (minor)..."
	@build/scripts/update-deps.sh minor

deps-update-major: ## Update dependencies to latest major versions
	@echo "⬆️  Updating dependencies (major)..."
	@build/scripts/update-deps.sh major

deps-security: ## Update dependencies with security fixes
	@echo "🔒 Updating security dependencies..."
	@build/scripts/update-deps.sh security true

deps-check: ## Check for dependency vulnerabilities
	@echo "🔒 Checking dependencies for vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "❌ govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Integration and monitoring
integration-check: ## Run comprehensive integration checks
	@echo "🔍 Running integration checks..."
	@build/scripts/integration-check.sh

performance-check: ## Run performance monitoring
	@echo "📊 Running performance checks..."
	@build/scripts/performance-monitor.sh

parallel-dev-status: ## Show parallel development status
	@echo "👥 Parallel development status..."
	@build/scripts/integration-check.sh | grep -A 20 "LLM Work Status"

# Documentation generation
generate-docs: ## Generate comprehensive project documentation
	@echo "📚 Generating documentation..."
	@build/scripts/generate-docs.sh

docs: generate-docs ## Alias for generate-docs