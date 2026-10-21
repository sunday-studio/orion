#!/usr/bin/env bash
# Installer tests against a local fake release (file:// base URL). Changes nothing on the host.
#   ORION_TEST_BINARY=path/to/orion-node-<os>-<arch> deploy/scripts/node-bootstrap-test.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OS="$(uname -s | tr 'A-Z' 'a-z')"
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; arm64|aarch64) ARCH=arm64 ;; armv7l) ARCH=armv7 ;; esac
ASSET="orion-node-${OS}-${ARCH}"
BIN="${ORION_TEST_BINARY:-}"
if [ -z "$BIN" ]; then
  BIN="$(mktemp -d)/$ASSET"
  (cd "$ROOT/apps/node" && CGO_ENABLED=0 go build -o "$BIN" .)
fi

REL="$(mktemp -d)"
trap 'rm -rf "$REL"' EXIT
cp "$BIN" "$REL/$ASSET"
cp "$ROOT/deploy/scripts/node-install.sh" "$REL/orion-node-install.sh"
cp "$ROOT/deploy/scripts/node-uninstall.sh" "$REL/orion-node-uninstall.sh"
cp "$ROOT/deploy/systemd/orion-node.service" "$REL/orion-node-systemd.service"
cp "$ROOT/deploy/launchd/com.orion.node.plist" "$REL/com.orion.node.plist"
sum() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }
(cd "$REL" && sum "$ASSET" orion-node-install.sh orion-node-uninstall.sh orion-node-systemd.service com.orion.node.plist > SHA256SUMS)

BOOT="$ROOT/deploy/scripts/node-bootstrap.sh"
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
check "dry run prints only the plan"       0 "dry run: this plan makes no changes" -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
check "plan is printed"                   0 "Plan"                -- --dry-run --yes --core-url http://core.example.com --base-url "$BASE"
check "dry run never fetches release assets" 0 "Plan"              -- --dry-run --yes --core-url http://core.example.com --base-url file:///not-a-release
check "missing core url is a clear error" 1 "Core URL is required" -- --dry-run --yes --base-url "$BASE"

echo "x" >> "$REL/$ASSET"            # tamper with the binary
check "tampered binary aborts"            2 "checksum does not match"   -- --verify-only --yes --core-url http://core.example.com --base-url "$BASE"
cp "$BIN" "$REL/$ASSET"
grep -v "$ASSET" "$REL/SHA256SUMS" > "$REL/SUMS.new" && mv "$REL/SUMS.new" "$REL/SHA256SUMS"
check "unlisted binary aborts"            2 "No checksum for"          -- --verify-only --yes --core-url http://core.example.com --base-url "$BASE"
check "signature required but cosign absent" 2 "cosign is required"  -- --verify-only --yes --require-signature --core-url http://core.example.com --base-url "$BASE"

# A real per-user install into a temporary HOME shows the steps, then a re-run is a no-op.
cp "$BIN" "$REL/$ASSET"
(cd "$REL" && sum "$ASSET" orion-node-install.sh orion-node-uninstall.sh orion-node-systemd.service com.orion.node.plist > SHA256SUMS)
H="$(mktemp -d)"
out="$(HOME="$H" "$BOOT" --user --no-start --yes --core-url http://core.example.com --base-url "$BASE" 2>&1)" || { printf 'FAIL user install\n%s\n' "$out"; fail=1; }
for want in "Orion Node installer" "Downloaded" "Verified" "sha256" "Installed" "Service" "installed, not started"; do
  printf '%s' "$out" | grep -q -- "$want" || { printf 'FAIL install output lacks %q\n%s\n' "$want" "$out"; fail=1; }
done
out="$(HOME="$H" "$BOOT" --user --no-start --yes --core-url http://core.example.com --base-url "$BASE" 2>&1)" || { printf 'FAIL rerun\n%s\n' "$out"; fail=1; }
printf '%s' "$out" | grep -q "nothing to do" && printf 'ok   re-running is a no-op\n' || { printf 'FAIL rerun is not a no-op\n%s\n' "$out"; fail=1; }
rm -rf "$H"

# A tampered download stops at Verified with the expected and actual hash, the fix and the help link.
echo "x" >> "$REL/$ASSET"
out="$("$BOOT" --user --no-start --yes --core-url http://core.example.com --base-url "$BASE" 2>&1)" && { echo "FAIL tampered install succeeded"; fail=1; }
for want in "Downloaded" "Verified" "checksum does not match" "expected" "got" "Nothing was installed" "Fix" "Help"; do
  printf '%s' "$out" | grep -q -- "$want" || { printf 'FAIL failure output lacks %q\n%s\n' "$want" "$out"; fail=1; }
done

# The finished steps read like the board: header, one line per step, the next command.
cp "$BIN" "$REL/$ASSET"
(cd "$REL" && sum "$ASSET" orion-node-install.sh orion-node-uninstall.sh orion-node-systemd.service com.orion.node.plist > SHA256SUMS)
H="$(mktemp -d)"
out="$(HOME="$H" "$BOOT" --user --no-start --yes --core-url http://core.example.com --base-url "$BASE" 2>&1)"
order="$(printf '%s\n' "$out" | grep -oE 'Orion Node installer|Downloaded|Verified|Installed|Service|orion-node service start' | tr '\n' '|')"
want="Orion Node installer|Downloaded|Verified|Installed|Service|orion-node service start|"
if [ "$order" = "$want" ]; then printf 'ok   steps appear in the board order\n'; else printf 'FAIL step order: %s\n%s\n' "$order" "$out"; fail=1; fi
rm -rf "$H"

# On a terminal the steps are a box whose every line is the same width.
# Only where a person runs it (stdin is a terminal) and BSD script(1) syntax applies; CI skips it.
if [ -t 0 ] && [ "$(uname -s)" = "Darwin" ] && command -v script >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  H="$(mktemp -d)"
  tty_out="$(HOME="$H" NO_COLOR=1 script -q /dev/null "$BOOT" --user --no-start --yes --core-url http://core.example.com --base-url "$BASE" 2>&1 | tr -d '\r' || true)"
  if ! printf '%s' "$tty_out" | grep -q "Steps"; then printf 'skip the terminal box (no pseudo-terminal here)\n'
  elif printf '%s' "$tty_out" | python3 -c '
import re,sys
t=sys.stdin.read()
lines=[l for l in t.split("\n") if l.startswith(" \u256d") or l.startswith(" \u2502") or l.startswith(" \u2570")]
ok=len(lines)>=4 and "Steps" in lines[0] and len({len(l) for l in lines})==1
sys.exit(0 if ok else 1)'; then printf 'ok   the terminal box is one width\n'; else printf 'FAIL terminal box\n%s\n' "$tty_out"; fail=1; fi
  rm -rf "$H"
fi

[ "$fail" -eq 0 ] && echo "installer tests passed"
exit "$fail"
