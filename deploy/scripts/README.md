# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Agent Scripts

- **agent-install.sh** - Installs the Orion Agent binary, config, state directory, and service.
- **agent-uninstall.sh** - Uninstalls the Orion agent, removes service files, and cleans up configuration.

### Installation Scripts

See [Agent install and upgrade](../../docs/deployment/agent-install-upgrade.md).

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Make scripts executable
chmod +x deploy/scripts/*.sh

# Run uninstall script
sudo ./deploy/scripts/agent-uninstall.sh
```
