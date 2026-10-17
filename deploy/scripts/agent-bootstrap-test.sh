#!/usr/bin/env bash
# Installer tests against a local fake release (file:// base URL). Changes nothing on the host.
#   ORION_TEST_BINARY=path/to/orion-agent-<os>-<arch> deploy/scripts/agent-bootstrap-test.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OS="$(uname -s | tr 'A-Z' 'a-z')"
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; arm64|aarch64) ARCH=arm64 ;; armv7l) ARCH=armv7 ;; esac
ASSET="orion-agent-${OS}-${ARCH}"
BIN="${ORION_TEST_BINARY:-}"
if [ -z "$BIN" ]; then
  BIN="$(mktemp -d)/$ASSET"
  (cd "$ROOT/apps/agent" && CGO_ENABLED=0 go build -o "$BIN" .)
fi

REL="$(mktemp -d)"
trap 'rm -rf "$REL"' EXIT
cp "$BIN" "$REL/$ASSET"
cp "$ROOT/deploy/scripts/agent-install.sh" "$REL/orion-agent-install.sh"
cp "$ROOT/deploy/scripts/agent-uninstall.sh" "$REL/orion-agent-uninstall.sh"
cp "$ROOT/deploy/systemd/orion-agent.service" "$REL/orion-agent-systemd.service"
cp "$ROOT/deploy/launchd/com.orion.agent.plist" "$REL/com.orion.agent.plist"
sum() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
(cd "$REL" && sum "$ASSET" orion-agent-install.sh orion-agent-uninstall.sh orion-agent-systemd.service com.orion.agent.plist > SHA256SUMS)

BOOT="$ROOT/deploy/scripts/agent-bootstrap.sh"
fail=0
check() { # name expected_exit expected_text -- args
  local name="$1" want="$2" text="$3"; shift 4
  local out code=0
  out="$("$BOOT" "$@" 2>&1)" || code=$?
  if [ "$code" -ne "$want" ] || ! printf '%s' "$out" | grep -q -- "$text"; then
    printf 'FAIL %s (exit %s, want %s, text %q)\n%s\n' "$name" "$code" "$want" "$text" "$out"; fail=1
  else printf 'ok   %s\n' "$name"; fi
}

BASE="file://$REL"
check "dry run passes and changes nothing" 0 "Nothing was changed" -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
check "plan is printed"                   0 "Plan"                -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
check "missing core url is a clear error" 1 "Core URL is required" -- --dry-run --yes --base-url "$BASE"

echo "x" >> "$REL/$ASSET"            # tamper with the binary
check "tampered binary aborts"            2 "Checksum mismatch"   -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
cp "$BIN" "$REL/$ASSET"
grep -v "$ASSET" "$REL/SHA256SUMS" > "$REL/SUMS.new" && mv "$REL/SUMS.new" "$REL/SHA256SUMS"
check "unlisted binary aborts"            2 "No checksum for"          -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
check "signature required but cosign absent" 2 "cosign is required"  -- --dry-run --yes --require-signature --core-url http://core.example.com --base-url "$BASE"

[ "$fail" -eq 0 ] && echo "installer tests passed"
exit "$fail"
