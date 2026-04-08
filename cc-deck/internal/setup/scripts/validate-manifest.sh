#!/bin/bash
# Validate cc-deck-setup.yaml manifest schema.
# Usage: validate-manifest.sh [manifest-path]
# Exit 0 if valid, exit 1 with error message if not.

set -euo pipefail

MANIFEST="${1:-cc-deck-setup.yaml}"

if [ ! -f "$MANIFEST" ]; then
  echo "Error: manifest not found: $MANIFEST"
  exit 1
fi

# Check YAML syntax
if ! yq '.' "$MANIFEST" > /dev/null 2>&1; then
  echo "Error: invalid YAML syntax in $MANIFEST"
  exit 1
fi

# Check required fields
VERSION=$(yq '.version // 0' "$MANIFEST")
if [ "$VERSION" -lt 1 ]; then
  echo "Error: version must be >= 1, got $VERSION"
  exit 1
fi

NAME=$(yq '.image.name // ""' "$MANIFEST")
if [ -z "$NAME" ]; then
  echo "Error: image.name is required"
  exit 1
fi

echo "Manifest valid: $NAME (version $VERSION)"
