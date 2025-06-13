# Build stage
FROM golang:1.23.4-alpine AS builder

# Install necessary packages
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-s -w -extldflags "-static"' \
    -a -installsuffix cgo \
    -o s3ry \
    ./cmd/s3ry

# Final stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/s3ry /s3ry

# Set environment
ENV TZ=UTC

# Set entrypoint
ENTRYPOINT ["/s3ry"]