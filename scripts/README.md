# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Agent Scripts

- **agent-uninstall.sh** - Uninstalls the Orion agent, removes service files, and cleans up configuration

### Installation Scripts

- **agent-install.sh** - Installs the Orion agent (to be created)
  - Detects OS (Linux/macOS)
  - Downloads correct binary
  - Creates config & state directories
  - Installs and enables systemd/launchd service

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Make scripts executable
chmod +x scripts/*.sh

# Run uninstall script
sudo ./scripts/agent-uninstall.sh
```
