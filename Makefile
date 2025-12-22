.PHONY: help build test test-integration test-claude-cli lint clean coverage install release snapshot

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

# Installation directory
INSTALL_DIR ?= /usr/local/bin

help:
	@echo "Available targets:"
	@echo "  build           - Build the conductor binary"
	@echo "  test            - Run all tests"
	@echo "  test-integration - Run integration tests (require external services)"
	@echo "  test-claude-cli - Run Claude Code CLI integration tests"
	@echo "  lint            - Run golangci-lint"
	@echo "  clean           - Clean build artifacts"
	@echo "  coverage        - Generate test coverage report"
	@echo "  install         - Install conductor binary to $(INSTALL_DIR)"
	@echo "  release         - Create a new release with GoReleaser"
	@echo "  snapshot        - Build snapshot release (no publish)"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  COMMIT=$(COMMIT)"
	@echo "  BUILD_DATE=$(BUILD_DATE)"

build:
	@echo "Building conductor $(VERSION)..."
	go build -ldflags="$(LDFLAGS)" -o bin/conductor ./cmd/conductor

test:
	go test -v -race ./...

test-integration:
	go test -v -race -tags=integration ./...

test-claude-cli:
	@command -v claude >/dev/null 2>&1 || command -v claude-code >/dev/null 2>&1 || { \
		echo "Error: Claude CLI not found. Install Claude Code to run these tests."; \
		exit 1; \
	}
	CONDUCTOR_CLAUDE_CLI=1 go test -v -tags=integration ./pkg/llm/providers/claudecode/...

lint:
	golangci-lint run

clean:
	rm -f conductor
	rm -rf bin/
	rm -rf dist/
	go clean -cache -testcache

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

install: build
	@echo "Installing conductor to $(INSTALL_DIR)..."
	@if [ -w "$(INSTALL_DIR)" ]; then \
		install -m 755 bin/conductor $(INSTALL_DIR)/conductor; \
	else \
		echo "$(INSTALL_DIR) is not writable, using sudo..."; \
		sudo install -m 755 bin/conductor $(INSTALL_DIR)/conductor; \
	fi
	@echo "Conductor installed successfully!"
	@echo "Run 'conductor version' to verify installation"

release:
	@if [ -z "$(shell which goreleaser)" ]; then \
		echo "Error: goreleaser not found"; \
		echo "Install it from: https://goreleaser.com/install/"; \
		exit 1; \
	fi
	@echo "Creating release $(VERSION)..."
	goreleaser release --clean

snapshot:
	@if [ -z "$(shell which goreleaser)" ]; then \
		echo "Error: goreleaser not found"; \
		echo "Install it from: https://goreleaser.com/install/"; \
		exit 1; \
	fi
	@echo "Building snapshot release..."
	goreleaser release --snapshot --clean
