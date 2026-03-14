#!/usr/bin/env bash
# Remove demo projects created by setup.sh.

set -euo pipefail

DEMO_DIR="/tmp/cc-deck-demo"

if [[ -d "$DEMO_DIR" ]]; then
    rm -rf "$DEMO_DIR"
    echo "Removed $DEMO_DIR"
else
    echo "Nothing to clean up ($DEMO_DIR does not exist)"
fi
