# Build stage
FROM golang:1.23-alpine AS builder

# Add target architecture for cross-compilation
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the control plane binary with verbose output
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -v -ldflags="-w -s" -o cloud-cp ./cmd/cloud-cp

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 orzbob

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/cloud-cp /usr/local/bin/cloud-cp

# Switch to non-root user
USER orzbob

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the control plane
ENTRYPOINT ["cloud-cp"]