#!/bin/bash

# Compatibility wrapper. Prefer ./scripts/generate-openapi.sh.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"$SCRIPT_DIR/generate-openapi.sh"
