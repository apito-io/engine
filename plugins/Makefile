# HashiCorp Plugins Master Makefile
# This file builds all HashiCorp plugins from the engine context

SHELL := /bin/bash
.PHONY: build-debug-all build-production-all clean-all help

# Default target
all: build-debug-all

# Build all plugins without debug flags for production
build-production-all:
	@echo "Building all HashiCorp plugins without debug flags..."
	@echo "================================================="
	@echo "Building hc-hello-world-plugin (production)..."
	@if [ -d "hc-hello-world-plugin" ]; then \
		(cd hc-hello-world-plugin && GOOS=linux go build -o ./hc-hello-world-plugin .) && echo "✓ hc-hello-world-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-hello-world-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-suchok-plugin (production)..."
	@if [ -d "hc-suchok-plugin" ]; then \
		(cd hc-suchok-plugin && GOOS=linux go build -o ./hc-suchok-plugin .) && echo "✓ hc-suchok-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-suchok-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-prottoy-plugin (production)..."
	@if [ -d "hc-prottoy-plugin" ]; then \
		(cd hc-prottoy-plugin && GOOS=linux go build -o ./hc-prottoy-plugin .) && echo "✓ hc-prottoy-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-prottoy-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-rosna-plugin (production)..."
	@if [ -d "hc-rosna-plugin" ]; then \
		(cd hc-rosna-plugin && GOOS=linux go build -o ./hc-rosna-plugin .) && echo "✓ hc-rosna-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-rosna-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-protiva-plugin (production)..."
	@if [ -d "hc-protiva-plugin" ]; then \
		(cd hc-protiva-plugin && GOOS=linux go build -o ./hc-protiva-plugin .) && echo "✓ hc-protiva-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-protiva-plugin directory not found, skipping"; \
	fi
	@echo "All Go-based plugins build process completed!"
	@echo "Note: Skipping hc-hello-world-js-plugin (JavaScript plugin)"

# Build all plugins with debug flags for debugging
build-debug-all:
	@echo "Building all HashiCorp plugins with debug flags..."
	@echo "================================================="
	@echo "Building hc-hello-world-plugin (debug)..."
	@if [ -d "hc-hello-world-plugin" ]; then \
		(cd hc-hello-world-plugin && go build -gcflags="all=-N -l" -o ./hc-hello-world-plugin .) && echo "✓ hc-hello-world-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-hello-world-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-suchok-plugin (debug)..."
	@if [ -d "hc-suchok-plugin" ]; then \
		(cd hc-suchok-plugin && go build -gcflags="all=-N -l" -o ./hc-suchok-plugin .) && echo "✓ hc-suchok-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-suchok-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-prottoy-plugin (debug)..."
	@if [ -d "hc-prottoy-plugin" ]; then \
		(cd hc-prottoy-plugin && go build -gcflags="all=-N -l" -o ./hc-prottoy-plugin .) && echo "✓ hc-prottoy-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-prottoy-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-rosna-plugin (debug)..."
	@if [ -d "hc-rosna-plugin" ]; then \
		(cd hc-rosna-plugin && go build -gcflags="all=-N -l" -o ./hc-rosna-plugin .) && echo "✓ hc-rosna-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-rosna-plugin directory not found, skipping"; \
	fi
	@echo "Building hc-protiva-plugin (debug)..."
	@if [ -d "hc-protiva-plugin" ]; then \
		(cd hc-protiva-plugin && go build -gcflags="all=-N -l" -o ./hc-protiva-plugin .) && echo "✓ hc-protiva-plugin built successfully" || exit 1; \
	else \
		echo "⚠ hc-protiva-plugin directory not found, skipping"; \
	fi
	@echo "All Go-based plugins build process completed!"
	@echo "Note: Skipping hc-hello-world-js-plugin (JavaScript plugin)"

# Clean all plugin binaries
clean-all:
	@echo "Cleaning all HashiCorp plugin binaries..."
	@if [ -d "hc-hello-world-plugin" ]; then \
		(cd hc-hello-world-plugin && rm -f hc-hello-world-plugin); \
	fi
	@if [ -d "hc-suchok-plugin" ]; then \
		(cd hc-suchok-plugin && rm -f hc-suchok-plugin); \
	fi
	@if [ -d "hc-prottoy-plugin" ]; then \
		(cd hc-prottoy-plugin && rm -f hc-prottoy-plugin); \
	fi
	@if [ -d "hc-rosna-plugin" ]; then \
		(cd hc-rosna-plugin && rm -f hc-rosna-plugin); \
	fi
	@if [ -d "hc-protiva-plugin" ]; then \
		(cd hc-protiva-plugin && rm -f hc-protiva-plugin); \
	fi
	@echo "All Go plugin binaries cleaned!"

# Help target
help:
	@echo "Available targets:"
	@echo "  build-production-all - Build all Go plugins without debug flags"
	@echo "  build-debug-all      - Build all Go plugins with debug flags"
	@echo "  clean-all            - Clean all Go plugin binaries"
	@echo "  help                 - Show this help message" 