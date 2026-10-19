# Server CLI cheat sheet

## Install and configuration

The installer creates an editable config and starts the service. Pin a release for reproducible
installs:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --version 0.1.2 \
  --core-url https://core.your-domain.tld
```

| | Linux | macOS |
|---|---|---|
| Config | `/etc/orion/config.yaml` | `/usr/local/etc/orion/config.yaml` |
| State | `/var/lib/orion/state.db` | `/usr/local/var/lib/orion/state.db` |
| Log | `/var/log/orion/agent.log` | `/usr/local/var/log/orion/agent.log` |

The default config has the Core URL, a 60 s reporting interval, location collection off and no
monitors. Add checks to it when you are ready. Do not delete `state.db` during an upgrade: it holds the
Server identity, token, maintenance state and monitor mapping.

Service logs, rollback, Docker monitor permissions and local-network notes are in
[Server install and upgrade](agent-install-upgrade.md).

## Day-to-day commands

| Command | What it does |
|---|---|
| `orion-agent status` | Show whether the service is running and connected to Core. |
| `orion-agent doctor` | Check config, connectivity and the service, and suggest fixes. |
| `orion-agent logs` | Show recent entries. Add `--level error` or `--since 1h`. |
| `orion-agent restart` | Restart the service after editing the config. |
| `orion-agent update` | Update the binary, keeping config and state. `-version 0.1.2` pins a version. |
| `orion-agent reconfigure --token-file <file>` | Reconnect a known host with a fresh Add server token when its local credential was lost. |

Add `--json` to `status`, `doctor` and `config show` for automation. To try a single collection cycle,
run `orion-agent run -once` (add `-verbose` when debugging registration or transport). Editing monitors
in the config does not need a reinstall: restart the service and it reconciles monitors by name.
