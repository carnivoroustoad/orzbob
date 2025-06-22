.PHONY: all build test clean proto

# Default target
all: build

# Build all binaries
build:
	go build -o bin/orz ./main.go
	go build -o bin/cloud-cp ./cmd/cloud-cp
	go build -o bin/cloud-agent ./cmd/cloud-agent

# Docker build targets
.PHONY: docker-build docker-build-runner

docker-build: docker-build-runner

docker-build-runner:
	docker build -f docker/runner.Dockerfile -t runner:dev .

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

# Kind cluster management
.PHONY: kind-up kind-down e2e-kind

kind-up:
	@if ! command -v kind &> /dev/null && ! test -f ~/go/bin/kind; then \
		echo "Error: kind is not installed. Please install kind from https://kind.sigs.k8s.io/"; \
		exit 1; \
	fi
	@KIND=$$(command -v kind || echo ~/go/bin/kind); \
	if $$KIND get clusters | grep -q orzbob-test; then \
		echo "Kind cluster 'orzbob-test' already exists"; \
	else \
		echo "Creating kind cluster 'orzbob-test'..."; \
		$$KIND create cluster --name orzbob-test; \
	fi
	@echo "Creating namespace 'orzbob-runners'..."
	@kubectl create namespace orzbob-runners --dry-run=client -o yaml | kubectl apply -f -
	@echo "Kind cluster ready!"

kind-down:
	@KIND=$$(command -v kind || echo ~/go/bin/kind); \
	if test -x "$$KIND" && $$KIND get clusters 2>/dev/null | grep -q orzbob-test; then \
		echo "Deleting kind cluster 'orzbob-test'..."; \
		$$KIND delete cluster --name orzbob-test; \
	else \
		echo "Kind cluster 'orzbob-test' not found"; \
	fi

e2e-kind: ## Run full e2e tests in kind
	@echo "Running e2e tests in kind..."
	./hack/e2e-kind.sh

e2e-kind-quick: kind-up ## Run quick e2e tests in existing kind cluster
	@echo "Running e2e tests..."
	go test -v ./test/e2e/... -tags=e2e

validate-cloud-config: ## Validate cloud.yaml configuration
	@if [ -f ".orz/cloud.yaml" ]; then \
		go run hack/validate-cloud-config.go .orz/cloud.yaml; \
	elif [ -f "cloud.yaml" ]; then \
		go run hack/validate-cloud-config.go cloud.yaml; \
	else \
		echo "No cloud.yaml found. Create one at .orz/cloud.yaml"; \
		echo "See examples/cloud.yaml for a template"; \
		exit 1; \
	fi