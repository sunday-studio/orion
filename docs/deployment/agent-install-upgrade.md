# Agent Install And Upgrade

Install the Agent on every Linux or macOS machine you want Orion to monitor. Core should already
be running from the Docker image before installing Agents.

## Paths

Linux:

- binary: `/usr/local/bin/orion-agent`;
- config: `/etc/orion/config.yaml`;
- state: `/var/lib/orion/state.db`;
- log: `/var/log/orion/agent.log`;
- service: `/etc/systemd/system/orion-agent.service`.

macOS:

- binary: `/usr/local/bin/orion-agent`;
- config: `/usr/local/etc/orion/config.yaml`;
- state: `/usr/local/var/lib/orion/state.db`;
- log: `/usr/local/var/log/orion/agent.log`;
- service: `/Library/LaunchDaemons/com.orion.agent.plist`.

## Install With Minimal Config

Use this when you only want the Agent to register and report basic host metrics first:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld
```

The installer uses `sudo` only when it needs to install the service. Use a Core URL the Agent host
can reach.

Common examples:

- `http://orion-core.local:8999`;
- `http://192.168.x.y:8999`;
- `http://100.x.y.z:8999` on Tailscale;
- `https://orion.example.com` behind a reverse proxy.

## Install With Sample Config

Download the sample config:

```sh
curl -fsSL -o orion-agent-config.yaml \
  https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-config.yaml
```

Edit:

- `core_url`;
- `meta.title`;
- monitor names and thresholds;
- any host-specific paths, ports, services, or Docker container names.

Then install:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --config ./orion-agent-config.yaml
```

Use `--overwrite-config` when replacing an already installed config. Use `--no-start` to install
files without starting the service.

## What The Installer Does

The bootstrap script:

- detects Linux or macOS and CPU architecture;
- downloads the matching Agent release binary;
- downloads the platform service files;
- installs the Agent binary, config, initialized state database, structured log file, and service;
- starts the Agent service unless `--no-start` is passed.

Existing config and state files are kept during normal installs so the Agent keeps the same local
identity and monitor mappings. Pass `--overwrite-config` only when you intentionally want to replace
the installed config.

Re-running the installer on a host that already has Orion installed is treated as a repair or
upgrade install. If only config or state paths remain from a previous uninstall, the installer
treats the run as a reinstall. Both paths refresh the binary and service files, preserve config and
`state.db` when present, normalize log file permissions, and start or restart the service unless
`--no-start` is passed. A host that was installed and then stopped will come back with the same
Agent identity when the installer starts it again.

By default, the release binary is downloaded from the latest GitHub release:

```txt
https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-<os>-<arch>
```

The Agent binary reports its own baked version to Core. Pass `--version` when you want to pin the
initial install to a specific release:

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

## Token Recovery

When Core rotates or reissues an Agent token, apply the replacement token to the existing state
database instead of re-registering:

```sh
orion-agent token apply --token-file /secure/path/replacement-token
orion-agent restart
```

`token apply` preserves `agent_id`, `core_url`, maintenance state, monitor mappings, and queued
reports. Use it when an administrator gives you a replacement token for the same Agent identity.
Prefer `--token-file` so the replacement token does not land in shell history. The command does
not print the replacement token.

If Core rejects the stored token, the Agent treats the authentication failure as terminal, stops
reporting, exits with a non-zero status, and leaves local state intact for diagnostics.

Use `orion-agent reconfigure` only when you intentionally need a new local registration, such as
moving the Agent to a different Core URL or fresh Core database. Reconfigure clears local
registration, monitor mappings, and queued reports so the Agent can register again.

## Service Commands

Linux:

```sh
orion-agent status
orion-agent doctor
orion-agent restart
orion-agent logs
orion-agent logs --lines 200
orion-agent logs --level error
orion-agent logs --since 1h
```

macOS:

```sh
orion-agent status
orion-agent doctor
orion-agent restart
orion-agent logs
orion-agent logs --lines 200
orion-agent logs --level error
orion-agent logs --since 1h
```

Installed Agent commands prompt for privileges when the operating system requires service,
state database, or binary access. Read-only commands such as `status`, `logs`, and
`config validate` avoid privilege prompts where possible. You do not need to prefix
`orion-agent` commands with `sudo`.

Use `orion-agent doctor` when the service does not start cleanly or the Agent does not appear in
Core. It checks service installation, config validity, state readability, log directory presence,
Core reachability, and Docker socket presence. `status`, `doctor`, and `config show` support
`--json` for automation.

The service writes structured JSON Lines logs to the platform log path above. The
`orion-agent logs` command pretty-prints those entries and falls back to systemd or launchd
diagnostics when the structured log file is not available yet.

## Docker Monitors On Linux

Docker container monitors call the local `docker` CLI. When the Agent is installed as a systemd
service, it runs as the `orion` user and needs permission to read `/var/run/docker.sock`.

On Linux, the installer adds `orion` to the `docker` group when that group exists before the Agent
service starts. To verify Docker monitor access:

```sh
sudo -u orion docker inspect <container>
```

If your host uses a custom Docker socket path or rootless Docker, configure the environment and
permissions so the `orion` user can run `docker inspect <container>` successfully. Without this,
Docker monitors will report failures even when the containers are healthy.

## Update

Use the installed Agent to update itself:

```sh
orion-agent update
```

Pin a target release when needed:

```sh
orion-agent update -version 0.1.2
```

The update command:

- downloads the matching release binary for the host OS and CPU architecture;
- backs up the current binary next to `/usr/local/bin/orion-agent`;
- replaces only the binary;
- keeps the installed config and `state.db`;
- resets service failure throttles;
- starts the Agent service;
- prints service status and recent service logs.

On Linux this includes the equivalent of:

```sh
sudo systemctl reset-failed orion-agent
sudo systemctl start orion-agent
sudo systemctl status orion-agent --no-pager
sudo journalctl -u orion-agent -n 80 --no-pager
```

The Agent identity and monitor mapping live in `state.db`, so updating the binary does not
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

Use the update command with the previous release version:

```sh
orion-agent update -version 0.1.1
```

The command restarts the service and prints status/logs after the rollback.

If the installed binary is missing or cannot run, use the bootstrap installer again as a repair
install. It keeps existing config and state by default:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld
```

## Uninstall

Uninstall from the published release:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-uninstall.sh | sudo bash
```

Or run the helper from a local checkout:

```sh
sudo ./deploy/scripts/agent-uninstall.sh
```

It stops the service and removes the binary and service files. Config, state, and the service
account are kept by default unless you approve the prompts or pass explicit flags:

```sh
sudo ./deploy/scripts/agent-uninstall.sh --keep-config --keep-state --keep-user
sudo ./deploy/scripts/agent-uninstall.sh --purge
```

Use the keep flags for a clean reinstall that preserves the Agent identity. Use `--purge` only when
you intentionally want to remove config, state, and the unused service account.

## Tailscale And Local Networks

For a home server deployment, prefer a stable local address for `core_url`:

- Tailscale MagicDNS: `http://orion-core.tailnet-name.ts.net:8999`;
- Tailscale IP: `http://100.x.y.z:8999`;
- local DNS: `http://orion-core.local:8999`;
- LAN IP: `http://192.168.x.y:8999`.

Use HTTPS if Core is exposed outside a trusted local network. Keep Agent traffic on Tailscale or a
private LAN when possible.
