# Orion Scripts

This directory contains installation, deployment, and utility scripts for the Orion project.

## Scripts

### Agent Scripts

- **agent-uninstall.sh** - Uninstalls the Orion agent, removes service files, and cleans up configuration

### Installation Scripts

- **agent-install.sh** — Planned. Will detect OS (Linux/macOS), download binary, create config and state dirs, and enable systemd/launchd.

### Manual install

Until `agent-install.sh` exists, install the agent by hand:

1. **Build** the agent: `cd apps/agent && go build -o orion-agent .`
2. **Copy** the binary to `/usr/local/bin/orion-agent`.
3. **Create** `/etc/orion/config.yaml` (Linux) or `/usr/local/etc/orion/config.yaml` (macOS) with `core_url`, `interval`, and optional `monitors`. Create `/var/lib/orion` (Linux) or `/usr/local/var/lib/orion` (macOS) for state.
4. **Install the service**:
   - **Linux (systemd)**: Copy `orion-agent.service` to `/etc/systemd/system/`, create `orion` user/group, then `systemctl enable --now orion-agent`.
   - **macOS (launchd)**: Copy `com.orion.agent.plist` to `/Library/LaunchDaemons/`, create `_orion` user/group, then `launchctl load /Library/LaunchDaemons/com.orion.agent.plist`.

Paths in the service files may need to match your config and state locations.

## Usage

All scripts should be run with appropriate permissions (often requires root/sudo for system-level operations).

```bash
# Make scripts executable
chmod +x deploy/scripts/*.sh

# Run uninstall script
sudo ./deploy/scripts/agent-uninstall.sh
```
