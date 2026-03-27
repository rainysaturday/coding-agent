.PHONY: build test clean version run

# Get version information from git
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell if git diff --quiet 2>/dev/null; then echo "clean"; else echo "dirty"; fi)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Go build flags for version embedding
LDFLAGS := -X main.gitHash=$(GIT_HASH) \
           -X main.gitDirty=$(GIT_DIRTY) \
           -X main.buildTime=$(BUILD_TIME)

# Binary name
BINARY := coding-agent

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./... -v

clean:
	rm -f $(BINARY)
	go clean

version:
	@echo "Git Hash: $(GIT_HASH)"
	@echo "Dirty:    $(GIT_DIRTY)"
	@echo "Built:    $(BUILD_TIME)"

run: build
	./$(BINARY)

# Default target
all: build test
