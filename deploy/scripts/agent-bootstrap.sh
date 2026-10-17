#!/usr/bin/env bash
# Orion Agent installer (front door).
#
#   curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
#       --core-url https://orion.example.com --enrollment-token-file /path/to/token
#
# Detects the platform, prints a plan, verifies every download against
# SHA256SUMS (and a signature when one is available), installs, enrolls the
# host, and runs `doctor`. Re-running is safe. Works on bash 3.2 (macOS).

set -euo pipefail

REPO="${ORION_REPO:-sunday-studio/orion}"
VERSION="${ORION_VERSION:-latest}"
BASE_URL="${ORION_RELEASE_BASE_URL:-}"
CORE_URL="${ORION_CORE_URL:-}"
CONFIG_SOURCE="${ORION_AGENT_CONFIG:-}"
CONFIG_URL="${ORION_AGENT_CONFIG_URL:-}"
TOKEN_FILE="${ORION_ENROLLMENT_TOKEN_FILE:-}"
PUBKEY="${ORION_MINISIGN_PUBKEY:-}"
REQUIRE_SIGNATURE="false"
START_SERVICE="true"
OVERWRITE_CONFIG="false"
DRY_RUN="false"
USER_MODE="false"
ASSUME_YES="false"
PURGE="false"
ACTION="install"
STEP=0

usage() {
  cat <<'EOF'
Usage: orion-agent-installer.sh [install|uninstall] [options]

Options:
  --core-url URL               Core URL (or ORION_CORE_URL).
  --enrollment-token-file F    File holding the one-time token from Console > Add server.
                               ORION_ENROLLMENT_TOKEN in the environment also works
                               (it is written to a private temp file, never passed on argv).
  --config PATH                Existing config file to install instead of a generated one.
  --config-url URL             Download a config file and install it.
  --version VERSION            Release to install. Default: latest.
  --repo OWNER/REPO            GitHub repository. Default: sunday-studio/orion.
  --base-url URL               Release asset base URL (mirrors, air-gapped, tests).
  --user                       Install for the current user only (no root, no system service).
  --no-start                   Install without starting the service.
  --overwrite-config           Replace an existing installed config.
  --pubkey FILE                minisign public key used to verify SHA256SUMS.minisig.
  --require-signature          Abort unless the signature verifies.
  --yes                        Do not prompt.
  --purge                      With uninstall: also remove config, state, logs and the service user.
  --dry-run                    Download and verify into a temp dir, print what would
                               change, and change nothing.
  -h, --help                   Show this help.

Exit codes: 0 ok, 1 usage or failed install, 2 verification failed (nothing installed).
EOF
}

step() { STEP=$((STEP + 1)); printf '\n[%02d] %s\n' "$STEP" "$1"; }
ok()   { printf '     ok: %s\n' "$1"; }
info() { printf '     %s\n' "$1"; }
die()  { printf 'error: %s\n' "$1" >&2; [ -n "${2:-}" ] && printf 'fix: %s\n' "$2" >&2; exit "${3:-1}"; }

require_value() { [ -n "${2:-}" ] || { printf '%s requires a value.\n' "$1" >&2; usage >&2; exit 1; }; }
require_cmd()   { command -v "$1" >/dev/null 2>&1 || die "missing required command: $1" "install $1 and re-run"; }

# ---------------------------------------------------------------- arguments
if [ "${1:-}" = "install" ] || [ "${1:-}" = "uninstall" ]; then ACTION="$1"; shift; fi
while [ "$#" -gt 0 ]; do
  case "$1" in
    --core-url) require_value "$1" "${2:-}"; CORE_URL="$2"; shift 2 ;;
    --enrollment-token-file) require_value "$1" "${2:-}"; TOKEN_FILE="$2"; shift 2 ;;
    --config) require_value "$1" "${2:-}"; CONFIG_SOURCE="$2"; shift 2 ;;
    --config-url) require_value "$1" "${2:-}"; CONFIG_URL="$2"; shift 2 ;;
    --version) require_value "$1" "${2:-}"; VERSION="$2"; shift 2 ;;
    --repo) require_value "$1" "${2:-}"; REPO="$2"; shift 2 ;;
    --base-url) require_value "$1" "${2:-}"; BASE_URL="$2"; shift 2 ;;
    --pubkey) require_value "$1" "${2:-}"; PUBKEY="$2"; shift 2 ;;
    --require-signature) REQUIRE_SIGNATURE="true"; shift ;;
    --user) USER_MODE="true"; shift ;;
    --no-start) START_SERVICE="false"; shift ;;
    --overwrite-config) OVERWRITE_CONFIG="true"; shift ;;
    --yes|-y) ASSUME_YES="true"; shift ;;
    --purge) PURGE="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; usage >&2; exit 1 ;;
  esac
done

# ---------------------------------------------------------------- platform
detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) die "unsupported OS: $(uname -s)" "Orion Agent supports Linux and macOS" ;;
  esac
}
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    armv7l|armv7) echo armv7 ;;
    *) die "unsupported architecture: $(uname -m)" "supported: amd64, arm64, armv7 (Linux only)" ;;
  esac
}
detect_init() {
  if [ "$1" = "darwin" ]; then echo launchd
  elif command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then echo systemd
  else echo none; fi
}
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else die "no sha256 tool found" "install coreutils (sha256sum) or perl-Digest-SHA (shasum)"; fi
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
[ "$OS" = "darwin" ] && [ "$ARCH" = "armv7" ] && die "armv7 is not a macOS architecture"
INIT="$(detect_init "$OS")"
ASSET="orion-agent-${OS}-${ARCH}"

if [ -z "$BASE_URL" ]; then
  if [ "$VERSION" = "latest" ]; then BASE_URL="https://github.com/${REPO}/releases/latest/download"
  else BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"; fi
fi

# ---------------------------------------------------------------- uninstall
if [ "$ACTION" = "uninstall" ]; then
  require_cmd curl
  WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
  printf 'Orion Agent uninstall\n'
  curl -fsSL "$BASE_URL/SHA256SUMS" -o "$WORK/SHA256SUMS"
  curl -fsSL "$BASE_URL/orion-agent-uninstall.sh" -o "$WORK/uninstall.sh"
  want="$(awk '$2=="orion-agent-uninstall.sh"||$2=="*orion-agent-uninstall.sh"{print $1}' "$WORK/SHA256SUMS")"
  [ -n "$want" ] && [ "$(sha256_of "$WORK/uninstall.sh")" = "$want" ] || die "uninstaller failed checksum verification" "" 2
  chmod +x "$WORK/uninstall.sh"
  args=(); [ "$DRY_RUN" = "true" ] && args+=(--dry-run); [ "$ASSUME_YES" = "true" ] && args+=(--yes); [ "$PURGE" = "true" ] && args+=(--purge); [ "$USER_MODE" = "true" ] && args+=(--user)
  if [ "$(id -u)" -ne 0 ] && [ "$USER_MODE" != "true" ]; then exec sudo "$WORK/uninstall.sh" ${args[@]+"${args[@]}"}; fi
  exec "$WORK/uninstall.sh" ${args[@]+"${args[@]}"}
fi

# ---------------------------------------------------------------- inputs
if [ -z "$CONFIG_SOURCE" ] && [ -z "$CONFIG_URL" ] && [ -z "$CORE_URL" ] && [ -r /dev/tty ] && [ "$ASSUME_YES" != "true" ]; then
  printf 'Orion Core URL: ' >/dev/tty; IFS= read -r CORE_URL </dev/tty
fi
[ -n "$CONFIG_SOURCE" ] || [ -n "$CONFIG_URL" ] || [ -n "$CORE_URL" ] || \
  die "Core URL is required" "pass --core-url https://your-core, or set ORION_CORE_URL"
case "$CONFIG_SOURCE" in ""|/*) ;; *) CONFIG_SOURCE="$(pwd)/$CONFIG_SOURCE" ;; esac
[ -z "$TOKEN_FILE" ] || [ -r "$TOKEN_FILE" ] || die "cannot read token file: $TOKEN_FILE"

require_cmd curl
require_cmd mktemp
require_cmd awk

# ---------------------------------------------------------------- plan
if [ "$USER_MODE" = "true" ]; then
  BIN_DIR="$HOME/.local/bin"; CFG="$HOME/.config/orion/config.yaml"; STATE_DIR="$HOME/.local/state/orion"
  PRIV="none (per-user install)"
else
  BIN_DIR="/usr/local/bin"
  if [ "$OS" = "linux" ]; then CFG="/etc/orion/config.yaml"; STATE_DIR="/var/lib/orion"
  else CFG="/usr/local/etc/orion/config.yaml"; STATE_DIR="/usr/local/var/lib/orion"; fi
  if [ "$(id -u)" -eq 0 ]; then PRIV="root"; else PRIV="sudo (will prompt)"; fi
fi

printf 'Orion Agent installer\n'
step "Plan"
info "platform:   $OS/$ARCH, init: $INIT"
info "release:    $REPO@$VERSION"
info "download:   $BASE_URL/$ASSET (+ SHA256SUMS)"
info "binary:     $BIN_DIR/orion-agent"
info "config:     $CFG"
info "state:      $STATE_DIR"
if [ "$USER_MODE" = "true" ]; then info "service:    per-user ($([ "$INIT" = launchd ] && echo launchd agent || echo systemd --user))"
else info "service:    orion-agent ($INIT), runs as an unprivileged user"; fi
info "enroll:     $([ -n "$TOKEN_FILE${ORION_ENROLLMENT_TOKEN:-}" ] && echo 'yes, with a one-time token' || echo 'no token given; the host registers on first start')"
info "privilege:  $PRIV"
[ "$DRY_RUN" = "true" ] && info "mode:       dry run, nothing will be changed"

if [ "$INIT" = "none" ] && [ "$USER_MODE" != "true" ]; then
  die "no supported init system found (systemd or launchd)" "use --user, or run the agent under your own supervisor: orion-agent run"
fi

# ---------------------------------------------------------------- download + verify
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
chmod 700 "$WORK"

fetch() { curl -fsSL "$1" -o "$2" || die "download failed: $1" "check the version, network access, and --base-url"; }

step "Download"
fetch "$BASE_URL/SHA256SUMS" "$WORK/SHA256SUMS"
fetch "$BASE_URL/$ASSET" "$WORK/$ASSET"
if [ "$USER_MODE" != "true" ]; then
  fetch "$BASE_URL/orion-agent-install.sh" "$WORK/orion-agent-install.sh"
  fetch "$BASE_URL/orion-agent-systemd.service" "$WORK/orion-agent-systemd.service"
  fetch "$BASE_URL/com.orion.agent.plist" "$WORK/com.orion.agent.plist"
fi
ok "downloaded release assets"

step "Verify"
sig_ok="no"
if [ -n "$PUBKEY" ] && command -v minisign >/dev/null 2>&1 && curl -fsSL "$BASE_URL/SHA256SUMS.minisig" -o "$WORK/SHA256SUMS.minisig" 2>/dev/null; then
  minisign -Vm "$WORK/SHA256SUMS" -p "$PUBKEY" -x "$WORK/SHA256SUMS.minisig" >/dev/null 2>&1 \
    || die "signature verification failed for SHA256SUMS" "do not install; report this at https://github.com/${REPO}/issues" 2
  sig_ok="yes"; ok "SHA256SUMS signature verified"
fi
if [ "$REQUIRE_SIGNATURE" = "true" ] && [ "$sig_ok" != "yes" ]; then
  die "signature required but could not be verified" "install minisign and pass --pubkey FILE" 2
fi
[ "$sig_ok" = "yes" ] || info "no signature check (checksums only); pass --pubkey to require one"

verify_asset() {
  local file="$1" name="$2" want got
  want="$(awk -v n="$name" '$2==n||$2=="*"n{print $1}' "$WORK/SHA256SUMS")"
  [ -n "$want" ] || die "$name is not listed in SHA256SUMS" "this release looks incomplete" 2
  got="$(sha256_of "$file")"
  [ "$got" = "$want" ] || die "checksum mismatch for $name" "expected $want, got $got; nothing was installed" 2
  ok "$name checksum matches"
}
verify_asset "$WORK/$ASSET" "$ASSET"
if [ "$USER_MODE" != "true" ]; then
  verify_asset "$WORK/orion-agent-install.sh" "orion-agent-install.sh"
  verify_asset "$WORK/orion-agent-systemd.service" "orion-agent-systemd.service"
  verify_asset "$WORK/com.orion.agent.plist" "com.orion.agent.plist"
fi
chmod +x "$WORK/$ASSET"
"$WORK/$ASSET" version >/dev/null 2>&1 || die "the downloaded binary does not run on this machine" "check the architecture ($OS/$ARCH)" 2
ok "binary runs on this machine"

if [ -n "$CONFIG_URL" ]; then fetch "$CONFIG_URL" "$WORK/config.yaml"; CONFIG_SOURCE="$WORK/config.yaml"; fi

# One-time token from the environment goes to a private file, never argv.
if [ -z "$TOKEN_FILE" ] && [ -n "${ORION_ENROLLMENT_TOKEN:-}" ]; then
  ( umask 077; printf '%s' "$ORION_ENROLLMENT_TOKEN" > "$WORK/enroll-token" )
  TOKEN_FILE="$WORK/enroll-token"
fi

# ---------------------------------------------------------------- install
if [ "$DRY_RUN" = "true" ] && [ "$USER_MODE" != "true" ]; then
  step "Dry run"
  chmod +x "$WORK/orion-agent-install.sh"
  args=(--binary "$WORK/$ASSET" --dry-run)
  [ -n "$CONFIG_SOURCE" ] && args+=(--config "$CONFIG_SOURCE") || args+=(--core-url "$CORE_URL")
  (cd "$WORK" && mkdir -p deploy/systemd deploy/launchd && cp orion-agent-systemd.service deploy/systemd/orion-agent.service && cp com.orion.agent.plist deploy/launchd/ && ./orion-agent-install.sh ${args[@]+"${args[@]}"})
  printf '\nDry run complete. Nothing was changed.\n'
  exit 0
fi
if [ "$DRY_RUN" = "true" ]; then
  printf '\nDry run complete (verification passed). Nothing was changed.\n'
  exit 0
fi

if [ "$USER_MODE" = "true" ]; then
  step "Install (per-user)"
  mkdir -p "$BIN_DIR" "$(dirname "$CFG")" "$STATE_DIR"; chmod 700 "$STATE_DIR"
  install -m 0755 "$WORK/$ASSET" "$BIN_DIR/orion-agent"
  ok "installed $BIN_DIR/orion-agent"
  if [ -f "$CFG" ] && [ "$OVERWRITE_CONFIG" != "true" ]; then info "keeping existing config $CFG"
  elif [ -n "$CONFIG_SOURCE" ]; then install -m 0600 "$CONFIG_SOURCE" "$CFG"; ok "installed config"
  else printf 'core_url: %s\ninterval: 60s\nlogging:\n  path: %s\nmonitors: []\n' "$CORE_URL" "$STATE_DIR/agent.log" > "$CFG"; chmod 600 "$CFG"; ok "wrote $CFG"; fi
  export ORION_AGENT_SOCKET="$STATE_DIR/agent.sock"
  if [ -n "$TOKEN_FILE" ]; then
    "$BIN_DIR/orion-agent" enroll --config "$CFG" --state "$STATE_DIR/state.db" --core-url "${CORE_URL:-$(awk '/^core_url:/{print $2}' "$CFG")}" --token-file "$TOKEN_FILE" || die "enrollment failed" "create a new token in Console > Add server"
  else
    "$BIN_DIR/orion-agent" state init --state "$STATE_DIR/state.db" >/dev/null
  fi
  step "Service (per-user)"
  if [ "$INIT" = "systemd" ]; then
    unit_dir="$HOME/.config/systemd/user"; mkdir -p "$unit_dir"
    cat > "$unit_dir/orion-agent.service" <<EOF
[Unit]
Description=Orion Agent (user)
After=network-online.target

[Service]
ExecStart=$BIN_DIR/orion-agent run --config $CFG --state $STATE_DIR/state.db --socket $STATE_DIR/agent.sock
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
    if [ "$START_SERVICE" = "true" ]; then systemctl --user enable --now orion-agent; ok "started (systemd --user)"; fi
  elif [ "$INIT" = "launchd" ]; then
    plist="$HOME/Library/LaunchAgents/com.orion.agent.user.plist"; mkdir -p "$(dirname "$plist")"
    cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.orion.agent.user</string>
<key>ProgramArguments</key><array>
<string>$BIN_DIR/orion-agent</string><string>run</string>
<string>--config</string><string>$CFG</string>
<string>--state</string><string>$STATE_DIR/state.db</string>
<string>--socket</string><string>$STATE_DIR/agent.sock</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
    if [ "$START_SERVICE" = "true" ]; then launchctl bootout "gui/$(id -u)" "$plist" >/dev/null 2>&1 || true; launchctl bootstrap "gui/$(id -u)" "$plist"; ok "started (launchd agent)"; fi
  else
    info "no init system; run: $BIN_DIR/orion-agent run"
  fi
else
  step "Install"
  SUDO=""; [ "$(id -u)" -ne 0 ] && { require_cmd sudo; SUDO="sudo"; }
  chmod +x "$WORK/orion-agent-install.sh"
  mkdir -p "$WORK/deploy/systemd" "$WORK/deploy/launchd"
  cp "$WORK/orion-agent-systemd.service" "$WORK/deploy/systemd/orion-agent.service"
  cp "$WORK/com.orion.agent.plist" "$WORK/deploy/launchd/com.orion.agent.plist"
  args=(--binary "$WORK/$ASSET")
  if [ -n "$CONFIG_SOURCE" ]; then args+=(--config "$CONFIG_SOURCE"); else args+=(--core-url "$CORE_URL"); fi
  [ -n "$TOKEN_FILE" ] && args+=(--enrollment-token-file "$TOKEN_FILE")
  [ "$START_SERVICE" = "false" ] && args+=(--no-start)
  [ "$OVERWRITE_CONFIG" = "true" ] && args+=(--overwrite-config)
  # shellcheck disable=SC2086
  (cd "$WORK" && $SUDO ./orion-agent-install.sh ${args[@]+"${args[@]}"})
fi

step "Health check"
if [ "$START_SERVICE" = "true" ]; then
  sleep 2
  if ! "$BIN_DIR/orion-agent" doctor; then
    printf '\nInstalled, but doctor found problems. Each failure above names the command that fixes it.\n'
    exit 0
  fi
fi
printf '\nOrion Agent is installed. Next: orion-agent status\n'
