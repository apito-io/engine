# Hello World JavaScript gRPC Plugin Makefile

PLUGIN_NAME=hc-hello-world-js-plugin
NODE_BINARY=node

# Install dependencies
install:
	npm install

# Start the plugin
start:
	npm start

# Build the plugin (no build needed for JS)
build:
	@echo "No build needed for JavaScript plugin"
	@echo "Plugin ready to run with: npm start"

# Build for production (optimization)
build-prod:
	@echo "No build needed for JavaScript plugin"
	@echo "Plugin ready to run with: npm start"

# Clean build artifacts and node_modules
clean:
	rm -rf node_modules
	rm -f package-lock.json

# Install dependencies (alias for install)
deps:
	npm install

# Update dependencies
update-deps:
	npm update

# Run the plugin (for testing)
run:
	node index.js

# Test the plugin
test:
	node test-plugin.js

# Check for syntax errors
lint:
	node --check index.js

# Show help
help:
	@echo "Available targets:"
	@echo "  install      - Install Node.js dependencies"
	@echo "  start        - Start the plugin (via npm start)"
	@echo "  build        - Build the plugin (no-op for JS)"
	@echo "  build-prod   - Build for production (no-op for JS)"
	@echo "  clean        - Clean node_modules and lock files"
	@echo "  deps         - Install dependencies (alias for install)"
	@echo "  update-deps  - Update dependencies"
	@echo "  run          - Run the plugin directly with node"
	@echo "  test         - Test the plugin"
	@echo "  lint         - Check syntax"
	@echo "  help         - Show this help"

.PHONY: install start build build-prod clean deps update-deps run test lint help

# Default target
.DEFAULT_GOAL := help 