.PHONY: build clean test version-bump-major version-bump-minor version-bump-patch install help

# Get version info for build
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.2.5")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS := -X 'github.com/marcdicarlo/osc/internal/version.Version=$(VERSION)' \
           -X 'github.com/marcdicarlo/osc/internal/version.Commit=$(COMMIT)' \
           -X 'github.com/marcdicarlo/osc/internal/version.Date=$(BUILD_DATE)'
LOCAL_GO_CACHE ?= $(CURDIR)/.cache/go-build
LOCAL_GO_MODCACHE ?= $(CURDIR)/.cache/go-mod
LOCAL_GO_ENV := GOCACHE=$(LOCAL_GO_CACHE) GOMODCACHE=$(LOCAL_GO_MODCACHE)

# Default target
all: build

# Build the binary with version information injected
build:
	@echo "Building osc $(VERSION) (commit: $(COMMIT), built: $(BUILD_DATE))"
	@mkdir -p $(LOCAL_GO_CACHE) $(LOCAL_GO_MODCACHE)
	@$(LOCAL_GO_ENV) go build -ldflags "$(LDFLAGS)" -o osc
	@echo "Build successful!"
	@./osc version

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f osc
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running tests..."
	@mkdir -p $(LOCAL_GO_CACHE) $(LOCAL_GO_MODCACHE)
	@$(LOCAL_GO_ENV) go test ./...

# Bump major version (e.g., 1.0.0 -> 2.0.0)
version-bump-major:
	@./scripts/bump-version.sh major

# Bump minor version (e.g., 0.1.0 -> 0.2.0)
version-bump-minor:
	@./scripts/bump-version.sh minor

# Bump patch version (e.g., 0.2.0 -> 0.2.1)
version-bump-patch:
	@./scripts/bump-version.sh patch

# Install to /usr/local/bin (requires sudo)
install: build
	@echo "Installing osc to /usr/local/bin..."
	@sudo cp osc /usr/local/bin/
	@echo "Installation complete!"

# Show help
help:
	@echo "Available targets:"
	@echo "  make build              - Build the osc binary with version information"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make test               - Run all tests"
	@echo "  make version-bump-major - Bump major version (1.0.0 -> 2.0.0)"
	@echo "  make version-bump-minor - Bump minor version (0.1.0 -> 0.2.0)"
	@echo "  make version-bump-patch - Bump patch version (0.2.0 -> 0.2.1)"
	@echo "  make install            - Install osc to /usr/local/bin"
	@echo "  make help               - Show this help message"
