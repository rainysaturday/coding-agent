BINARY      := coding-agent
PKG         := github.com/coding-agent/coding-agent/version

GIT_HASH    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_DIRTY   := $(shell git diff --quiet 2>/dev/null && echo clean || echo dirty)
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -X $(PKG).GitHash=$(GIT_HASH) -X $(PKG).GitDirty=$(GIT_DIRTY) -X $(PKG).BuildTime=$(BUILD_TIME)

.PHONY: all build-all build build-native test vet lint fmt fmtcheck tidy clean run \
        build-darwin-arm64 build-linux-amd64 build-windows-amd64 \
        linux darwin-arm64 windows

all: build-all

build: build-native

# build-all compiles the binary for every supported platform.
build-all: build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o coding-agent-darwin-arm64 .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o coding-agent-linux-amd64 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o coding-agent-windows-amd64.exe .

build-native:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Aliases kept for convenience (same outputs as the build-* targets).
linux: build-linux-amd64
darwin-arm64: build-darwin-arm64
windows: build-windows-amd64

test:
	go test ./...

vet:
	go vet ./...

lint: vet fmtcheck

fmt:
	gofmt -w .

fmtcheck:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)

tidy:
	go mod tidy

run: build-native
	./$(BINARY)

clean:
	rm -f $(BINARY) coding-agent-darwin-arm64 coding-agent-linux-amd64 coding-agent-windows-amd64.exe
	rm -rf dist
