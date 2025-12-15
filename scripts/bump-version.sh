#!/bin/bash

# Check if version type argument is provided
if [ $# -ne 1 ]; then
    echo "Usage: $0 [major|minor|patch]"
    exit 1
fi

VERSION_TYPE=$1

# Get the current version from git tags
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "0.2.5")

# Remove 'v' prefix if present
CURRENT_VERSION=${CURRENT_VERSION#v}

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
    "patch")
        NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))"
        ;;
    *)
        echo "Invalid version type. Use 'major', 'minor', or 'patch'"
        exit 1
        ;;
esac

# Create git tag
echo "Bumping $VERSION_TYPE version: $CURRENT_VERSION -> $NEW_VERSION"
git tag -a "v$NEW_VERSION" -m "Release version $NEW_VERSION"

if [ $? -eq 0 ]; then
    echo "Created tag v$NEW_VERSION"
    echo ""
    echo "To push the tag to remote, run:"
    echo "  git push origin v$NEW_VERSION"
    echo ""
    echo "To build with this version, run:"
    echo "  ./scripts/build.sh"
else
    echo "Failed to create tag"
    exit 1
fi
