#!/usr/bin/env bash

set -euo pipefail

REPO="sunday-studio/orion"
BRANCH="main"
VERSION="latest"
CORE_URL=""
CONFIG_SOURCE=""
START_SERVICE="true"
OVERWRITE_CONFIG="false"
DRY_RUN="false"

usage() {
  printf '%s\n' "Usage: curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- --core-url http://core:8999"
  printf '%s\n' ""
  printf '%s\n' "Options:"
  printf '%s\n' "  --core-url URL       Core URL written to config when --config is not provided."
  printf '%s\n' "  --config PATH        Existing local config file to install."
  printf '%s\n' "  --config-url URL     Download a config file and install it."
  printf '%s\n' "  --version VERSION    Orion release version to pin. Defaults to latest."
  printf '%s\n' "  --repo OWNER/REPO    GitHub repository. Defaults to sunday-studio/orion."
  printf '%s\n' "  --branch BRANCH      Raw GitHub branch for installer files. Defaults to main."
  printf '%s\n' "  --no-start           Install files without starting the service."
  printf '%s\n' "  --overwrite-config   Replace an existing installed config."
  printf '%s\n' "  --dry-run            Print install actions without changing the system."
  printf '%s\n' "  -h, --help           Show this help."
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
  printf 'Downloading %s\n' "$url"
  curl -fsSL "$url" -o "$output"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --core-url)
      CORE_URL="${2:-}"
      shift 2
      ;;
    --config)
      CONFIG_SOURCE="${2:-}"
      shift 2
      ;;
    --config-url)
      CONFIG_URL="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --branch)
      BRANCH="${2:-}"
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

if [ "$DRY_RUN" != "true" ] && [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "Run this installer with sudo so it can install the Agent service." >&2
  exit 1
fi

if [ -z "$CONFIG_SOURCE" ] && [ -z "${CONFIG_URL:-}" ] && [ -z "$CORE_URL" ]; then
  printf '%s\n' "--core-url, --config, or --config-url is required." >&2
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

OS="$(detect_os)"
ARCH="$(detect_arch)"
ASSET="orion-agent-${OS}-${ARCH}"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${BRANCH}"
RELEASE_BASE="https://github.com/${REPO}/releases/download/${VERSION}"
if [ "$VERSION" = "latest" ]; then
  RELEASE_BASE="https://github.com/${REPO}/releases/latest/download"
fi
WORK_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

mkdir -p "$WORK_DIR/deploy/scripts" "$WORK_DIR/deploy/systemd" "$WORK_DIR/deploy/launchd"

INSTALLER="$WORK_DIR/deploy/scripts/agent-install.sh"
BINARY="$WORK_DIR/$ASSET"
download "$RAW_BASE/deploy/scripts/agent-install.sh" "$INSTALLER"
download "$RAW_BASE/deploy/systemd/orion-agent.service" "$WORK_DIR/deploy/systemd/orion-agent.service"
download "$RAW_BASE/deploy/launchd/com.orion.agent.plist" "$WORK_DIR/deploy/launchd/com.orion.agent.plist"
download "$RELEASE_BASE/$ASSET" "$BINARY"
chmod +x "$INSTALLER" "$BINARY"

if [ -n "${CONFIG_URL:-}" ]; then
  CONFIG_SOURCE="$WORK_DIR/config.yaml"
  download "$CONFIG_URL" "$CONFIG_SOURCE"
fi

ARGS=("--binary" "$BINARY")
if [ -n "$CONFIG_SOURCE" ]; then
  ARGS+=("--config" "$CONFIG_SOURCE")
else
  ARGS+=("--core-url" "$CORE_URL")
fi
if [ "$START_SERVICE" = "false" ]; then
  ARGS+=("--no-start")
fi
if [ "$OVERWRITE_CONFIG" = "true" ]; then
  ARGS+=("--overwrite-config")
fi
if [ "$DRY_RUN" = "true" ]; then
  ARGS+=("--dry-run")
fi

cd "$WORK_DIR"
"$INSTALLER" "${ARGS[@]}"
