.PHONY: build test lint bench test-integration

build:
	go build -o bin/s3ry ./cmd/s3ry

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
