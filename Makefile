.PHONY: build test lint bench test-integration

# Stamp version metadata into the binary; trimpath keeps the build
# reproducible across checkout locations.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -trimpath -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

build:
	go build $(LDFLAGS) -o bin/s3ry ./cmd/s3ry

test:
	go test -race ./...

# Prefer the mise-pinned toolchain so make lint uses the same golangci-lint
# version everywhere; fall back to whatever is on PATH.
GOLANGCI_LINT := $(shell command -v mise >/dev/null 2>&1 && echo "mise exec -- golangci-lint" || echo "golangci-lint")

lint:
	$(GOLANGCI_LINT) run ./...

# Compare runs with: go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
bench:
	go test -run '^$$' -bench . -benchmem -count=6 ./internal/s3/ | tee bench.txt

test-integration:
	go test -tags=integration -race -count=1 -run Integration -v ./internal/s3/
