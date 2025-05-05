#!/bin/bash

# Check if version type argument is provided
if [ $# -ne 1 ]; then
    echo "Usage: $0 [major|minor]"
    exit 1
fi

VERSION_TYPE=$1

# Get the current version from version.go
CURRENT_VERSION=$(grep 'Version = ' internal/version/version.go | cut -d'"' -f2)

# Split version into major, minor, and patch
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Calculate new version based on type
case $VERSION_TYPE in
    "major")
        NEW_VERSION="$((MAJOR + 1)).0.0"
        ;;
    "minor")
        NEW_VERSION="$MAJOR.$((MINOR + 1)).0"
        ;;
    *)
        echo "Invalid version type. Use 'major' or 'minor'"
        exit 1
        ;;
esac

# Get current commit hash and date
COMMIT_HASH=$(git rev-parse --short HEAD)
BUILD_DATE=$(date -u '+%Y-%m-%d %H:%M:%S UTC')

# Update version.go with new version
sed -i "s/Version = \".*\"/Version = \"$NEW_VERSION\"/" internal/version/version.go
sed -i "s/Commit = \".*\"/Commit = \"$COMMIT_HASH\"/" internal/version/version.go
sed -i "s/Date = \".*\"/Date = \"$BUILD_DATE\"/" internal/version/version.go

# Print the new version
echo "Bumped $VERSION_TYPE version to $NEW_VERSION" 