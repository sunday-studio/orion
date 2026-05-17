#!/bin/bash

# Orion Agent Uninstall Script
# Removes the agent installation and optionally cleans up config/state files.

set -euo pipefail

LINUX_USER="orion"
LINUX_GROUP="orion"
MACOS_USER="_orion"
MACOS_GROUP="_orion"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="orion-agent"
DRY_RUN="false"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: sudo ./deploy/scripts/agent-uninstall.sh [options]"
    echo ""
    echo "Options:"
    echo "  --dry-run    Print uninstall actions without changing the system."
    echo "  -h, --help   Show this help."
}

run_cmd() {
    if [[ "$DRY_RUN" == "true" ]]; then
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

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}" >&2
            usage
            exit 1
            ;;
    esac
done

echo -e "${YELLOW}Orion Agent Uninstaller${NC}"
echo "================================"

# Detect OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    ORION_USER="$LINUX_USER"
    ORION_GROUP="$LINUX_GROUP"
    CONFIG_DIR="/etc/orion"
    STATE_DIR="/var/lib/orion"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
    ORION_USER="$MACOS_USER"
    ORION_GROUP="$MACOS_GROUP"
    CONFIG_DIR="/usr/local/etc/orion"
    STATE_DIR="/usr/local/var/lib/orion"
else
    echo -e "${RED}Unsupported OS: $OSTYPE${NC}"
    exit 1
fi

# Check if running as root
if [[ "$DRY_RUN" != "true" && $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Stop and disable service
if [[ "$OS" == "linux" ]]; then
    if systemctl is-active --quiet $SERVICE_NAME 2>/dev/null; then
        echo -e "${YELLOW}Stopping $SERVICE_NAME service...${NC}"
        run_cmd systemctl stop $SERVICE_NAME || true
    fi
    
    if systemctl is-enabled --quiet $SERVICE_NAME 2>/dev/null; then
        echo -e "${YELLOW}Disabling $SERVICE_NAME service...${NC}"
        run_cmd systemctl disable $SERVICE_NAME || true
    fi
    
    # Remove systemd service file
    if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
        echo -e "${YELLOW}Removing systemd service file...${NC}"
        run_cmd rm -f "/etc/systemd/system/$SERVICE_NAME.service"
        run_cmd systemctl daemon-reload
    fi
elif [[ "$OS" == "macos" ]]; then
    # Stop launchd service
    if launchctl print system/com.orion.agent >/dev/null 2>&1; then
        echo -e "${YELLOW}Stopping launchd service...${NC}"
        run_cmd launchctl bootout system /Library/LaunchDaemons/com.orion.agent.plist || true
    fi
    
    # Remove launchd plist files
    if [ -f /Library/LaunchDaemons/com.orion.agent.plist ]; then
        run_cmd rm -f /Library/LaunchDaemons/com.orion.agent.plist
    fi
fi

# Remove binary
if [ -f "$INSTALL_DIR/orion-agent" ]; then
    echo -e "${YELLOW}Removing binary...${NC}"
    run_cmd rm -f "$INSTALL_DIR/orion-agent"
fi

# Remove config directory
if [ -d "$CONFIG_DIR" ]; then
    echo -e "${YELLOW}Removing config directory...${NC}"
    if [[ "$DRY_RUN" == "true" ]]; then
        run_cmd rm -rf "$CONFIG_DIR"
    else
        read -p "Remove config directory $CONFIG_DIR? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            run_cmd rm -rf "$CONFIG_DIR"
        else
            echo -e "${YELLOW}Keeping config directory $CONFIG_DIR${NC}"
        fi
    fi
fi

# Remove state directory
if [ -d "$STATE_DIR" ]; then
    echo -e "${YELLOW}Removing state directory...${NC}"
    if [[ "$DRY_RUN" == "true" ]]; then
        run_cmd rm -rf "$STATE_DIR"
    else
        read -p "Remove state directory $STATE_DIR? This removes the local agent identity. (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            run_cmd rm -rf "$STATE_DIR"
        else
            echo -e "${YELLOW}Keeping state directory $STATE_DIR${NC}"
        fi
    fi
fi

# Remove user and group (if they exist and are not used by other services)
if id "$ORION_USER" &>/dev/null; then
    echo -e "${YELLOW}Checking if $ORION_USER user can be removed...${NC}"
    # Only remove if no other processes are using this user
    if ! pgrep -u "$ORION_USER" > /dev/null 2>&1; then
        if [[ "$DRY_RUN" == "true" ]]; then
            if [[ "$OS" == "linux" ]]; then
                run_cmd userdel "$ORION_USER"
                run_cmd groupdel "$ORION_GROUP"
            else
                run_cmd dscl . -delete "/Users/$ORION_USER"
                run_cmd dscl . -delete "/Groups/$ORION_GROUP"
            fi
        else
            read -p "Remove $ORION_USER user and group? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                if [[ "$OS" == "linux" ]]; then
                    run_cmd userdel "$ORION_USER" 2>/dev/null || true
                    run_cmd groupdel "$ORION_GROUP" 2>/dev/null || true
                else
                    run_cmd dscl . -delete "/Users/$ORION_USER" 2>/dev/null || true
                    run_cmd dscl . -delete "/Groups/$ORION_GROUP" 2>/dev/null || true
                fi
            fi
        fi
    else
        echo -e "${YELLOW}User $ORION_USER is still in use, skipping removal${NC}"
    fi
fi

if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "${GREEN}Uninstall dry run complete!${NC}"
else
    echo -e "${GREEN}Uninstall complete!${NC}"
fi
echo ""
if [[ "$DRY_RUN" == "true" ]]; then
    echo "The following would be removed when present:"
else
    echo "The following were removed:"
fi
echo "  - Binary: $INSTALL_DIR/orion-agent"
echo "  - Service files"
if [[ "$OS" == "linux" ]]; then
    echo "  - systemd service"
elif [[ "$OS" == "macos" ]]; then
    echo "  - launchd service"
fi
echo ""
if [[ "$DRY_RUN" == "true" ]]; then
    echo "Dry run did not remove config or state."
else
    echo "The following may still exist (you chose to keep them):"
    echo "  - Config: $CONFIG_DIR (if you chose to keep it)"
    echo "  - State: $STATE_DIR (if you chose to keep it)"
fi
