# Agent Install And Upgrade

This guide covers the self-hosted Agent deployment path for Linux systemd and macOS launchd.

## Paths

Linux:

- binary: `/usr/local/bin/orion-agent`;
- config: `/etc/orion/config.yaml`;
- state: `/var/lib/orion/state.db`;
- service: `/etc/systemd/system/orion-agent.service`.

macOS:

- binary: `/usr/local/bin/orion-agent`;
- config: `/usr/local/etc/orion/config.yaml`;
- state: `/usr/local/var/lib/orion/state.db`;
- service: `/Library/LaunchDaemons/com.orion.agent.plist`.

## Install

Build the Agent from this checkout:

```sh
VERSION=v0.1.0 make agent-build
```

Install with a generated minimal config:

```sh
sudo ./deploy/scripts/agent-install.sh \
  --core-url http://orion-core.local:8999 \
  --binary ./apps/agent/orion-agent
```

Install with an existing config:

```sh
sudo ./deploy/scripts/agent-install.sh \
  --config ./deploy/examples/home-server-config.yaml \
  --binary ./apps/agent/orion-agent
```

Use `--no-start` to install files without starting the service. Use `--overwrite-config` to replace an installed config.

## Post-Install Verification

After the service starts, verify the install before leaving the host:

- The service should be active with the Linux or macOS service command below.
- `state.db` should exist in the platform state path. This file stores the local Agent identity, token, maintenance flag, and monitor ID mapping.
- The Agent should appear once in the Console Agents view.
- Configured monitors should appear after their first interval.
- Restarting the Agent should reuse the same Agent and monitor records.

If the service starts but nothing appears in Core, check that `core_url` is reachable from the monitored host and that the host clock is correct.

## Service Commands

Linux:

```sh
sudo systemctl status orion-agent
sudo systemctl restart orion-agent
sudo journalctl -u orion-agent -f
```

macOS:

```sh
sudo launchctl print system/com.orion.agent
sudo launchctl kickstart -k system/com.orion.agent
tail -f /usr/local/var/log/orion-agent.log
```

## Docker Monitors On Linux

Docker container monitors call the local `docker` CLI. When the Agent is installed as a systemd service, it runs as the `orion` user and cannot read `/var/run/docker.sock` unless that user has permission.

If you use Docker monitors, add the service user to the Docker group and restart the Agent:

```sh
sudo usermod -aG docker orion
sudo systemctl restart orion-agent
```

If your host uses a custom Docker socket path or rootless Docker, configure the environment and permissions so the `orion` user can run `docker inspect <container>` successfully. Without this, Docker monitors will report failures even when the containers are healthy.

## Upgrade

Build or download the new Agent binary, then replace the installed binary and restart the service.

Linux:

```sh
sudo install -m 0755 ./apps/agent/orion-agent /usr/local/bin/orion-agent
sudo systemctl restart orion-agent
sudo systemctl status orion-agent
```

macOS:

```sh
sudo install -m 0755 ./apps/agent/orion-agent /usr/local/bin/orion-agent
sudo launchctl kickstart -k system/com.orion.agent
sudo launchctl print system/com.orion.agent
```

The Agent identity and monitor mapping live in `state.db`, so replacing the binary does not re-register the server unless that state file is removed.

After an upgrade, confirm:

- the service is active;
- the Agent still appears as the same Agent in Console;
- monitors were not duplicated;
- new reports arrive after the configured Agent and monitor intervals.

Do not remove `state.db` during a normal upgrade. Removing it intentionally makes the Agent register as a fresh local identity, although Core can reconcile duplicate monitor names during registration.

## Rollback

Keep the previous binary before replacing it:

```sh
sudo cp /usr/local/bin/orion-agent /usr/local/bin/orion-agent.previous
```

Rollback by restoring it and restarting the service:

```sh
sudo install -m 0755 /usr/local/bin/orion-agent.previous /usr/local/bin/orion-agent
```

Then restart with the Linux or macOS service command above.

## Uninstall

```sh
sudo ./deploy/scripts/agent-uninstall.sh
```

The uninstall script stops the service and removes the binary. It asks before removing config and user/group records.
It also asks before removing state because state contains the local Agent identity, token, maintenance flag, and monitor ID mapping.

## Tailscale And Local Networks

For a home server deployment, prefer a stable local address for `core_url`:

- Tailscale MagicDNS: `http://orion-core.tailnet-name.ts.net:8999`;
- Tailscale IP: `http://100.x.y.z:8999`;
- local DNS: `http://orion-core.local:8999`;
- LAN IP: `http://192.168.x.y:8999`.

Use HTTPS if Core is exposed outside a trusted local network. Keep Agent traffic on Tailscale or a private LAN when possible.
