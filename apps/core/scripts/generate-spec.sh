#!/bin/bash

# Script to generate OpenAPI spec from Go code
# This should be run before generating web API clients to ensure the spec is up to date

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Generating OpenAPI spec..."
cd "$CORE_DIR"

# Check if swag is installed
if ! command -v swag &> /dev/null; then
    echo "swag not found. Installing..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Generate the spec
swag init -g main.go -o docs --parseDependency --parseInternal

echo "OpenAPI spec generated successfully at $CORE_DIR/docs/swagger.yaml"
