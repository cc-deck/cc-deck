#!/bin/bash
# Safely update a section of cc-deck-build.yaml.
# Usage: update-manifest.sh <section> <yaml-fragment> [manifest-path]
#
# Example:
#   update-manifest.sh tools '"Go compiler >= 1.23"' cc-deck-build.yaml
#   update-manifest.sh sources '{"url":"https://...","ref":"main"}' cc-deck-build.yaml

set -euo pipefail

SECTION="${1:?Usage: update-manifest.sh <section> <yaml-fragment> [manifest-path]}"
FRAGMENT="${2:?Usage: update-manifest.sh <section> <yaml-fragment> [manifest-path]}"
MANIFEST="${3:-cc-deck-build.yaml}"

if [ ! -f "$MANIFEST" ]; then
  echo "Error: manifest not found: $MANIFEST"
  exit 1
fi

if ! command -v yq >/dev/null 2>&1; then
  echo "Error: yq is required for manifest updates"
  exit 1
fi

# Append to the section array
yq -i ".${SECTION} += [${FRAGMENT}]" "$MANIFEST"
echo "Updated ${SECTION} in ${MANIFEST}"
