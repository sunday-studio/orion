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
cd apps/agent
go build -o orion-agent .
cd ../..
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

## Tailscale And Local Networks

For a home server deployment, prefer a stable local address for `core_url`:

- Tailscale MagicDNS: `http://orion-core.tailnet-name.ts.net:8999`;
- Tailscale IP: `http://100.x.y.z:8999`;
- local DNS: `http://orion-core.local:8999`;
- LAN IP: `http://192.168.x.y:8999`.

Use HTTPS if Core is exposed outside a trusted local network. Keep Agent traffic on Tailscale or a private LAN when possible.
