# Node CLI cheat sheet

The `orion-node` command manages the Node on a machine. Install, upgrade and uninstall are explained
step by step in [Install and manage a Node](node-install-upgrade.md); this page is the quick reference.
Every command takes `--help`.

## Where things live

| | Linux | macOS |
|---|---|---|
| Config | `/etc/orion/config.yaml` | `/usr/local/etc/orion/config.yaml` |
| State | `/var/lib/orion/state.db` | `/usr/local/var/lib/orion/state.db` |
| Log | `/var/log/orion/node.log` | `/usr/local/var/log/orion/node.log` |

A fresh install has the Core URL, a 60 s reporting interval and no monitors. Do not delete `state.db`
during an upgrade: it holds the server's identity, token, maintenance state and monitor mapping.

## See what is happening

| Command | What it does |
|---|---|
| `orion-node status` | Whether the Node is running and connected to Core. `--short` for one line, `--watch` to redraw live. |
| `orion-node doctor` | Checks config, Core, the token, the service and the log, and prints the fix for anything wrong. |
| `orion-node logs` | Recent Node log lines. `--lines 200`, `--level error`, `--since 1h`, `--follow` (full screen). |
| `orion-node usage` | What apps cost on this machine. `--watch` to redraw live, `--top 10` for more rows. |
| `orion-node discover` | What the Node found that you could monitor. `--rescan` looks again, `--dismissed` lists what you hid. |
| `orion-node config show` | The settings in effect and which file each came from. |
| `orion-node version` | The installed version. |

## Change things

| Command | What it does |
|---|---|
| `orion-node setup` | Choose what to monitor in a guided form. `--accept-discovered --yes` accepts every high-confidence suggestion without asking; `--dry-run` shows the change and stops. |
| `orion-node discover dismiss <id>` / `restore <id>` / `ignore <id>` | Hide a suggestion, bring it back, or never suggest it again. |
| `orion-node config edit` / `validate` / `diff` | Edit the config in your editor with validation, check it, or compare it with another file. |
| `orion-node restart` | Restart the service after editing the config. Editing monitors does not need a reinstall: the Node reconciles them by name. |
| `orion-node maintenance down "reason" --for 2h` | Mute alerts for planned work (up to 720 h). `orion-node maintenance up` ends it early. |
| `orion-node update` | Update the binary, keeping config and state. `--version v0.2.0` pins a release; `--rollback` goes back. |
| `orion-node token apply --token-file <file>` | Apply a replacement token Core issued for this server. |
| `orion-node reconfigure --token-file <file>` | Reconnect this host when its local credential was lost, with a fresh Add server token. |

## Scripts and cron

| Command | What it does |
|---|---|
| `orion-node emit --severity error --event backup.failed "backup failed"` | Send one log entry to Core. Needs an ingest key from the Console (`--key-file`, or `ORION_INGEST_KEY`). |
| `orion-node run-job --name nightly -- ./backup.sh` | Run a command, ship its output and exit code, and ping a heartbeat monitor. The command's own exit code is passed through. |
| `orion-node run --once` | Run a single collection cycle in the foreground, for debugging registration or transport (add `--verbose`). |

## Output, exit codes and interfaces

Add `--json` to `status`, `doctor`, `discover` and `config show` for scripts; the output carries a
`schema_version`. `NO_COLOR=1` or `TERM=dumb` gives the plain layout: no colour, boxes or glyphs, one
fact per line.

| Code | Meaning |
|---|---|
| 0 | OK |
| 1 | Failed checks or errors |
| 2 | Usage error: unknown command or flag |
| 3 | Needs elevation: run with `sudo` |

An interface appears only when a person is on a terminal and the command needs input or takes time.
Everything else is plain text, always when piped or with `--json`.

| Kind | Commands |
|---|---|
| Forms | `setup`, `enroll`, `token apply`, `maintenance down` (with no arguments), `stop`, `config edit` |
| Live, in place | `doctor`, `update`, `status --watch`, `usage --watch` |
| Full screen | `logs --follow` |
| Plain | everything else |

Set `ACCESSIBLE=1` (or use `TERM=dumb`) to get line prompts instead of forms in `setup` and `enroll`.
Installed Node commands ask for privileges themselves when the system needs them, so you do not prefix
them with `sudo`.
