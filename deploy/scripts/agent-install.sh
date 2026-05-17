#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="orion-agent"
LINUX_USER="orion"
LINUX_GROUP="orion"
MACOS_USER="_orion"
MACOS_GROUP="_orion"
INSTALL_DIR="/usr/local/bin"

CORE_URL=""
BINARY_PATH=""
CONFIG_SOURCE=""
START_SERVICE="true"
OVERWRITE_CONFIG="false"
DRY_RUN="false"

usage() {
  printf '%s\n' "Usage: sudo ./deploy/scripts/agent-install.sh --core-url http://core:8999 [options]"
  printf '%s\n' ""
  printf '%s\n' "Options:"
  printf '%s\n' "  --core-url URL       Core URL written to config when creating a new config."
  printf '%s\n' "  --binary PATH        Agent binary to install. Defaults to ./apps/agent/orion-agent, then ./orion-agent."
  printf '%s\n' "  --config PATH        Existing config file to install instead of generating a minimal one."
  printf '%s\n' "  --no-start           Install files without starting the service."
  printf '%s\n' "  --overwrite-config   Replace an existing installed config."
  printf '%s\n' "  --dry-run            Print install actions without changing the system."
  printf '%s\n' "  -h, --help           Show this help."
}

run_cmd() {
  if [ "$DRY_RUN" = "true" ]; then
    printf '+ %q' "$1"
    shift
    for arg in "$@"; do
      printf ' %q' "$arg"
    done
    printf '\n'
    return 0
  fi
  "$@"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --core-url)
      CORE_URL="${2:-}"
      shift 2
      ;;
    --binary)
      BINARY_PATH="${2:-}"
      shift 2
      ;;
    --config)
      CONFIG_SOURCE="${2:-}"
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
  printf '%s\n' "This script must be run as root." >&2
  exit 1
fi

detect_os() {
  case "$(uname -s)" in
    Linux) printf '%s\n' "linux" ;;
    Darwin) printf '%s\n' "macos" ;;
    *)
      printf '%s\n' "Unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

resolve_binary() {
  if [ -n "$BINARY_PATH" ]; then
    printf '%s\n' "$BINARY_PATH"
    return
  fi
  if [ -x "./apps/agent/orion-agent" ]; then
    printf '%s\n' "./apps/agent/orion-agent"
    return
  fi
  if [ -x "./orion-agent" ]; then
    printf '%s\n' "./orion-agent"
    return
  fi
  printf '%s\n' "No agent binary found. Build one with: cd apps/agent && go build -o orion-agent ." >&2
  exit 1
}

write_minimal_config() {
  config_path="$1"
  if [ -z "$CORE_URL" ]; then
    printf '%s\n' "--core-url is required when --config is not provided." >&2
    exit 1
  fi
  {
    printf 'core_url: %s\n' "$CORE_URL"
    printf 'interval: 60s\n'
    printf 'monitors: []\n'
  } > "$config_path"
}

install_config() {
  config_path="$1"
  owner="$2"
  group="$3"

  if [ -f "$config_path" ] && [ "$OVERWRITE_CONFIG" != "true" ]; then
    printf 'Keeping existing config: %s\n' "$config_path"
    return
  fi

  if [ -n "$CONFIG_SOURCE" ]; then
    if [ ! -f "$CONFIG_SOURCE" ]; then
      printf 'Config file does not exist: %s\n' "$CONFIG_SOURCE" >&2
      exit 1
    fi
    run_cmd install -m 0640 -o "$owner" -g "$group" "$CONFIG_SOURCE" "$config_path"
  else
    if [ "$DRY_RUN" = "true" ]; then
      printf '+ generate minimal config at %q for core %q\n' "$config_path" "$CORE_URL"
      return
    fi
    tmp_config="$(mktemp)"
    write_minimal_config "$tmp_config"
    run_cmd install -m 0640 -o "$owner" -g "$group" "$tmp_config" "$config_path"
    rm -f "$tmp_config"
  fi
}

install_linux() {
  binary="$1"
  config_dir="/etc/orion"
  state_dir="/var/lib/orion"

  if ! getent group "$LINUX_GROUP" >/dev/null; then
    run_cmd groupadd --system "$LINUX_GROUP"
  fi
  if ! id "$LINUX_USER" >/dev/null 2>&1; then
    run_cmd useradd --system --gid "$LINUX_GROUP" --home-dir "$state_dir" --shell /usr/sbin/nologin "$LINUX_USER"
  fi

  run_cmd install -d -m 0750 -o "$LINUX_USER" -g "$LINUX_GROUP" "$config_dir"
  run_cmd install -d -m 0750 -o "$LINUX_USER" -g "$LINUX_GROUP" "$state_dir"
  run_cmd install -m 0755 "$binary" "$INSTALL_DIR/orion-agent"
  install_config "$config_dir/config.yaml" "$LINUX_USER" "$LINUX_GROUP"
  run_cmd install -m 0644 "deploy/systemd/orion-agent.service" "/etc/systemd/system/$SERVICE_NAME.service"

  run_cmd systemctl daemon-reload
  run_cmd systemctl enable "$SERVICE_NAME"
  if [ "$START_SERVICE" = "true" ]; then
    run_cmd systemctl restart "$SERVICE_NAME"
  fi
}

next_macos_id() {
  dscl . -list /Users UniqueID 2>/dev/null |
    awk '$2 >= 380 && $2 < 500 { used[$2] = 1 } END { for (i = 380; i < 500; i++) if (!used[i]) { print i; exit } }'
}

ensure_macos_account() {
  account_id=""
  if [ "$DRY_RUN" = "true" ]; then
    account_id="380"
    run_cmd dscl . -create "/Groups/$MACOS_GROUP"
    run_cmd dscl . -create "/Groups/$MACOS_GROUP" PrimaryGroupID "$account_id"
    run_cmd dscl . -create "/Groups/$MACOS_GROUP" Password '*'
    run_cmd dscl . -create "/Users/$MACOS_USER"
    run_cmd dscl . -create "/Users/$MACOS_USER" UniqueID "$account_id"
    run_cmd dscl . -create "/Users/$MACOS_USER" PrimaryGroupID "$account_id"
    run_cmd dscl . -create "/Users/$MACOS_USER" NFSHomeDirectory /usr/local/var/lib/orion
    run_cmd dscl . -create "/Users/$MACOS_USER" UserShell /usr/bin/false
    run_cmd dscl . -create "/Users/$MACOS_USER" RealName "Orion Agent"
    run_cmd dscl . -create "/Users/$MACOS_USER" Password '*'
    return
  fi

  if ! dscl . -read "/Groups/$MACOS_GROUP" >/dev/null 2>&1; then
    account_id="$(next_macos_id)"
    if [ -z "$account_id" ]; then
      printf '%s\n' "No available macOS system id in range 380-499." >&2
      exit 1
    fi
    run_cmd dscl . -create "/Groups/$MACOS_GROUP"
    run_cmd dscl . -create "/Groups/$MACOS_GROUP" PrimaryGroupID "$account_id"
    run_cmd dscl . -create "/Groups/$MACOS_GROUP" Password '*'
  fi
  if ! dscl . -read "/Users/$MACOS_USER" >/dev/null 2>&1; then
    if [ -z "$account_id" ]; then
      account_id="$(dscl . -read "/Groups/$MACOS_GROUP" PrimaryGroupID | awk '{ print $2 }')"
    fi
    run_cmd dscl . -create "/Users/$MACOS_USER"
    run_cmd dscl . -create "/Users/$MACOS_USER" UniqueID "$account_id"
    run_cmd dscl . -create "/Users/$MACOS_USER" PrimaryGroupID "$account_id"
    run_cmd dscl . -create "/Users/$MACOS_USER" NFSHomeDirectory /usr/local/var/lib/orion
    run_cmd dscl . -create "/Users/$MACOS_USER" UserShell /usr/bin/false
    run_cmd dscl . -create "/Users/$MACOS_USER" RealName "Orion Agent"
    run_cmd dscl . -create "/Users/$MACOS_USER" Password '*'
  fi
}

install_macos() {
  binary="$1"
  config_dir="/usr/local/etc/orion"
  state_dir="/usr/local/var/lib/orion"
  log_dir="/usr/local/var/log"
  plist_path="/Library/LaunchDaemons/com.orion.agent.plist"

  ensure_macos_account
  run_cmd install -d -m 0750 -o "$MACOS_USER" -g "$MACOS_GROUP" "$config_dir"
  run_cmd install -d -m 0750 -o "$MACOS_USER" -g "$MACOS_GROUP" "$state_dir"
  run_cmd install -d -m 0755 "$log_dir"
  run_cmd install -m 0755 "$binary" "$INSTALL_DIR/orion-agent"
  install_config "$config_dir/config.yaml" "$MACOS_USER" "$MACOS_GROUP"
  run_cmd install -m 0644 "deploy/launchd/com.orion.agent.plist" "$plist_path"

  if [ "$DRY_RUN" = "true" ]; then
    run_cmd launchctl bootout system "$plist_path"
  else
    launchctl bootout system "$plist_path" >/dev/null 2>&1 || true
  fi
  if [ "$START_SERVICE" = "true" ]; then
    run_cmd launchctl bootstrap system "$plist_path"
  fi
}

OS="$(detect_os)"
BINARY="$(resolve_binary)"

if [ ! -x "$BINARY" ]; then
  printf 'Agent binary is not executable: %s\n' "$BINARY" >&2
  exit 1
fi

case "$OS" in
  linux) install_linux "$BINARY" ;;
  macos) install_macos "$BINARY" ;;
esac

if [ "$DRY_RUN" = "true" ]; then
  printf '%s\n' "Orion Agent install dry run complete."
elif [ "$START_SERVICE" = "true" ]; then
  printf '%s\n' "Orion Agent installed."
  printf '%s\n' "Service started: $SERVICE_NAME"
else
  printf '%s\n' "Orion Agent installed."
  printf '%s\n' "Service installed but not started."
fi
