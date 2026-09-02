.PHONY: build test lint bench

build:
	go build ./cmd/s3ry

test:
	go test -race ./...

lint:
	golangci-lint run ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...
