#!/usr/bin/env bash

set -euo pipefail

REPO="sunday-studio/orion"
VERSION="${ORION_VERSION:-latest}"
CORE_URL="${ORION_CORE_URL:-}"
# Override the release download location (mirrors, air-gapped installs, tests).
RELEASE_BASE_OVERRIDE="${ORION_RELEASE_BASE_URL:-}"
# Verification: checksums are always required. The cosign signature on SHA256SUMS is
# checked when cosign is installed and required when ORION_REQUIRE_SIGNATURE=true.
SKIP_VERIFY="false"
REQUIRE_SIGNATURE="${ORION_REQUIRE_SIGNATURE:-false}"
CONFIG_SOURCE="${ORION_AGENT_CONFIG:-}"
CONFIG_URL="${ORION_AGENT_CONFIG_URL:-}"
START_SERVICE="true"
OVERWRITE_CONFIG="false"
DRY_RUN="false"
BOOTSTRAP_STEP=0

usage() {
  printf '%s\n' "Usage: curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash"
  printf '%s\n' ""
  printf '%s\n' "Options:"
  printf '%s\n' "  --core-url URL       Core URL written to config when --config is not provided."
  printf '%s\n' "  --config PATH        Existing local config file to install."
  printf '%s\n' "  --config-url URL     Download a config file and install it."
  printf '%s\n' "  --version VERSION    Orion release version to pin. Defaults to latest."
  printf '%s\n' "  --repo OWNER/REPO    GitHub repository. Defaults to sunday-studio/orion."
  printf '%s\n' "  --no-start           Install files without starting the service."
  printf '%s\n' "  --overwrite-config   Replace an existing installed config."
  printf '%s\n' "  --dry-run            Print install actions without changing the system."
  printf '%s\n' "  --require-signature  Fail unless the release signature verifies with cosign."
  printf '%s\n' "  --skip-verify        Install without checksum verification (old releases only; unsafe)."
  printf '%s\n' "  -h, --help           Show this help."
}

step() {
  BOOTSTRAP_STEP=$((BOOTSTRAP_STEP + 1))
  printf '\n[%02d] %s\n' "$BOOTSTRAP_STEP" "$1"
}

ok() {
  printf '     ok: %s\n' "$1"
}

info() {
  printf '     %s\n' "$1"
}

require_value() {
  local flag="$1"
  local value="${2:-}"

  if [ -z "$value" ]; then
    printf '%s requires a value.\n' "$flag" >&2
    usage >&2
    exit 1
  fi
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf '%s\n' "linux" ;;
    Darwin) printf '%s\n' "darwin" ;;
    *)
      printf 'Unsupported OS: %s\n' "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf '%s\n' "amd64" ;;
    arm64|aarch64) printf '%s\n' "arm64" ;;
    *)
      printf 'Unsupported architecture: %s\n' "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

download() {
  url="$1"
  output="$2"
  info "download: $url"
  curl -fsSL "$url" -o "$output"
  ok "saved $(basename "$output")"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# verify_asset LOCAL_PATH PUBLISHED_NAME: compare LOCAL_PATH's sha256 with its SHA256SUMS entry.
verify_asset() {
  local path="$1" name="$2" expected actual
  expected="$(awk -v n="$name" '$2 == n || $2 == "*" n {print $1; exit}' "$WORK_DIR/SHA256SUMS")"
  if [ -z "$expected" ]; then
    printf 'No checksum for %s in SHA256SUMS; refusing to install.\n' "$name" >&2
    exit 1
  fi
  actual="$(sha256_of "$path")"
  if [ "$expected" != "$actual" ]; then
    printf 'Checksum mismatch for %s (expected %s, got %s); refusing to install.\n' "$name" "$expected" "$actual" >&2
    exit 1
  fi
  ok "checksum verified: $name"
}

verify_signature() {
  if command -v cosign >/dev/null 2>&1; then
    download "$RELEASE_BASE/SHA256SUMS.sig" "$WORK_DIR/SHA256SUMS.sig"
    download "$RELEASE_BASE/SHA256SUMS.pem" "$WORK_DIR/SHA256SUMS.pem"
    cosign verify-blob \
      --certificate "$WORK_DIR/SHA256SUMS.pem" \
      --signature "$WORK_DIR/SHA256SUMS.sig" \
      --certificate-identity-regexp "^https://github.com/${REPO}/\.github/workflows/agent-binaries\.yml@" \
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
      "$WORK_DIR/SHA256SUMS" >/dev/null 2>&1 || {
        printf 'Release signature verification failed; refusing to install.\n' >&2
        exit 1
      }
    ok "release signature verified (cosign)"
  elif [ "$REQUIRE_SIGNATURE" = "true" ]; then
    printf 'cosign is required (--require-signature / ORION_REQUIRE_SIGNATURE) but is not installed.\n' >&2
    exit 1
  else
    info "signature not checked: install cosign and use --require-signature to verify who published this release"
  fi
}

prompt_core_url() {
  if [ -n "$CORE_URL" ] || [ -n "$CONFIG_SOURCE" ] || [ -n "$CONFIG_URL" ]; then
    return
  fi
  if [ ! -r /dev/tty ]; then
    return
  fi
  printf '%s' "Orion Core URL: " >/dev/tty
  IFS= read -r CORE_URL </dev/tty
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --core-url)
      require_value "$1" "${2:-}"
      CORE_URL="${2:-}"
      shift 2
      ;;
    --config)
      require_value "$1" "${2:-}"
      CONFIG_SOURCE="${2:-}"
      shift 2
      ;;
    --config-url)
      require_value "$1" "${2:-}"
      CONFIG_URL="${2:-}"
      shift 2
      ;;
    --version)
      require_value "$1" "${2:-}"
      VERSION="${2:-}"
      shift 2
      ;;
    --repo)
      require_value "$1" "${2:-}"
      REPO="${2:-}"
      shift 2
      ;;
    --no-start)
      START_SERVICE="false"
      shift
      ;;
    --overwrite-config)
      OVERWRITE_CONFIG="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    --require-signature)
      REQUIRE_SIGNATURE="true"
      shift
      ;;
    --skip-verify)
      SKIP_VERIFY="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown option: %s\n' "$1" >&2
      usage
      exit 1
      ;;
  esac
done

prompt_core_url

if [ -z "$CONFIG_SOURCE" ] && [ -z "${CONFIG_URL:-}" ] && [ -z "$CORE_URL" ]; then
  printf '%s\n' "Core URL is required. Run interactively, set ORION_CORE_URL, or pass --core-url." >&2
  usage
  exit 1
fi

if [ -n "$CONFIG_SOURCE" ]; then
  case "$CONFIG_SOURCE" in
    /*) ;;
    *) CONFIG_SOURCE="$(pwd)/$CONFIG_SOURCE" ;;
  esac
fi

require_cmd curl
require_cmd mktemp

printf '%s\n' "Orion Agent bootstrap installer"
if [ "$DRY_RUN" = "true" ]; then
  info "mode: dry run"
fi

SUDO=()
if [ "$DRY_RUN" != "true" ] && [ "$(id -u)" -ne 0 ]; then
  step "Privilege"
  require_cmd sudo
  SUDO=(sudo)
  ok "sudo available"
fi

step "Platform"
OS="$(detect_os)"
ARCH="$(detect_arch)"
ASSET="orion-agent-${OS}-${ARCH}"
ok "detected $OS/$ARCH"

RELEASE_BASE="https://github.com/${REPO}/releases/download/${VERSION}"
if [ "$VERSION" = "latest" ]; then
  RELEASE_BASE="https://github.com/${REPO}/releases/latest/download"
fi
if [ -n "$RELEASE_BASE_OVERRIDE" ]; then
  RELEASE_BASE="${RELEASE_BASE_OVERRIDE%/}"
fi
if [ "$VERSION" = "latest" ] && [ -z "$RELEASE_BASE_OVERRIDE" ]; then
  info "warning: installing 'latest'; pin a release with --version vX.Y.Z for reproducible installs"
fi
info "release: $REPO@$VERSION"
WORK_DIR="$(mktemp -d)"
info "workspace: $WORK_DIR"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$WORK_DIR/deploy/scripts" "$WORK_DIR/deploy/systemd" "$WORK_DIR/deploy/launchd"

INSTALLER="$WORK_DIR/deploy/scripts/agent-install.sh"
BINARY="$WORK_DIR/$ASSET"
step "Download release assets"
download "$RELEASE_BASE/orion-agent-install.sh" "$INSTALLER"
download "$RELEASE_BASE/orion-agent-systemd.service" "$WORK_DIR/deploy/systemd/orion-agent.service"
download "$RELEASE_BASE/com.orion.agent.plist" "$WORK_DIR/deploy/launchd/com.orion.agent.plist"
download "$RELEASE_BASE/$ASSET" "$BINARY"

if [ "$SKIP_VERIFY" = "true" ]; then
  info "warning: --skip-verify set; release assets are NOT verified"
else
  step "Verify release assets"
  download "$RELEASE_BASE/SHA256SUMS" "$WORK_DIR/SHA256SUMS"
  verify_signature
  verify_asset "$INSTALLER" "orion-agent-install.sh"
  verify_asset "$WORK_DIR/deploy/systemd/orion-agent.service" "orion-agent-systemd.service"
  verify_asset "$WORK_DIR/deploy/launchd/com.orion.agent.plist" "com.orion.agent.plist"
  verify_asset "$BINARY" "$ASSET"
fi

chmod +x "$INSTALLER" "$BINARY"
ok "assets are executable"

if [ -n "${CONFIG_URL:-}" ]; then
  step "Download config"
  CONFIG_SOURCE="$WORK_DIR/config.yaml"
  download "$CONFIG_URL" "$CONFIG_SOURCE"
fi

step "Prepare install arguments"
ARGS=("--binary" "$BINARY")
if [ -n "$CONFIG_SOURCE" ]; then
  ARGS+=("--config" "$CONFIG_SOURCE")
  info "config: $CONFIG_SOURCE"
else
  ARGS+=("--core-url" "$CORE_URL")
  info "core_url: $CORE_URL"
fi
if [ "$START_SERVICE" = "false" ]; then
  ARGS+=("--no-start")
  info "service start: disabled"
else
  info "service start: enabled"
fi
if [ "$OVERWRITE_CONFIG" = "true" ]; then
  ARGS+=("--overwrite-config")
  info "config replacement: enabled"
fi
if [ "$DRY_RUN" = "true" ]; then
  ARGS+=("--dry-run")
fi
ok "install arguments ready"

cd "$WORK_DIR"
step "Run service installer"
if [ -n "${ORION_ENROLLMENT_TOKEN:-}" ]; then
  ${SUDO[@]+"${SUDO[@]}"} env "ORION_ENROLLMENT_TOKEN=$ORION_ENROLLMENT_TOKEN" "$INSTALLER" "${ARGS[@]}"
else
  ${SUDO[@]+"${SUDO[@]}"} "$INSTALLER" "${ARGS[@]}"
fi
ok "installer finished"
