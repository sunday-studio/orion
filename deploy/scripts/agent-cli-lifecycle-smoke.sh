#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AGENT_BINARY="${AGENT_BINARY:-/bin/echo}"
WORK_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

assert_contains() {
  local file="$1"
  local expected="$2"

  if ! grep -Fq -- "$expected" "$file"; then
    printf 'expected %s to contain: %s\n' "$file" "$expected" >&2
    printf '%s\n' "--- $file ---" >&2
    sed -n '1,220p' "$file" >&2
    exit 1
  fi
}

assert_fails() {
  local output="$1"
  shift

  set +e
  "$@" >"$output" 2>&1
  local code=$?
  set -e
  if [ "$code" -eq 0 ]; then
    printf 'expected command to fail: %s\n' "$*" >&2
    exit 1
  fi
}

cd "$ROOT_DIR"

assert_fails "$WORK_DIR/install-missing-value.out" deploy/scripts/agent-install.sh --dry-run --core-url
assert_contains "$WORK_DIR/install-missing-value.out" "--core-url requires a value."

assert_fails "$WORK_DIR/bootstrap-missing-value.out" deploy/scripts/agent-bootstrap.sh --dry-run --version
assert_contains "$WORK_DIR/bootstrap-missing-value.out" "--version requires a value."

NO_COLOR=1 deploy/scripts/agent-install.sh \
  --dry-run \
  --binary "$AGENT_BINARY" \
  --core-url http://127.0.0.1:8999 \
  --no-start >"$WORK_DIR/install-dry-run.out"
assert_contains "$WORK_DIR/install-dry-run.out" "Agent installer -"
assert_contains "$WORK_DIR/install-dry-run.out" "Next commands"
assert_contains "$WORK_DIR/install-dry-run.out" "orion-agent config show"

NO_COLOR=1 deploy/scripts/agent-uninstall.sh \
  --dry-run \
  --keep-config \
  --keep-state \
  --keep-user >"$WORK_DIR/uninstall-keep-dry-run.out"
assert_contains "$WORK_DIR/uninstall-keep-dry-run.out" "Agent uninstaller"
assert_contains "$WORK_DIR/uninstall-keep-dry-run.out" "Config is kept by default"
assert_contains "$WORK_DIR/uninstall-keep-dry-run.out" "State is kept by default"

NO_COLOR=1 deploy/scripts/agent-uninstall.sh \
  --dry-run \
  --purge >"$WORK_DIR/uninstall-purge-dry-run.out"
assert_contains "$WORK_DIR/uninstall-purge-dry-run.out" "Agent uninstaller"
assert_contains "$WORK_DIR/uninstall-purge-dry-run.out" "Config removal requested"
assert_contains "$WORK_DIR/uninstall-purge-dry-run.out" "State removal requested"

printf '%s\n' "Agent CLI lifecycle smoke passed."
