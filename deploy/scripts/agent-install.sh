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
INSTALL_STEP=0
INSTALLED_CONFIG_PATH=""

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

step() {
  INSTALL_STEP=$((INSTALL_STEP + 1))
  printf '\n[%02d] %s\n' "$INSTALL_STEP" "$1"
}

ok() {
  if [ "$DRY_RUN" = "true" ]; then
    printf '     plan: %s\n' "$1"
    return
  fi
  printf '     ok: %s\n' "$1"
}

skip() {
  printf '     skip: %s\n' "$1"
}

info() {
  printf '     %s\n' "$1"
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
  local config_path="$1"
  if [ -z "$CORE_URL" ]; then
    printf '%s\n' "--core-url is required when --config is not provided." >&2
    exit 1
  fi
  {
    printf 'core_url: %s\n' "$CORE_URL"
    printf 'interval: 60s\n'
    printf 'geo_location: false\n'
    printf 'monitors: []\n'
  } > "$config_path"
}

config_has_core_url() {
  local config_path="$1"
  awk '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*core_url[[:space:]]*:[[:space:]]*[^[:space:]]/ { found = 1 }
    END { exit found ? 0 : 1 }
  ' "$config_path"
}

install_config() {
  local config_path="$1"
  local owner="$2"
  local group="$3"
  local tmp_config=""

  step "Config"
  if [ -f "$config_path" ] && [ "$OVERWRITE_CONFIG" != "true" ]; then
    if ! config_has_core_url "$config_path"; then
      printf '     error: existing config is missing core_url: %s\n' "$config_path" >&2
      printf '     rerun with --overwrite-config to regenerate it from --core-url, or edit the file manually.\n' >&2
      exit 1
    fi
    skip "keeping existing config at $config_path"
    return
  fi

  if [ -n "$CONFIG_SOURCE" ]; then
    if [ ! -f "$CONFIG_SOURCE" ]; then
      printf 'Config file does not exist: %s\n' "$CONFIG_SOURCE" >&2
      exit 1
    fi
    run_cmd install -m 0640 -o "$owner" -g "$group" "$CONFIG_SOURCE" "$config_path"
    ok "installed config from $CONFIG_SOURCE to $config_path"
  else
    if [ "$DRY_RUN" = "true" ]; then
      printf '+ generate minimal config at %q for core %q\n' "$config_path" "$CORE_URL"
      ok "would generate default config at $config_path"
      return
    fi
    tmp_config="$(mktemp)"
    write_minimal_config "$tmp_config"
    run_cmd install -m 0640 -o "$owner" -g "$group" "$tmp_config" "$config_path"
    rm -f "$tmp_config"
    ok "generated default config at $config_path"
  fi
}

initialize_state() {
  local state_path="$1"
  local owner="$2"
  local group="$3"

  step "State database"
  if [ "$DRY_RUN" = "true" ]; then
    printf '+ initialize state database at %q\n' "$state_path"
    ok "would initialize state database at $state_path"
    return
  fi

  "$INSTALL_DIR/orion-agent" state init -state "$state_path" >/dev/null
  run_cmd chown "$owner:$group" "$state_path"
  ok "initialized state database at $state_path"
}

install_linux() {
  binary="$1"
  config_dir="/etc/orion"
  INSTALLED_CONFIG_PATH="$config_dir/config.yaml"
  state_dir="/var/lib/orion"
  state_path="$state_dir/state.db"

  step "Service account"
  if ! getent group "$LINUX_GROUP" >/dev/null; then
    run_cmd groupadd --system "$LINUX_GROUP"
    ok "created group $LINUX_GROUP"
  else
    skip "group $LINUX_GROUP already exists"
  fi
  if ! id "$LINUX_USER" >/dev/null 2>&1; then
    run_cmd useradd --system --gid "$LINUX_GROUP" --home-dir "$state_dir" --shell /usr/sbin/nologin "$LINUX_USER"
    ok "created user $LINUX_USER"
  else
    skip "user $LINUX_USER already exists"
  fi

  step "Directories"
  run_cmd install -d -m 0750 -o "$LINUX_USER" -g "$LINUX_GROUP" "$config_dir"
  run_cmd install -d -m 0750 -o "$LINUX_USER" -g "$LINUX_GROUP" "$state_dir"
  ok "prepared $config_dir and $state_dir"

  step "Binary"
  run_cmd install -m 0755 "$binary" "$INSTALL_DIR/orion-agent"
  ok "install binary to $INSTALL_DIR/orion-agent"

  install_config "$INSTALLED_CONFIG_PATH" "$LINUX_USER" "$LINUX_GROUP"
  initialize_state "$state_path" "$LINUX_USER" "$LINUX_GROUP"

  step "Systemd service"
  run_cmd install -m 0644 "deploy/systemd/orion-agent.service" "/etc/systemd/system/$SERVICE_NAME.service"
  ok "install service file"

  run_cmd systemctl daemon-reload
  ok "reload systemd"
  run_cmd systemctl enable "$SERVICE_NAME"
  ok "enable $SERVICE_NAME"
  if [ "$START_SERVICE" = "true" ]; then
    run_cmd systemctl restart "$SERVICE_NAME"
    ok "start $SERVICE_NAME"

    step "Service state"
    if [ "$DRY_RUN" = "true" ]; then
      ok "would verify $SERVICE_NAME is active"
    elif systemctl is-active --quiet "$SERVICE_NAME"; then
      ok "$SERVICE_NAME is active"
    else
      printf '     error: %s did not become active\n' "$SERVICE_NAME" >&2
      exit 1
    fi
  else
    skip "service start disabled by --no-start"
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
    ok "would ensure user and group $MACOS_USER"
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
    ok "created group $MACOS_GROUP"
  else
    skip "group $MACOS_GROUP already exists"
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
    ok "created user $MACOS_USER"
  else
    skip "user $MACOS_USER already exists"
  fi
}

install_macos() {
  binary="$1"
  config_dir="/usr/local/etc/orion"
  INSTALLED_CONFIG_PATH="$config_dir/config.yaml"
  state_dir="/usr/local/var/lib/orion"
  state_path="$state_dir/state.db"
  log_dir="/usr/local/var/log"
  plist_path="/Library/LaunchDaemons/com.orion.agent.plist"

  step "Service account"
  ensure_macos_account

  step "Directories"
  run_cmd install -d -m 0750 -o "$MACOS_USER" -g "$MACOS_GROUP" "$config_dir"
  run_cmd install -d -m 0750 -o "$MACOS_USER" -g "$MACOS_GROUP" "$state_dir"
  run_cmd install -d -m 0755 "$log_dir"
  ok "prepared $config_dir, $state_dir, and $log_dir"

  step "Binary"
  run_cmd install -m 0755 "$binary" "$INSTALL_DIR/orion-agent"
  ok "install binary to $INSTALL_DIR/orion-agent"

  install_config "$INSTALLED_CONFIG_PATH" "$MACOS_USER" "$MACOS_GROUP"
  initialize_state "$state_path" "$MACOS_USER" "$MACOS_GROUP"

  step "Launchd service"
  run_cmd install -m 0644 "deploy/launchd/com.orion.agent.plist" "$plist_path"
  ok "install launchd plist"

  if [ "$DRY_RUN" = "true" ]; then
    run_cmd launchctl bootout system "$plist_path"
  else
    launchctl bootout system "$plist_path" >/dev/null 2>&1 || true
  fi
  if [ "$START_SERVICE" = "true" ]; then
    run_cmd launchctl bootstrap system "$plist_path"
    ok "start $SERVICE_NAME"

    step "Service state"
    if [ "$DRY_RUN" = "true" ]; then
      ok "would verify $SERVICE_NAME is loaded"
    elif launchctl print system/com.orion.agent >/dev/null 2>&1; then
      ok "$SERVICE_NAME is loaded"
    else
      printf '     error: %s did not load\n' "$SERVICE_NAME" >&2
      exit 1
    fi
  else
    skip "service start disabled by --no-start"
  fi
}

printf '%s\n' "Orion Agent installer"
if [ "$DRY_RUN" = "true" ]; then
  info "mode: dry run"
fi

OS="$(detect_os)"
info "platform: $OS"
BINARY="$(resolve_binary)"
info "binary: $BINARY"

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

if [ -n "$INSTALLED_CONFIG_PATH" ]; then
  printf '%s\n' "Edit the Agent config at: $INSTALLED_CONFIG_PATH"
  printf '%s\n' "This file is owned by the Agent service account; use sudo to update it."
fi
