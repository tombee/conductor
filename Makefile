.PHONY: help build test lint clean coverage

help:
	@echo "Available targets:"
	@echo "  build      - Build the conduct binary"
	@echo "  test       - Run all tests"
	@echo "  lint       - Run golangci-lint"
	@echo "  clean      - Clean build artifacts"
	@echo "  coverage   - Generate test coverage report"

build:
	go build -v ./...

test:
	go test -v -race ./...

lint:
	golangci-lint run

clean:
	rm -f conduct
	go clean -cache -testcache

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
