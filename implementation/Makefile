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

.PHONY: all build build-all build-darwin-arm64 build-linux-amd64 build-windows-amd64 clean test vet lint help
.PARALLEL:

all: build-all

help:
	@echo "Minimal Coding Agent Harness"
	@echo ""
	@echo "Usage:"
	@echo "  make all                        - Build for all platforms (arm64-darwin, x86_64-linux, x86_64-windows)"
	@echo "  make build                      - Build for the current platform"
	@echo "  make build-all                  - Build for all platforms (alias for make all)"
	@echo "  make build-darwin-arm64         - Build for Apple Silicon macOS"
	@echo "  make build-linux-amd64          - Build for 64-bit Linux"
	@echo "  make build-windows-amd64        - Build for 64-bit Windows"
	@echo "  make test                       - Run tests"
	@echo "  make vet                        - Run go vet"
	@echo "  make lint                       - Run golangci-lint (if available)"
	@echo "  make clean                      - Clean build artifacts"
	@echo "  make version                    - Show version information"
	@echo "  make help                       - Show this help"

build:
	@echo "Building $(BINARY) for current platform..."
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

build-all:
	@$(MAKE) -j build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-arm64:
	@echo "Building $(BINARY) for arm64-darwin..."
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 .

build-linux-amd64:
	@echo "Building $(BINARY) for x86_64-linux..."
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .

build-windows-amd64:
	@echo "Building $(BINARY) for x86_64-windows..."
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty: $(GIT_DIRTY)"
	@echo "Build Time: $(BUILD_TIME)"
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe .

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
	rm -f $(BINARY)-darwin-arm64 $(BINARY)-linux-amd64 $(BINARY)-windows-amd64.exe
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
