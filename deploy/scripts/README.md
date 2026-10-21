# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Server Scripts

- **node-bootstrap.sh** - Curlable installer that downloads a released Server binary and installs it as a service.
- **node-install.sh** - Installs the Orion Server binary, config, state directory, and service.
- **node-uninstall.sh** - Uninstalls the Orion server, removes service files, and cleans up configuration.
- **node-cli-lifecycle-smoke.sh** - Dry-run smoke test for install, uninstall, and lifecycle CLI output.

### Installation Scripts

See [Server install and upgrade](../../docs/deployment/node-install-upgrade.md).

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Install from a published release
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld

# Uninstall from a published release
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-uninstall.sh | sudo bash

# Run uninstall script from a local checkout and keep reinstall data
sudo ./deploy/scripts/node-uninstall.sh --keep-config --keep-state --keep-user

# Fully remove config, state, and unused service account too
sudo ./deploy/scripts/node-uninstall.sh --purge

# Run non-mutating lifecycle script checks
deploy/scripts/node-cli-lifecycle-smoke.sh
```
