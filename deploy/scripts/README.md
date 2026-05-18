# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Agent Scripts

- **agent-bootstrap.sh** - Curlable installer that downloads a released Agent binary and installs it as a service.
- **agent-install.sh** - Installs the Orion Agent binary, config, state directory, and service.
- **agent-uninstall.sh** - Uninstalls the Orion agent, removes service files, and cleans up configuration.

### Installation Scripts

See [Agent install and upgrade](../../docs/deployment/agent-install-upgrade.md).

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Install from a published release
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- \
  --core-url http://orion-core.local:8999

# Run uninstall script
sudo ./deploy/scripts/agent-uninstall.sh
```
