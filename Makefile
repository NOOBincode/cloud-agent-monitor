.PHONY: all build clean test test-cover lint lint-fix fmt vet wire deps help

APP_NAME := cloud-agent-monitor
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

all: deps lint test build

build:
	@echo "Building $(APP_NAME)..."
	@go build $(LDFLAGS) -o bin/platform-api ./cmd/platform-api
	@go build $(LDFLAGS) -o bin/worker ./cmd/worker
	@go build $(LDFLAGS) -o bin/agent ./cmd/agent
	@go build $(LDFLAGS) -o bin/advisor-worker ./cmd/advisor-worker
	@go build $(LDFLAGS) -o bin/obs-mcp ./cmd/obs-mcp
	@echo "Build complete!"

clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf dist/
	@go clean -cache -testcache -modcache
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	@go test -v -race -count=1 ./...

test-cover:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

lint-fix:
	@echo "Running linters with auto-fix..."
	@golangci-lint run --fix ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w -local cloud-agent-monitor .

vet:
	@echo "Running go vet..."
	@go vet ./...

wire:
	@echo "Generating wire dependencies..."
	@wire ./cmd/platform-api/...
	@echo "Wire generation complete!"

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies ready!"

deps-update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "Dependencies updated!"

security:
	@echo "Running security scan..."
	@gosec ./...
	@echo "Security scan complete!"

run:
	@echo "Running platform-api..."
	@go run ./cmd/platform-api

run-worker:
	@echo "Running worker..."
	@go run ./cmd/worker

run-agent:
	@echo "Running agent..."
	@go run ./cmd/agent

docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) .
	@echo "Docker image built: $(APP_NAME):$(VERSION)"

docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 $(APP_NAME):$(VERSION)

help:
	@echo "Available targets:"
	@echo "  make build        - Build all binaries"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make test         - Run tests"
	@echo "  make test-cover   - Run tests with coverage report"
	@echo "  make lint         - Run linters"
	@echo "  make lint-fix     - Run linters with auto-fix"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run go vet"
	@echo "  make wire         - Generate wire dependencies"
	@echo "  make deps         - Download dependencies"
	@echo "  make deps-update  - Update dependencies"
	@echo "  make security     - Run security scan"
	@echo "  make run          - Run platform-api"
	@echo "  make run-worker   - Run worker"
	@echo "  make run-agent    - Run agent"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Run Docker container"
