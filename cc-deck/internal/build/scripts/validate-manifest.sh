#!/bin/bash
# Validate cc-deck-build.yaml manifest schema.
# Usage: validate-manifest.sh [manifest-path]
# Exit 0 if valid, exit 1 with error message if not.

set -euo pipefail

MANIFEST="${1:-cc-deck-build.yaml}"

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
if ! [[ "$VERSION" =~ ^[0-9]+$ ]]; then
  echo "Error: version must be an integer >= 1, got '$VERSION'"
  exit 1
fi
if [ "$VERSION" -lt 1 ]; then
  echo "Error: version must be >= 1, got $VERSION"
  exit 1
fi

# Check target configuration (at least one target should be present for builds)
CONTAINER_NAME=$(yq '.targets.container.name // ""' "$MANIFEST")
SSH_HOST=$(yq '.targets.ssh.host // ""' "$MANIFEST")

if [ -n "$CONTAINER_NAME" ]; then
  echo "Manifest valid: container target '$CONTAINER_NAME' (version $VERSION)"
elif [ -n "$SSH_HOST" ]; then
  echo "Manifest valid: ssh target '$SSH_HOST' (version $VERSION)"
else
  echo "Manifest valid: no targets configured yet (version $VERSION)"
fi
