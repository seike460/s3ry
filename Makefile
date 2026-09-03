.PHONY: build test lint bench

build:
	go build ./cmd/s3ry

test:
	go test -race ./...

lint:
	golangci-lint run ./...

# Compare runs with: go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
bench:
	go test -run '^$$' -bench . -benchmem -count=6 ./internal/s3/ | tee bench.txt
