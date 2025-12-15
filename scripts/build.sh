#!/bin/bash

# Build script that injects version information at build time

# Get the current version from version.go (or use environment variable if set)
if [ -z "$VERSION" ]; then
    VERSION=$(grep 'Version = ' internal/version/version.go | cut -d'"' -f2)
    # If version.go shows "dev", try to get from git tags
    if [ "$VERSION" = "dev" ]; then
        VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "0.2.5")
    fi
fi

# Get current commit hash (short form)
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Get current date in UTC
BUILD_DATE=$(date -u '+%Y-%m-%d %H:%M:%S UTC')

# Build with ldflags to inject version information
LDFLAGS="-X 'github.com/marcdicarlo/osc/internal/version.Version=$VERSION' \
         -X 'github.com/marcdicarlo/osc/internal/version.Commit=$COMMIT' \
         -X 'github.com/marcdicarlo/osc/internal/version.Date=$BUILD_DATE'"

echo "Building osc $VERSION (commit: $COMMIT, built: $BUILD_DATE)"
go build -ldflags "$LDFLAGS" -o osc

if [ $? -eq 0 ]; then
    echo "Build successful!"
    ./osc version
else
    echo "Build failed!"
    exit 1
fi
