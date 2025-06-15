.PHONY: all build test clean proto

# Default target
all: build

# Build all binaries
build:
	go build -o bin/orz ./main.go
	go build -o bin/cloud-cp ./cmd/cloud-cp
	go build -o bin/cloud-agent ./cmd/cloud-agent

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Generate protobuf code
proto:
	@if ! command -v buf &> /dev/null; then \
		echo "Installing buf..."; \
		go install github.com/bufbuild/buf/cmd/buf@latest; \
	fi
	@if ! command -v protoc-gen-go &> /dev/null; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi
	@if ! command -v protoc-gen-go-grpc &> /dev/null; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi
	buf generate

# Development helpers
.PHONY: deps
deps:
	go mod download
	go mod tidy

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...