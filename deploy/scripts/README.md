# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Agent Scripts

- **agent-bootstrap.sh** - Curlable installer that downloads a released Agent binary and installs it as a service.
- **agent-install.sh** - Installs the Orion Agent binary, config, state directory, and service.
- **agent-uninstall.sh** - Uninstalls the Orion agent, removes service files, and cleans up configuration.
- **agent-cli-lifecycle-smoke.sh** - Dry-run smoke test for install, uninstall, and lifecycle CLI output.

### Installation Scripts

See [Agent install and upgrade](../../docs/deployment/agent-install-upgrade.md).

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Install from a published release
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld

# Uninstall from a published release
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-uninstall.sh | sudo bash

# Run uninstall script from a local checkout and keep reinstall data
sudo ./deploy/scripts/agent-uninstall.sh --keep-config --keep-state --keep-user

# Fully remove config, state, and unused service account too
sudo ./deploy/scripts/agent-uninstall.sh --purge

# Run non-mutating lifecycle script checks
deploy/scripts/agent-cli-lifecycle-smoke.sh
```
