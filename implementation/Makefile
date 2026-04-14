# Minimal Coding Agent Harness Makefile

# Version information
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell if git diff --quiet 2>/dev/null; then echo "clean"; else echo "dirty"; fi)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build flags
LDFLAGS := -X main.gitHash=$(GIT_HASH) \
           -X main.gitDirty=$(GIT_DIRTY) \
           -X main.buildTime=$(BUILD_TIME)

# Output binary name
BINARY := coding-agent

.PHONY: all build clean test vet lint help

all: build

help:
	@echo "Minimal Coding Agent Harness"
	@echo ""
	@echo "Usage:"
	@echo "  make build     - Build the binary"
	@echo "  make test      - Run tests"
	@echo "  make vet       - Run go vet"
	@echo "  make lint      - Run golangci-lint (if available)"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make version   - Show version information"
	@echo "  make help      - Show this help"

build:
	@echo "Building $(BINARY)..."
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

build-windows:
	@echo "Building $(BINARY) for Windows..."
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY).exe .

build-linux:
	@echo "Building $(BINARY) for Linux..."
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux .

build-macos:
	@echo "Building $(BINARY) for macOS..."
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-macos .

build-all: build-windows build-linux build-macos

test:
	@echo "Running tests with coverage..."
	go test -v -cover ./...

test-cover:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race:
	@echo "Running tests with race detector..."
	go test -v -race ./...

vet:
	@echo "Running go vet..."
	go vet ./...

lint:
	@echo "Running golangci-lint..."
	which golangci-lint > /dev/null || (echo "golangci-lint not found, installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	rm -f $(BINARY) $(BINARY).exe $(BINARY)-linux $(BINARY)-macos
	rm -f coverage.out coverage.html
	go clean

version:
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"

mod-tidy:
	go mod tidy

fmt:
	go fmt ./...
