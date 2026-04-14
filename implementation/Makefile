BINARY := coding-agent
VERSION ?= dev
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_TREE := $(shell test -n "$(shell git status --porcelain 2>/dev/null)" && echo dirty || echo clean)

.PHONY: build build-with-version test

build:
	go build -ldflags "-X main.version=$(VERSION) -X main.gitHash=$(GIT_HASH) -X main.gitTree=$(GIT_TREE)" -o $(BINARY) .

build-with-version: build

test:
	go test ./...
