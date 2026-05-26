#!/usr/bin/env bash

set -euo pipefail

LINUX_USER="orion"
LINUX_GROUP="orion"
MACOS_USER="_orion"
MACOS_GROUP="_orion"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="orion-agent"

DRY_RUN="false"
REMOVE_CONFIG="prompt"
REMOVE_STATE="prompt"
REMOVE_USER="prompt"
UNINSTALL_STEP=0
COLOR_ENABLED="false"

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  COLOR_ENABLED="true"
fi

usage() {
  printf '%s\n' "Usage: sudo ./deploy/scripts/agent-uninstall.sh [options]"
  printf '%s\n' ""
  printf '%s\n' "Options:"
  printf '%s\n' "  --keep-config       Keep installed config without prompting."
  printf '%s\n' "  --remove-config     Remove installed config without prompting."
  printf '%s\n' "  --keep-state        Keep local state and Agent identity without prompting."
  printf '%s\n' "  --remove-state      Remove local state and Agent identity without prompting."
  printf '%s\n' "  --keep-user         Keep the service user and group without prompting."
  printf '%s\n' "  --remove-user       Remove the service user and group when unused."
  printf '%s\n' "  --purge             Remove config, state, and unused service user/group."
  printf '%s\n' "  --dry-run           Print uninstall actions without changing the system."
  printf '%s\n' "  -h, --help          Show this help."
}

supports_color() {
  [ "$COLOR_ENABLED" = "true" ]
}

ansi() {
  if supports_color; then
    printf '\033[%sm' "$1"
  fi
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
  UNINSTALL_STEP=$((UNINSTALL_STEP + 1))
  printf '\n[%02d] %s\n' "$UNINSTALL_STEP" "$1"
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

print_orion_banner() {
  local accent=""
  local reset=""
  local dim=""

  accent="$(ansi '36;1')"
  dim="$(ansi '2')"
  reset="$(ansi '0')"

  printf '%s' "$accent"
  printf '%s\n' "   ____       _             "
  printf '%s\n' "  / __ \\_____(_)___  ____   "
  printf '%s\n' " / / / / ___/ / __ \\/ __ \\  "
  printf '%s\n' "/ /_/ / /  / / /_/ / / / /  "
  printf '%s\n' "\\____/_/  /_/\\____/_/ /_/   "
  printf '%s' "$reset"
  printf '%sAgent uninstaller%s\n' "$dim" "$reset"
}

ask_remove() {
  local setting="$1"
  local prompt="$2"

  case "$setting" in
    remove) return 0 ;;
    keep) return 1 ;;
  esac

  if [ "$DRY_RUN" = "true" ] || [ ! -r /dev/tty ]; then
    return 1
  fi

  printf '%s' "$prompt" >/dev/tty
  IFS= read -r reply </dev/tty
  case "$reply" in
    y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

detect_os() {
  case "$(uname -s)" in
    Linux) printf '%s\n' "linux" ;;
    Darwin) printf '%s\n' "macos" ;;
    *)
      printf 'Unsupported OS: %s\n' "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --keep-config)
      REMOVE_CONFIG="keep"
      shift
      ;;
    --remove-config)
      REMOVE_CONFIG="remove"
      shift
      ;;
    --keep-state)
      REMOVE_STATE="keep"
      shift
      ;;
    --remove-state)
      REMOVE_STATE="remove"
      shift
      ;;
    --keep-user)
      REMOVE_USER="keep"
      shift
      ;;
    --remove-user)
      REMOVE_USER="remove"
      shift
      ;;
    --purge)
      REMOVE_CONFIG="remove"
      REMOVE_STATE="remove"
      REMOVE_USER="remove"
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
      usage >&2
      exit 1
      ;;
  esac
done

OS="$(detect_os)"
case "$OS" in
  linux)
    ORION_USER="$LINUX_USER"
    ORION_GROUP="$LINUX_GROUP"
    CONFIG_DIR="/etc/orion"
    STATE_DIR="/var/lib/orion"
    SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
    ;;
  macos)
    ORION_USER="$MACOS_USER"
    ORION_GROUP="$MACOS_GROUP"
    CONFIG_DIR="/usr/local/etc/orion"
    STATE_DIR="/usr/local/var/lib/orion"
    SERVICE_FILE="/Library/LaunchDaemons/com.orion.agent.plist"
    ;;
esac

if [ "$DRY_RUN" != "true" ] && [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' "This script must be run as root." >&2
  exit 1
fi

print_orion_banner
if [ "$DRY_RUN" = "true" ]; then
  info "mode: dry run"
fi
info "platform: $OS"

step "Service"
if [ "$OS" = "linux" ]; then
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    run_cmd systemctl stop "$SERVICE_NAME" || true
    ok "stop $SERVICE_NAME"
  else
    skip "$SERVICE_NAME is not active"
  fi
  if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
    run_cmd systemctl disable "$SERVICE_NAME" || true
    ok "disable $SERVICE_NAME"
  else
    skip "$SERVICE_NAME is not enabled"
  fi
  if [ -f "$SERVICE_FILE" ]; then
    run_cmd rm -f "$SERVICE_FILE"
    run_cmd systemctl daemon-reload
    ok "removed systemd service file"
  else
    skip "systemd service file not found"
  fi
else
  if launchctl print system/com.orion.agent >/dev/null 2>&1; then
    run_cmd launchctl bootout system "$SERVICE_FILE" || true
    ok "stopped launchd service"
  else
    skip "launchd service is not loaded"
  fi
  if [ -f "$SERVICE_FILE" ]; then
    run_cmd rm -f "$SERVICE_FILE"
    ok "removed launchd plist"
  else
    skip "launchd plist not found"
  fi
fi

step "Binary"
if [ -f "$INSTALL_DIR/orion-agent" ]; then
  run_cmd rm -f "$INSTALL_DIR/orion-agent"
  ok "removed $INSTALL_DIR/orion-agent"
else
  skip "binary not found"
fi

step "Config"
if [ -d "$CONFIG_DIR" ]; then
  if ask_remove "$REMOVE_CONFIG" "Remove config directory $CONFIG_DIR? (y/N): "; then
    run_cmd rm -rf "$CONFIG_DIR"
    ok "removed config directory"
  else
    skip "keeping config directory $CONFIG_DIR"
  fi
else
  skip "config directory not found"
fi

step "State"
if [ -d "$STATE_DIR" ]; then
  if ask_remove "$REMOVE_STATE" "Remove state directory $STATE_DIR? This removes the local Agent identity. (y/N): "; then
    run_cmd rm -rf "$STATE_DIR"
    ok "removed state directory"
  else
    skip "keeping state directory $STATE_DIR"
  fi
else
  skip "state directory not found"
fi

step "Service account"
if id "$ORION_USER" >/dev/null 2>&1; then
  if pgrep -u "$ORION_USER" >/dev/null 2>&1; then
    skip "user $ORION_USER is still in use"
  elif ask_remove "$REMOVE_USER" "Remove $ORION_USER user and $ORION_GROUP group? (y/N): "; then
    if [ "$OS" = "linux" ]; then
      run_cmd userdel "$ORION_USER" 2>/dev/null || true
      run_cmd groupdel "$ORION_GROUP" 2>/dev/null || true
    else
      run_cmd dscl . -delete "/Users/$ORION_USER" 2>/dev/null || true
      run_cmd dscl . -delete "/Groups/$ORION_GROUP" 2>/dev/null || true
    fi
    ok "removed service user and group"
  else
    skip "keeping service user and group"
  fi
else
  skip "service user not found"
fi

if [ "$DRY_RUN" = "true" ]; then
  printf '\n%s\n' "Orion Agent uninstall dry run complete."
else
  printf '\n%s\n' "Orion Agent uninstalled."
fi

printf '\n%s\n' "After uninstall"
printf '  %s\n' "Binary and service files are removed when present."
if [ "$REMOVE_CONFIG" = "remove" ]; then
  printf '  %s\n' "Config removal requested: $CONFIG_DIR."
else
  printf '  %s\n' "Config is kept by default: $CONFIG_DIR."
fi
if [ "$REMOVE_STATE" = "remove" ]; then
  printf '  %s\n' "State removal requested: $STATE_DIR."
else
  printf '  %s\n' "State is kept by default: $STATE_DIR."
fi
printf '  %s\n' "Use --purge for a full wipe, or --keep-config --keep-state --keep-user for a quiet uninstall."
