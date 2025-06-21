# Build stage
FROM golang:1.23-alpine AS builder

# Add target architecture for cross-compilation
ARG TARGETARCH

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the cloud-agent binary with verbose output and timeout
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -v -ldflags="-w -s" -o cloud-agent ./cmd/cloud-agent

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    bash \
    git \
    tmux \
    openssh-client \
    ca-certificates \
    curl \
    jq

# Create non-root user
RUN adduser -D -u 1000 runner

# Create workspace directory
RUN mkdir -p /workspace && chown runner:runner /workspace

# Copy binary from builder
COPY --from=builder /build/cloud-agent /usr/local/bin/cloud-agent

# Set user and working directory
USER runner
WORKDIR /workspace

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/cloud-agent"]

# Default command runs the agent
CMD []