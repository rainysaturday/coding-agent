.PHONY: build build-with-version test clean fmt vet

# Version information
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY := $(shell git diff --quiet 2>/dev/null && echo "clean" || echo "dirty")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS := -ldflags "-X main.gitHash=$(GIT_HASH) -X main.gitDirty=$(GIT_DIRTY) -X main.buildTime=$(BUILD_TIME)"

build:
	cd implementation && go build -o ../coding-agent $(LDFLAGS)

build-with-version:
	cd implementation && go build -o ../coding-agent $(LDFLAGS)

build-dev:
	cd implementation && go build -o ../coding-agent

test:
	cd implementation && go test ./...

test-verbose:
	cd implementation && go test -v ./...

test-coverage:
	cd implementation && go test -cover ./...

clean:
	rm -f coding-agent
	rm -f debug.log
	cd implementation && go clean

fmt:
	gofmt -w implementation/

vet:
	cd implementation && go vet ./...

install:
	cd implementation && go install
