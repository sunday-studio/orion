#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$CORE_DIR"

export GOCACHE="${GOCACHE:-/private/tmp/orion-go-cache}"

if ! command -v swag >/dev/null 2>&1; then
    echo "swag not found. Install it with: go install github.com/swaggo/swag/cmd/swag@latest" >&2
    exit 1
fi

swag init -g main.go -o docs --parseDependency --parseInternal
cp docs/swagger.yaml openapi.yaml

echo "Generated $CORE_DIR/openapi.yaml"
