.POSIX:
.SUFFIXES:

# Build variables
# Note: Version variables are not currently used in the code
# but kept here for future use
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

# Paths
PREFIX     ?= /usr/local
BINDIR     ?= $(PREFIX)/bin
DESTDIR    ?=

# Go settings
GO         ?= go
GOFLAGS    ?=
CGO_ENABLED ?= 0

# Binary name
BINARY     := basar

# Default target
all: build

# Build binary
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/basar

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/basar
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 ./cmd/basar

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 ./cmd/basar
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 ./cmd/basar

build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-windows-amd64.exe ./cmd/basar

# Install to system
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

# Install to user directory
install-user: build
	install -d $(HOME)/.local/bin
	install -m 755 $(BINARY) $(HOME)/.local/bin/$(BINARY)

# Uninstall from system
uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

# Run tests
test:
	$(GO) test -v -race ./...

# Run tests with coverage
cover:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	$(GO) fmt ./...
	gofmt -s -w .

# Lint code
lint:
	golangci-lint run ./...

# Vet code
vet:
	$(GO) vet ./...

# Clean build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-* coverage.out coverage.html

# Show help
help:
	@echo "basar Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all          Build binary (default)"
	@echo "  build        Build binary"
	@echo "  build-all    Build for all platforms"
	@echo "  install      Install to $(BINDIR)"
	@echo "  install-user Install to ~/.local/bin"
	@echo "  uninstall    Remove from $(BINDIR)"
	@echo "  test         Run tests"
	@echo "  cover        Run tests with coverage"
	@echo "  fmt          Format code"
	@echo "  lint         Lint code"
	@echo "  vet          Vet code"
	@echo "  clean        Remove build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  PREFIX       Install prefix (default: /usr/local)"
	@echo "  DESTDIR      Staging directory for packaging"

.PHONY: all build build-all build-linux build-darwin build-windows \
        install install-user uninstall test cover fmt lint vet clean help
