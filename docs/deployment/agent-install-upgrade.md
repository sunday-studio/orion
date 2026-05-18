# Agent Install And Upgrade

Install the Agent on every Linux or macOS machine you want Orion to monitor. Core should already
be running from the Docker image before installing Agents.

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

## Install With Minimal Config

Use this when you only want the Agent to register and report basic host metrics first:

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- \
  --core-url http://orion-core.local:8999 \
  --version v0.1.0
```

Replace `http://orion-core.local:8999` with a Core URL the Agent host can reach.

Common examples:

- `http://orion-core.local:8999`;
- `http://192.168.x.y:8999`;
- `http://100.x.y.z:8999` on Tailscale;
- `https://orion.example.com` behind a reverse proxy.

## Install With Sample Config

Download the sample config:

```sh
curl -fsSL -o orion-agent-config.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/examples/home-server-config.yaml
```

Edit:

- `core_url`;
- `meta.title`;
- monitor names and thresholds;
- any host-specific paths, ports, services, or Docker container names.

Then install:

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- \
  --config ./orion-agent-config.yaml \
  --version v0.1.0
```

Use `--overwrite-config` when replacing an already installed config. Use `--no-start` to install
files without starting the service.

## What The Installer Does

The bootstrap script:

- detects Linux or macOS and CPU architecture;
- downloads the matching Agent release binary;
- downloads the platform service files;
- installs the Agent binary, config, state directory, and service;
- starts the Agent service unless `--no-start` is passed.

The release binary is downloaded from:

```txt
https://github.com/sunday-studio/orion/releases/download/<version>/orion-agent-<os>-<arch>
```

## Post-Install Verification

After the service starts:

- the service should be active with the Linux or macOS service command below;
- `state.db` should exist in the platform state path;
- the Agent should appear once in the Console Agents view;
- configured monitors should appear after their first interval;
- restarting the Agent should reuse the same Agent and monitor records.

If the service starts but nothing appears in Core, check that `core_url` is reachable from the
monitored host and that the host clock is correct.

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

Docker container monitors call the local `docker` CLI. When the Agent is installed as a systemd
service, it runs as the `orion` user and cannot read `/var/run/docker.sock` unless that user has
permission.

If you use Docker monitors, add the service user to the Docker group and restart the Agent:

```sh
sudo usermod -aG docker orion
sudo systemctl restart orion-agent
```

If your host uses a custom Docker socket path or rootless Docker, configure the environment and
permissions so the `orion` user can run `docker inspect <container>` successfully. Without this,
Docker monitors will report failures even when the containers are healthy.

## Upgrade

Run the bootstrap installer again with the new release version:

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- \
  --config ./orion-agent-config.yaml \
  --version v0.1.1
```

The Agent identity and monitor mapping live in `state.db`, so replacing the binary does not
re-register the server unless that state file is removed.

After an upgrade, confirm:

- the service is active;
- the Agent still appears as the same Agent in Console;
- monitors were not duplicated;
- new reports arrive after the configured Agent and monitor intervals.

Do not remove `state.db` during a normal upgrade. Removing it intentionally makes the Agent
register as a fresh local identity, although Core can reconcile duplicate monitor names during
registration.

## Rollback

Run the bootstrap installer with the previous release version:

```sh
curl -fsSL https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/scripts/agent-bootstrap.sh | sudo bash -s -- \
  --config ./orion-agent-config.yaml \
  --version v0.1.0
```

Then verify the service is active and reports are arriving.

## Uninstall

The uninstall helper is intended for local checkouts or downloaded release bundles:

```sh
sudo ./deploy/scripts/agent-uninstall.sh
```

It stops the service and removes the binary. It asks before removing config, state, and
user/group records.

## Tailscale And Local Networks

For a home server deployment, prefer a stable local address for `core_url`:

- Tailscale MagicDNS: `http://orion-core.tailnet-name.ts.net:8999`;
- Tailscale IP: `http://100.x.y.z:8999`;
- local DNS: `http://orion-core.local:8999`;
- LAN IP: `http://192.168.x.y:8999`.

Use HTTPS if Core is exposed outside a trusted local network. Keep Agent traffic on Tailscale or a
private LAN when possible.
