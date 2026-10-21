#!/usr/bin/env bash
# Orion Node installer (front door).
#
#   curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh | bash -s -- \
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
CONFIG_SOURCE="${ORION_NODE_CONFIG:-}"
CONFIG_URL="${ORION_NODE_CONFIG_URL:-}"
TOKEN_FILE="${ORION_ENROLLMENT_TOKEN_FILE:-}"
SKIP_VERIFY="false"
REQUIRE_SIGNATURE="${ORION_REQUIRE_SIGNATURE:-false}"
START_SERVICE="true"
OVERWRITE_CONFIG="false"
DRY_RUN="false"
VERIFY_ONLY="false"
USER_MODE="false"
ASSUME_YES="false"
PURGE="false"
ACTION="install"
STEP=0

usage() {
  cat <<'EOF'
Usage: orion-node-installer.sh [install|uninstall] [options]

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
  --require-signature          Abort unless the cosign signature on SHA256SUMS verifies.
  --skip-verify                Install without checksum verification (old releases only; unsafe).
  --yes                        Do not prompt.
  --purge                      With uninstall: also remove config, state, logs and the service user.
  --dry-run                    Print the install plan only; do not download or change anything.
  --verify-only                Download and verify release assets, then exit without installing.
  -h, --help                   Show this help.

Exit codes: 0 ok, 1 usage or failed install, 2 verification failed (nothing installed).
EOF
}

# ---------------------------------------------------------------- output
# The installer prints one line per finished step and a result card. Colour is
# only used on a terminal (and never with NO_COLOR); spinners are a carriage
# return redraw, and without a terminal each step prints one plain line.
ESC="$(printf '\033')"
TTY="false"; [ -t 1 ] && TTY="true"
if [ "$TTY" = "true" ] && [ -z "${NO_COLOR:-}" ]; then
  C_OK="${ESC}[32m"; C_BAD="${ESC}[31m"; C_WARN="${ESC}[33m"; C_DIM="${ESC}[90m"; C_BOLD="${ESC}[1m"; C_OFF="${ESC}[0m"
else
  C_OK=""; C_BAD=""; C_WARN=""; C_DIM=""; C_BOLD=""; C_OFF=""
fi
PANEL_W=86
HELP_URL="https://orion.thesunday.studio/docs/install"
NSTEPS=0; PRINTED=0; HEADER_DONE="false"

# vlen: visible width of a string (colour codes removed, UTF-8 counted by character).
vlen() { printf '%s' "$1" | sed "s/${ESC}\[[0-9;]*m//g" | LC_ALL=C tr -d '\200-\277' | wc -c | tr -d ' '; }
pad()  { local n sp; n="$(vlen "$1")"; sp=$(( $2 - n )); [ "$sp" -lt 0 ] && sp=0; printf '%s%*s' "$1" "$sp" ''; }
rule() { printf '%*s' "$1" '' | sed 's/ /─/g'; }
now_ms() { local t; t="$(date +%s%N 2>/dev/null || true)"; case "$t" in ""|*N) echo $(( $(date +%s) * 1000 )) ;; *) echo $(( t / 1000000 )) ;; esac; }
secs() { local ms="$1"; printf '%d.%d s' $(( ms / 1000 )) $(( (ms % 1000) / 100 )); }

box_top() { # title right colour
  local t=" $1 " r="" dash; [ -n "${2:-}" ] && r=" $2 "
  dash=$(( PANEL_W - 2 - $(vlen "$t") - $(vlen "$r") )); [ "$dash" -lt 1 ] && dash=1
  printf ' %s╭%s%s%s%s%s%s%s╮%s\n' "${3:-$C_DIM}" "$C_OFF" "$C_DIM" "$t" "${3:-$C_DIM}" "$(rule "$dash")" "$C_DIM" "$r" "$C_OFF"
}
box_row() { # text colour
  printf ' %s│%s%s%s│%s\n' "${2:-$C_DIM}" "$C_OFF" "$(pad " $1" $(( PANEL_W - 2 )))" "${2:-$C_DIM}" "$C_OFF"
}
box_bottom() { printf ' %s╰%s╯%s\n' "${1:-$C_DIM}" "$(rule $(( PANEL_W - 2 )))" "$C_OFF"; }

working() { [ "$TTY" = "true" ] && printf '\r%s  ⠋ %s%s\033[K' "$C_DIM" "$1" "$C_OFF" || true; }
clear_working() { [ "$TTY" = "true" ] && printf '\r\033[K' || true; }

# header prints the title once, with the release and platform on the right.
header() {
  [ "$HEADER_DONE" = "true" ] && return 0
  HEADER_DONE="true"
  local left=" ${C_OK}◆${C_OFF} ${C_BOLD}Orion Node installer${C_OFF}" right="${1:-$VERSION} · $OS/$ARCH · $INIT"
  printf '\n%s%*s%s%s%s\n\n' "$left" $(( PANEL_W - $(vlen "$left") - $(vlen "$right") )) '' "$C_DIM" "$right" "$C_OFF"
}

step() { STEP=$((STEP + 1)); }
# record: finish one step (mark is ok or warn). Steps are shown together once
# the install is done, or when something fails, so a no-op re-run stays short.
record() { # mark name detail ms
  S_MARK[$NSTEPS]="$1"; S_NAME[$NSTEPS]="$2"; S_DETAIL[$NSTEPS]="${3:-}"; S_MS[$NSTEPS]="${4:-0}"; NSTEPS=$((NSTEPS + 1))
}
mark_of() { if [ "$1" = "warn" ]; then printf '%s!%s' "$C_WARN" "$C_OFF"; else printf '%s✓%s' "$C_OK" "$C_OFF"; fi; }
print_step() { # index style
  local i="$1" m t=""; m="$(mark_of "${S_MARK[$i]}")"
  [ "${S_MS[$i]}" -gt 0 ] && t="$(secs "${S_MS[$i]}")"
  if [ "$2" = "box" ]; then
    local d="${S_DETAIL[$i]}"; [ "$(vlen "$d")" -gt 42 ] && d="$(printf '%s' "$d" | cut -c1-41)…"
    box_row "$m $(pad "${S_NAME[$i]}" 30)${C_DIM}$(pad "$d" 43)$(printf '%5s' "$t")${C_OFF}   "
  else
    printf '   %s %s%s%s%s\n' "$m" "$(pad "${S_NAME[$i]}" 30)" "$C_DIM" "${S_DETAIL[$i]}" "$C_OFF"
  fi
}
flush_steps() { # print any recorded step not yet shown (used before an error)
  local i=$PRINTED; while [ "$i" -lt "$NSTEPS" ]; do print_step "$i" plain; i=$((i + 1)); done; PRINTED=$NSTEPS
}
show_steps_box() {
  header "${RELEASE_VERSION:-$VERSION}"
  if [ "$TTY" != "true" ]; then flush_steps; return 0; fi
  local i=0; box_top "Steps" "$NSTEPS of $NSTEPS"
  while [ "$i" -lt "$NSTEPS" ]; do print_step "$i" box; i=$((i + 1)); done
  box_bottom; PRINTED=$NSTEPS
}

ok()   { printf '   %s✓%s %s\n' "$C_OK" "$C_OFF" "$1"; }
info() { printf '     %s%s%s\n' "$C_DIM" "$1" "$C_OFF"; }
# die prints what finished, the failure, the fix and where to read more.
die() {
  clear_working >&2; flush_steps >&2
  printf '   %s✗%s %s\n' "$C_BAD" "$C_OFF" "$1" >&2
  if [ -n "${2:-}" ]; then printf '\n %sFix%s   %s\n %sHelp%s  %s\n' "$C_DIM" "$C_OFF" "$2" "$C_DIM" "$C_OFF" "$HELP_URL" >&2; fi
  exit "${3:-1}"
}
# fail_step is die for a named step: "✗ Verified   reason" plus detail lines.
fail_step() { # name reason fix code detail...
  local name="$1" reason="$2" fix="$3" code="$4"; shift 4
  clear_working >&2; flush_steps >&2
  printf '   %s✗%s %s%s%s%s\n' "$C_BAD" "$C_OFF" "$(pad "$name" 22)" "$C_BAD" "$reason" "$C_OFF" >&2
  while [ "$#" -gt 0 ]; do printf '     %s%s%s\n' "$C_DIM" "$1" "$C_OFF" >&2; shift; done
  printf '\n %sFix%s   %s\n %sHelp%s  %s\n' "$C_DIM" "$C_OFF" "$fix" "$C_DIM" "$C_OFF" "$HELP_URL" >&2
  exit "$code"
}

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
    --skip-verify) SKIP_VERIFY="true"; shift ;;
    --require-signature) REQUIRE_SIGNATURE="true"; shift ;;
    --user) USER_MODE="true"; shift ;;
    --no-start) START_SERVICE="false"; shift ;;
    --overwrite-config) OVERWRITE_CONFIG="true"; shift ;;
    --yes|-y) ASSUME_YES="true"; shift ;;
    --purge) PURGE="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    --verify-only) VERIFY_ONLY="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; usage >&2; exit 1 ;;
  esac
done

# ---------------------------------------------------------------- platform
detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) die "unsupported OS: $(uname -s)" "Orion Node supports Linux and macOS" ;;
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
ASSET="orion-node-${OS}-${ARCH}"

if [ -z "$BASE_URL" ]; then
  if [ "$VERSION" = "latest" ]; then BASE_URL="https://github.com/${REPO}/releases/latest/download"
  else BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"; fi
fi

# ---------------------------------------------------------------- uninstall
if [ "$ACTION" = "uninstall" ]; then
  require_cmd curl
  WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
  printf 'Orion Node uninstall\n'
  curl -fsSL "$BASE_URL/SHA256SUMS" -o "$WORK/SHA256SUMS"
  curl -fsSL "$BASE_URL/orion-node-uninstall.sh" -o "$WORK/uninstall.sh"
  if command -v cosign >/dev/null 2>&1 && [ "$SKIP_VERIFY" != "true" ]; then
    curl -fsSL "$BASE_URL/SHA256SUMS.sig" -o "$WORK/SHA256SUMS.sig" && curl -fsSL "$BASE_URL/SHA256SUMS.pem" -o "$WORK/SHA256SUMS.pem" \
      && cosign verify-blob --certificate "$WORK/SHA256SUMS.pem" --signature "$WORK/SHA256SUMS.sig" \
        --certificate-identity-regexp "^https://github.com/${REPO}/\.github/workflows/node-binaries\.yml@" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" "$WORK/SHA256SUMS" >/dev/null 2>&1 \
      || die "Release signature verification failed; refusing to run the uninstaller." "" 2
  elif [ "$REQUIRE_SIGNATURE" = "true" ]; then
    die "cosign is required (--require-signature) but is not installed." "install cosign" 2
  fi
  want="$(sha256_of "$WORK/uninstall.sh")"
  grep -Eq "^$want[[:space:]]+\*?orion-node-uninstall\.sh$" "$WORK/SHA256SUMS" || die "uninstaller failed checksum verification" "" 2
  chmod +x "$WORK/uninstall.sh"
  args=(); [ "$DRY_RUN" = "true" ] && args+=(--dry-run); [ "$ASSUME_YES" = "true" ] && args+=(--yes); [ "$PURGE" = "true" ] && args+=(--purge); [ "$USER_MODE" = "true" ] && args+=(--user)
  rc=0
  if [ "$(id -u)" -ne 0 ] && [ "$USER_MODE" != "true" ]; then sudo "$WORK/uninstall.sh" ${args[@]+"${args[@]}"} || rc=$?
  else "$WORK/uninstall.sh" ${args[@]+"${args[@]}"} || rc=$?; fi
  exit "$rc"
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

if [ "$DRY_RUN" = "true" ]; then
  header
  printf '  Plan\n'
  printf '  ● system     %s/%s, %s\n' "$OS" "$ARCH" "$INIT"
  printf '  ● download   %s, static\n' "$ASSET"
  printf '  ● verify     sha256%s\n' "$([ "$REQUIRE_SIGNATURE" = "true" ] && printf ' and signature' || printf '')"
  printf '  ● create     %s, %s, %s\n' "$([ "$USER_MODE" = "true" ] && printf 'user directories' || printf 'user orion')" "$(dirname "$CFG")" "$STATE_DIR"
  printf '  ● install    orion-node (%s)\n' "$INIT"
  printf '  ● enroll     %s\n' "$CORE_URL"
  printf '\n  dry run: this plan makes no changes.\n'
  exit 0
fi

if [ "$INIT" = "none" ] && [ "$USER_MODE" != "true" ]; then
  die "no supported init system found (systemd or launchd)" "use --user, or run the node under your own supervisor: orion-node run"
fi

# ---------------------------------------------------------------- download + verify
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
chmod 700 "$WORK"

fetch() { curl -fsSL "$1" -o "$2" || die "download failed: $1" "check the version, network access, and --base-url"; }

step "Download"
T0="$(now_ms)"; working "Downloading orion-node…"
fetch "$BASE_URL/SHA256SUMS" "$WORK/SHA256SUMS"
fetch "$BASE_URL/$ASSET" "$WORK/$ASSET"
if [ "$USER_MODE" != "true" ]; then
  fetch "$BASE_URL/orion-node-install.sh" "$WORK/orion-node-install.sh"
  fetch "$BASE_URL/orion-node-systemd.service" "$WORK/orion-node-systemd.service"
  fetch "$BASE_URL/com.orion.node.plist" "$WORK/com.orion.node.plist"
fi
clear_working
asset_bytes="$(wc -c < "$WORK/$ASSET" | tr -d ' ')"
RELEASE_VERSION="$(printf '%s' "$VERSION" | sed 's/^v//')"   # for "latest", read from the binary after it is verified
record ok "Downloaded" "$(awk -v b="$asset_bytes" 'BEGIN{printf "%.1f MB", b/1048576}')" $(( $(now_ms) - T0 ))

step "Verify"
T0="$(now_ms)"
if [ "$SKIP_VERIFY" = "true" ]; then
  SIGNED=""; VERIFY_DETAIL="WARNING: contents are NOT verified (--skip-verify)"; VERIFY_MARK="warn"
else
  if command -v cosign >/dev/null 2>&1; then
    fetch "$BASE_URL/SHA256SUMS.sig" "$WORK/SHA256SUMS.sig"
    fetch "$BASE_URL/SHA256SUMS.pem" "$WORK/SHA256SUMS.pem"
    cosign verify-blob --certificate "$WORK/SHA256SUMS.pem" --signature "$WORK/SHA256SUMS.sig" \
      --certificate-identity-regexp "^https://github.com/${REPO}/\.github/workflows/node-binaries\.yml@" \
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" "$WORK/SHA256SUMS" >/dev/null 2>&1 \
      || die "Release signature verification failed; refusing to install." "do not install; report this at https://github.com/${REPO}/issues" 2
    SIGNED="yes"
  elif [ "$REQUIRE_SIGNATURE" = "true" ]; then
    die "cosign is required (--require-signature / ORION_REQUIRE_SIGNATURE) but is not installed." "install cosign" 2
  else
    SIGNED="no"
  fi

  verify_asset() {
    local file="$1" name="$2" want got
    want="$(awk -v n="$name" '$2==n||$2=="*"n{print $1; exit}' "$WORK/SHA256SUMS")"
    [ -n "$want" ] || die "No checksum for $name in SHA256SUMS; refusing to install." "this release looks incomplete; try another --version" 2
    got="$(sha256_of "$file")"
    [ "$got" = "$want" ] || fail_step "Verified" "checksum does not match" "run the command again, or pin a version with --version ${RELEASE_VERSION:-X.Y.Z}" 2 \
      "expected $(printf '%s' "$want" | cut -c1-6)…   got $(printf '%s' "$got" | cut -c1-6)…   ($name)" \
      "The download may be corrupted or tampered with. Nothing was installed."
  }
  verify_asset "$WORK/$ASSET" "$ASSET"
  if [ "$USER_MODE" != "true" ]; then
    verify_asset "$WORK/orion-node-install.sh" "orion-node-install.sh"
    verify_asset "$WORK/orion-node-systemd.service" "orion-node-systemd.service"
    verify_asset "$WORK/com.orion.node.plist" "com.orion.node.plist"
  fi
  VERIFY_MARK="ok"; VERIFY_DETAIL="sha256"
  case "$SIGNED" in
    yes) VERIFY_DETAIL="sha256 and cosign signature" ;;
    no)  VERIFY_DETAIL="sha256, signature not checked" ;;
  esac
fi
record "$VERIFY_MARK" "Verified" "$VERIFY_DETAIL" $(( $(now_ms) - T0 ))
chmod +x "$WORK/$ASSET"
if "$WORK/$ASSET" version >/dev/null 2>&1; then
  [ "$RELEASE_VERSION" = "latest" ] && RELEASE_VERSION="$("$WORK/$ASSET" version 2>/dev/null | awk '{print $NF}')"
elif [ "$SKIP_VERIFY" = "true" ]; then info "warning: the downloaded binary did not run here"
else die "the downloaded binary does not run on this machine" "check the architecture ($OS/$ARCH)" 2; fi

if [ "$VERIFY_ONLY" = "true" ]; then
  flush_steps
  printf '\n  %sVerification complete; no files were installed.%s\n' "$C_DIM" "$C_OFF"
  exit 0
fi

# A verified release identical to a complete existing install is a no-op. A
# token, config replacement, or stopped/missing service still proceeds because
# the operator explicitly asked to change or repair that part of the install.
already_installed() {
  [ -x "$BIN_DIR/orion-node" ] || return 1
  [ -f "$CFG" ] || return 1
  [ -f "$STATE_DIR/state.db" ] || return 1
  [ -z "$TOKEN_FILE" ] || return 1
  [ -z "$CONFIG_SOURCE" ] || return 1
  [ -z "$CONFIG_URL" ] || return 1
  [ "$OVERWRITE_CONFIG" != "true" ] || return 1
  local installed_version release_version
  installed_version="$($BIN_DIR/orion-node version 2>/dev/null || true)"
  release_version="$($WORK/$ASSET version 2>/dev/null || true)"
  [ -n "$installed_version" ] && [ "$installed_version" = "$release_version" ] || return 1
  if [ "$START_SERVICE" = "true" ]; then
    if [ "$USER_MODE" = "true" ] && [ "$INIT" = "systemd" ]; then
      systemctl --user is-active --quiet orion-node 2>/dev/null || return 1
    elif [ "$USER_MODE" = "true" ] && [ "$INIT" = "launchd" ]; then
      launchctl print "gui/$(id -u)/com.orion.node.user" >/dev/null 2>&1 || return 1
    elif [ "$OS" = "linux" ]; then
      systemctl is-active --quiet orion-node 2>/dev/null || return 1
    else
      launchctl print system/com.orion.node >/dev/null 2>&1 || return 1
    fi
  fi
  return 0
}

if [ -n "$CONFIG_URL" ]; then fetch "$CONFIG_URL" "$WORK/config.yaml"; CONFIG_SOURCE="$WORK/config.yaml"; fi

# One-time token from the environment goes to a private file, never argv.
if [ -z "$TOKEN_FILE" ] && [ -n "${ORION_ENROLLMENT_TOKEN:-}" ]; then
  ( umask 077; printf '%s' "$ORION_ENROLLMENT_TOKEN" > "$WORK/enroll-token" )
  TOKEN_FILE="$WORK/enroll-token"
fi

if already_installed; then
  printf '\n'
  ok "$(pad "Already on ${RELEASE_VERSION:-$VERSION}" 30)${C_DIM}nothing to do${C_OFF}"
  ok "$(pad "Service" 30)${C_DIM}running${C_OFF}"
  printf '\n   %s▸%s %sorion-node status%s\n' "$C_OK" "$C_OFF" "$C_BOLD" "$C_OFF"
  exit 0
fi

# ---------------------------------------------------------------- install
if [ "$DRY_RUN" = "true" ] && [ "$USER_MODE" != "true" ]; then
  step "Dry run"
  chmod +x "$WORK/orion-node-install.sh"
  args=(--binary "$WORK/$ASSET" --dry-run)
  [ -n "$CONFIG_SOURCE" ] && args+=(--config "$CONFIG_SOURCE") || args+=(--core-url "$CORE_URL")
  (cd "$WORK" && mkdir -p deploy/systemd deploy/launchd && cp orion-node-systemd.service deploy/systemd/orion-node.service && cp com.orion.node.plist deploy/launchd/ && ./orion-node-install.sh ${args[@]+"${args[@]}"})
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
  # The node was called the agent before the rename: stop and remove the old per-user service and binary.
  systemctl --user disable --now orion-agent >/dev/null 2>&1 || true
  rm -f "$HOME/.config/systemd/user/orion-agent.service"
  launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.orion.agent.user.plist" >/dev/null 2>&1 || true
  rm -f "$HOME/Library/LaunchAgents/com.orion.agent.user.plist" "$BIN_DIR/orion-agent"
  install -m 0755 "$WORK/$ASSET" "$BIN_DIR/orion-node"
  record ok "Installed" "$BIN_DIR/orion-node"
  if [ -f "$CFG" ] && [ "$OVERWRITE_CONFIG" != "true" ]; then true
  elif [ -n "$CONFIG_SOURCE" ]; then install -m 0600 "$CONFIG_SOURCE" "$CFG"
  else printf 'core_url: %s\ninterval: 60s\nlogging:\n  path: %s\nmonitors: []\n' "$CORE_URL" "$STATE_DIR/node.log" > "$CFG"; chmod 600 "$CFG"; fi
  export ORION_NODE_SOCKET="$STATE_DIR/node.sock"
  if [ -n "$TOKEN_FILE" ]; then
    working "Enrolling this host…"; T0="$(now_ms)"
    "$BIN_DIR/orion-node" enroll --config "$CFG" --state "$STATE_DIR/state.db" --core-url "${CORE_URL:-$(awk '/^core_url:/{print $2}' "$CFG")}" --token-file "$TOKEN_FILE" >"$WORK/enroll.log" 2>&1 \
      || { cat "$WORK/enroll.log" >&2; die "enrollment failed" "create a new token in Console > Add server"; }
    clear_working; ENROLLED_MS=$(( $(now_ms) - T0 ))
  else
    "$BIN_DIR/orion-node" state init --state "$STATE_DIR/state.db" >/dev/null
  fi
  step "Service (per-user)"
  if [ "$INIT" = "systemd" ]; then
    unit_dir="$HOME/.config/systemd/user"; mkdir -p "$unit_dir"
    cat > "$unit_dir/orion-node.service" <<EOF
[Unit]
Description=Orion Node (user)
After=network-online.target

[Service]
ExecStart=$BIN_DIR/orion-node run --config $CFG --state $STATE_DIR/state.db --socket $STATE_DIR/node.sock
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload 2>/dev/null || true
    if [ "$START_SERVICE" = "true" ]; then systemctl --user enable --now orion-node; fi
  elif [ "$INIT" = "launchd" ]; then
    plist="$HOME/Library/LaunchAgents/com.orion.node.user.plist"; mkdir -p "$(dirname "$plist")"
    cat > "$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.orion.node.user</string>
<key>ProgramArguments</key><array>
<string>$BIN_DIR/orion-node</string><string>run</string>
<string>--config</string><string>$CFG</string>
<string>--state</string><string>$STATE_DIR/state.db</string>
<string>--socket</string><string>$STATE_DIR/node.sock</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
    if [ "$START_SERVICE" = "true" ]; then launchctl bootout "gui/$(id -u)" "$plist" >/dev/null 2>&1 || true; launchctl bootstrap "gui/$(id -u)" "$plist"; fi
  else
    true
  fi
  if [ "$START_SERVICE" != "true" ]; then SERVICE_DETAIL="installed, not started (--no-start)"
  elif [ "$INIT" = "systemd" ]; then SERVICE_DETAIL="systemd user unit, runs as you"
  elif [ "$INIT" = "launchd" ]; then SERVICE_DETAIL="launchd job, runs as you"
  else SERVICE_DETAIL="no init system; run: $BIN_DIR/orion-node run"; fi
  record ok "Service" "$SERVICE_DETAIL"
else
  step "Install"
  SUDO=""; [ "$(id -u)" -ne 0 ] && { require_cmd sudo; SUDO="sudo"; }
  chmod +x "$WORK/orion-node-install.sh"
  mkdir -p "$WORK/deploy/systemd" "$WORK/deploy/launchd"
  cp "$WORK/orion-node-systemd.service" "$WORK/deploy/systemd/orion-node.service"
  cp "$WORK/com.orion.node.plist" "$WORK/deploy/launchd/com.orion.node.plist"
  args=(--binary "$WORK/$ASSET")
  if [ -n "$CONFIG_SOURCE" ]; then args+=(--config "$CONFIG_SOURCE"); else args+=(--core-url "$CORE_URL"); fi
  [ -n "$TOKEN_FILE" ] && args+=(--enrollment-token-file "$TOKEN_FILE")
  [ "$START_SERVICE" = "false" ] && args+=(--no-start)
  [ "$OVERWRITE_CONFIG" = "true" ] && args+=(--overwrite-config)
  # Keep the front-door installer concise. On failure, reveal the complete
  # lower-level install transcript so no diagnostic information is lost.
  if ! (cd "$WORK" && $SUDO ./orion-node-install.sh ${args[@]+"${args[@]}"}) >"$WORK/install.log" 2>&1; then
    cat "$WORK/install.log" >&2
    die "installation failed" "fix the reported step and re-run the same command"
  fi
  record ok "Installed" "$BIN_DIR/orion-node"
  if [ "$START_SERVICE" != "true" ]; then SERVICE_DETAIL="installed, not started (--no-start)"
  elif [ "$INIT" = "systemd" ]; then SERVICE_DETAIL="systemd unit, runs as user orion"
  else SERVICE_DETAIL="launchd daemon, runs as the orion service user"; fi
  record ok "Service" "$SERVICE_DETAIL"
  ENROLLED_MS=0
fi
if [ -n "$TOKEN_FILE" ]; then
  HOST_NAME="$(hostname -s 2>/dev/null || hostname)"
  CORE_HOST="$(printf '%s' "${CORE_URL:-$(awk '/^core_url:/{print $2}' "$CFG" 2>/dev/null)}" | sed 's|^[a-z]*://||; s|/.*$||')"
  record ok "Enrolled" "$HOST_NAME with $CORE_HOST" "${ENROLLED_MS:-0}"
fi

step "Health check"
HOST_NAME="$(hostname -s 2>/dev/null || hostname)"
if [ "$START_SERVICE" = "true" ]; then
  sleep 2
  working "Waiting for the first report…"
  doctor_ok="true"
  if ! "$BIN_DIR/orion-node" doctor --no-color >"$WORK/doctor.log" 2>&1; then
    doctor_ok="false"
  fi
  clear_working
  if [ "$doctor_ok" != "true" ]; then
    show_steps_box
    printf '\n' ; sed 's/^/  /' "$WORK/doctor.log"
    die "Doctor found problems" "run orion-node doctor after applying the fixes above"
  fi
  passed="$(awk '{for(i=2;i<=NF;i++) if($i=="passed"){n=$(i-1); if(n=="checks") n=$(i-2); if(n ~ /^[0-9]+$/) v=n}} END{print v}' "$WORK/doctor.log")"
  warned="$(awk '{for(i=2;i<=NF;i++) if($i ~ /^warnings?$/ && $(i-1) ~ /^[0-9]+$/) v=$(i-1)} END{print v}' "$WORK/doctor.log")"
  found="$("$BIN_DIR/orion-node" discover --json 2>/dev/null | grep -c '"confidence"' || true)"
  doc_row="${C_OK}✓ ${passed:-all} passed${C_OFF}"
  [ -n "$warned" ] && [ "$warned" != "0" ] && doc_row="$doc_row   ${C_WARN}! $warned warning${C_OFF}"
  show_steps_box
  box_top "$HOST_NAME is connected" "●" "$C_OK"
  box_row "${C_DIM}Doctor${C_OFF}       $doc_row" "$C_OK"
  box_row "${C_DIM}Collecting${C_OFF}   CPU · memory · disk · network · load · uptime  ${C_DIM}(no setup needed)${C_OFF}" "$C_OK"
  if [ "${found:-0}" -gt 0 ] 2>/dev/null; then
    box_row "${C_DIM}Detected${C_OFF}     ${C_BOLD}$found things${C_OFF} to monitor" "$C_OK"
  fi
  box_row "" "$C_OK"
  box_row "${C_OK}▸${C_OFF} ${C_BOLD}orion-node setup${C_OFF}   ${C_DIM}or open${C_OFF} ${CORE_URL%/}/servers/$HOST_NAME" "$C_OK"
  box_bottom "$C_OK"
  printf '\n  %sNo questions were asked. Setup is optional; host metrics are already reporting.%s\n' "$C_DIM" "$C_OFF"
else
  show_steps_box
  printf '\n   %s▸%s %sorion-node service start%s\n' "$C_OK" "$C_OFF" "$C_BOLD" "$C_OFF"
fi
