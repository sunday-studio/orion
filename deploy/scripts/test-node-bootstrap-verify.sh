#!/usr/bin/env bash
# Exercises release verification in node-bootstrap.sh against a local fake release
# served over file://. Run by `make repository-smoke` and CI.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BOOTSTRAP="$HERE/node-bootstrap.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

case "$(uname -s)" in Linux) OS=linux ;; Darwin) OS=darwin ;; *) echo "unsupported OS"; exit 0 ;; esac
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; arm64|aarch64) ARCH=arm64 ;; *) echo "unsupported arch"; exit 0 ;; esac
ASSET="orion-node-${OS}-${ARCH}"

sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

make_release() {
  local dir="$1"
  rm -rf "$dir"; mkdir -p "$dir"
  printf '#!/usr/bin/env bash\necho installer-ran "$@"\n' > "$dir/orion-node-install.sh"
  printf '[Unit]\nDescription=orion node\n' > "$dir/orion-node-systemd.service"
  printf '<plist/>\n' > "$dir/com.orion.node.plist"
  printf '#!/bin/sh\necho node\n' > "$dir/$ASSET"
  (cd "$dir" && sha SHA256SUMS 2>/dev/null >/dev/null || true)
  (cd "$dir" && sha orion-node-install.sh orion-node-systemd.service com.orion.node.plist "$ASSET" > SHA256SUMS)
}

run_bootstrap() {
  local dir="$1"; shift
  ORION_RELEASE_BASE_URL="file://$dir" bash "$BOOTSTRAP" --verify-only --core-url http://core.example:8999 --version v0.0.0 "$@" >"$WORK/out.txt" 2>&1
}

fail() { echo "FAIL: $1"; echo "--- output ---"; cat "$WORK/out.txt"; exit 1; }
expect_ok()   { run_bootstrap "$@" || fail "expected success: $*"; }
expect_fail() { local want="$1"; shift; if run_bootstrap "$@"; then fail "expected failure: $*"; fi; grep -q "$want" "$WORK/out.txt" || fail "expected message '$want' for: $*"; }

REL="$WORK/release"

make_release "$REL"
expect_ok "$REL"
echo "ok: untampered release installs"

make_release "$REL"; printf 'evil' >> "$REL/$ASSET"
expect_fail "checksum does not match" "$REL"
echo "ok: tampered binary is refused"

make_release "$REL"; printf 'echo pwned\n' >> "$REL/orion-node-install.sh"
expect_fail "checksum does not match" "$REL"
echo "ok: tampered installer script is refused"

make_release "$REL"; grep -v "$ASSET" "$REL/SHA256SUMS" > "$REL/S" && mv "$REL/S" "$REL/SHA256SUMS"
expect_fail "No checksum for" "$REL"
echo "ok: asset missing from SHA256SUMS is refused"

make_release "$REL"; rm "$REL/SHA256SUMS"
if run_bootstrap "$REL"; then fail "release without SHA256SUMS must not install"; fi
echo "ok: release without SHA256SUMS is refused"

make_release "$REL"; printf 'evil' >> "$REL/$ASSET"
expect_ok "$REL" --skip-verify
grep -q "NOT verified" "$WORK/out.txt" || fail "--skip-verify must warn loudly"
echo "ok: --skip-verify is an explicit, loud opt-out"

if ! command -v cosign >/dev/null 2>&1; then
  make_release "$REL"
  expect_fail "cosign is required" "$REL" --require-signature
  echo "ok: --require-signature fails without cosign"
fi

echo "all bootstrap verification checks passed"
