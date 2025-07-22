# Hello World gRPC Plugin Makefile

PLUGIN_NAME=hc-hello-world-plugin
BINARY_NAME=hc-hello-world-plugin

.PHONY: help build clean test tidy fmt deps run

# Default target
help:
	@echo "Available targets:"
	@echo "  build        - Build plugin"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  tidy         - Tidy dependencies"
	@echo "  fmt          - Format code"
	@echo "  deps         - Install dependencies"
	@echo "  run          - Run plugin"

# Build the plugin
build:
	@echo "Building plugin..."
	go build -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"

build-debug:
	@echo "Building plugin for debugging..."
	go build -gcflags="all=-N -l" -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"

# Build for production (smaller binary)
build-prod:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) .

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	go clean -cache
	@echo "Cleaned build artifacts"

# Run tests
test:
	go test -v ./...

# Tidy dependencies
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Install dependencies
deps:
	go mod download

# Run the plugin (for testing)
run:
	go run main.go

# Update dependencies
update-deps:
	go get -u github.com/apito-io/buffers
	go mod tidy