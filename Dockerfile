# Multi-stage Docker build for Multi-Cluster Manager
# This creates a minimal, secure container image for production use

# Build stage - use full Go environment for compilation
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for build process
RUN addgroup -g 1001 -S mcm && \
    adduser -u 1001 -S mcm -G mcm

# Set working directory
WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimizations
# CGO_ENABLED=0 creates a static binary
# -ldflags reduces binary size and adds version info
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -a -installsuffix cgo \
    -o mcm ./cmd/mcm

# Production stage - minimal runtime environment
FROM scratch

# Import timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create non-root user in the final image
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the binary from builder stage
COPY --from=builder /build/mcm /usr/local/bin/mcm

# Create directories for configuration and data
USER mcm
WORKDIR /app

# Add labels for metadata (OCI standard)
LABEL org.opencontainers.image.title="Multi-Cluster Manager" \
      org.opencontainers.image.description="A CLI tool for managing Kubernetes workloads across multiple clusters" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/celikgo/multicluster-manager" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.authors="Your Name <your.email@example.com>"

# Expose health check endpoint (if we add one in the future)
EXPOSE 8080

# Default command
ENTRYPOINT ["/usr/local/bin/mcm"]
CMD ["--help"]

# Health check (optional - useful for container orchestration)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/usr/local/bin/mcm", "clusters", "test"] || exit 1
