# Install and manage a Node

A **Node** is the small program that runs on each machine you want Orion to watch. It reports the
machine's health to Core, runs the checks you choose, and sends its own logs. Install one on every
Linux or macOS machine you want to monitor. Core has to be running first; see
[Set up Core on a new server with Docker](docker-new-server.md).

What you need:

- a Linux machine with systemd, or a macOS machine, on amd64 or arm64 (Windows is not supported);
- `curl`, and `sudo` rights on the machine;
- a Core address the machine can reach (the Node only makes outbound connections; open no port on it).

> **Running the Node in a container is not supported yet.** It is deferred (decision D-031). A
> container image builds and runs, but it has only been tested on a desktop Docker, and it cannot see
> services, containers or pm2 apps. Install the Node on the host as described here. The state of the
> container image is written up in [Running the Node as a container](node-container.md).

## 1. Install

In the Console open **Servers**, choose **Add server**, pick Linux or macOS and copy the command it
shows. It has a one-time token in it and looks like this:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='orn_enr_...' bash -s -- --core-url 'https://orion.example.com'
```

On macOS the command has no `sudo`; the installer asks for your password when it needs it. Run the
command on the machine you want to watch. The token works once and expires after 15 minutes.

You should see this (Linux; on macOS the Service line says `launchd daemon, runs as user _orion`):

```text
 ◆ Orion Node installer                                  0.2.0 · linux/arm64 · systemd

   ✓ Downloaded                    14.4 MB
   ✓ Verified                      sha256, signature not checked
   ✓ Installed                     /usr/local/bin/orion-node
   ✓ Service                       systemd unit, runs as user orion
   ✓ Enrolled                      web-01 with orion.example.com
 ╭ web-01 is connected ──────────────────────────────────────────────────────────── ● ╮
 │ Doctor       ✓ 13 passed                                                           │
 │ Collecting   CPU · memory · disk · network · load · uptime  (no setup needed)      │
 │ Detected     2 things to monitor                                                   │
 │                                                                                    │
 │ ▸ orion-node setup   or open https://orion.example.com/servers                     │
 ╰────────────────────────────────────────────────────────────────────────────────────╯

  No questions were asked. Setup is optional; host metrics are already reporting.
```

Your numbers, names and version will differ. The installer downloads the Node, checks its checksum,
installs it as a service, registers the machine with Core and starts it. It asks no questions.

**Check it.** The machine appears under **Servers** in the Console within a minute. On the machine:

```sh
orion-node status
```

```text
web-01 [up] connected to orion.example.com
last report 22:38:23 (10 s ago), queue 0 pending
next: orion-node setup (2 suggestions waiting)
```

```sh
orion-node doctor
```

```text
  [ok]   Config           /etc/orion/config.yaml
  [ok]   Service          systemd · pid 229
  [ok]   State database   /var/lib/orion/state.db
  [ok]   Version          0.2.0
  [ok]   DNS              orion.example.com resolves to 203.0.113.10
  [ok]   TLS              valid until Jan 2, 2027
  [ok]   Core reachable   orion.example.com 24 ms
  [ok]   Token            valid, node web-01
  [ok]   Clock skew       +0.7 s (limit 5 s)
  [ok]   Last scan        22:33 · found 2 things
  [skip] Scan access      nothing blocked
  [ok]   Log access       can read /var/log/orion/node.log
  [ok]   Log sources      no product log sources configured
  [skip] Log keys         no log sources
  [ok]   Spool            0 pending
  [ok]   Disk space       384 GB free

  14 passed   0 warnings   0 failed   exit 0
```

`doctor` exits 0 when nothing failed and 1 when something did; each problem comes with the exact fix.
You do not need to put `sudo` in front of `orion-node` commands: it asks for privileges itself when a
command needs them.

### Installing without the Console's command

You need the Core address and a way for the Node to register. Either use a one-time token from
**Add server**, or the shared secret set as `ORION_ENROLLMENT_TOKEN` on Core (see
[Enrollment secret](#enrollment-secret)):

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='<token or shared secret>' bash -s -- --core-url https://orion.example.com
```

Useful options: `--version vX.Y.Z` pins a release, `--config ./file.yaml` installs your own config
(start from [`orion-node-config.yaml`](https://github.com/sunday-studio/orion/releases/latest/download/orion-node-config.yaml),
and see the [configuration reference](node-config-reference.md)), `--no-start` installs without
starting, `--dry-run` prints what it would do, `--user` installs for the current user only (see
[Per-user install](#per-user-install)). `orion-node-installer.sh --help` lists them all.

## 2. Choose what to monitor

A fresh Node already reports CPU, memory, disk, network, load and uptime. To also check services,
containers, ports and disks, let it look at the machine and suggest what to watch. Nothing is added
until you say so.

```sh
orion-node discover
```

```text
Discover: found on web-01, 2 things
Services (1)
  [ ] orion-node        Check it is running             medium
Storage (1)
  [ ] /var/lib/orion    Track the space it uses         medium
```

Pick what to monitor in a guided form, or accept every high-confidence suggestion in one go:

```sh
orion-node setup
orion-node setup --accept-discovered --yes
```

```text
  ok: Core reachable 2ms   Server token valid

[ok]   Scanned: found 2 things
[ok]   Accepted: 0 high-confidence
[ok]   Config written: /etc/orion/config.yaml
[ok]   Applied: no restart

  --dry-run prints the diff and stops. Log sources always need a key.
```

`--dry-run` shows the change to the config and stops; secret values in it are hidden. `setup` rewrites
the config file in a standard layout, so comments are not kept; to edit by hand and keep your comments,
use `orion-node config edit`. The suggestions also appear in the Console under the server's **Detected**
tab.

## 3. Day to day

| Command | What it does |
|---|---|
| `orion-node status` | Whether the Node is running and connected. `--short` is one line; `--watch` redraws live. |
| `orion-node doctor` | Checks the config, Core, the token and the service, and says how to fix what is wrong. |
| `orion-node logs` | Recent Node log lines. `--lines 200`, `--level error`, `--since 1h`. |
| `orion-node usage` | What the machine's apps cost: CPU, memory, storage. |
| `orion-node config show` | The settings in effect and where each came from. `config validate`, `config diff` and `config edit` too. |
| `orion-node maintenance down "kernel upgrade" --for 1h` | Mute alerts for planned work; `orion-node maintenance up` ends it early. |
| `orion-node restart` | Restart the service after editing the config. |

Every command takes `--help`. `status`, `doctor`, `discover` and `config show` take `--json` for
scripts. The [CLI cheat sheet](server-cli-cheatsheet.md) has the rest, including exit codes.

## 4. Upgrade

```sh
orion-node update
```

```text
   [ok] Downloaded                    14.4 MB
   [ok] Checksum matches
   [ok] Service stopped
   [ok] Swapped binary
   [ok] Healthy

   [ok] Updated to 0.3.0 in 1 s
     previous version kept · undo with orion-node update --rollback
```

It downloads the matching release, verifies its checksum, swaps the binary, restarts the service and
waits until the Node reports healthy. Config and state are kept, so the machine keeps its identity;
do not delete `state.db`. `orion-node update --version 0.3.0` installs a specific release.

**Undo it:**

```sh
orion-node update --rollback
```

```text
   [ok] Stopped service
   [ok] Restored 0.2.0
   [ok] Started and healthy

   [ok] Rolled back to 0.2.0
```

**Repair a broken install** by running the installer again. It keeps the config and state, so the
machine keeps its identity, and a Node that is already current is left alone:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo bash -s -- --core-url https://orion.example.com
```

**Coming from the old agent?** The Node used to be called the agent. Upgrade Core first, then run the
installer on each server; see [Upgrade notes: the agent is now the node](upgrade-notes-node-rename.md).

## 5. Uninstall

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo bash -s -- uninstall --yes
```

It stops the service and removes the binary and service file. Config, state and the service account
are kept, so installing again later gives the same server identity. To remove everything, add
`--purge`:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo bash -s -- uninstall --purge --yes
```

```text
[03] Binary
     ok: removed /usr/local/bin/orion-node

[04] Config
     ok: removed config directory

[05] State
     ok: removed state directory

[06] Logs and runtime files
     ok: removed /var/log/orion

[07] Service account
     ok: removed service user and group

Orion Node uninstalled.
```

The server stays listed in the Console after an uninstall. Remove it there (open the server, choose
**Remove server**) to drop it and decide what happens to its history.

## Where things live

| | Linux | macOS |
|---|---|---|
| Binary | `/usr/local/bin/orion-node` | `/usr/local/bin/orion-node` |
| Config | `/etc/orion/config.yaml` | `/usr/local/etc/orion/config.yaml` |
| State (identity, token) | `/var/lib/orion/state.db` | `/usr/local/var/lib/orion/state.db` |
| Log | `/var/log/orion/node.log` | `/usr/local/var/log/orion/node.log` |
| Service | `/etc/systemd/system/orion-node.service` | `/Library/LaunchDaemons/com.orion.node.plist` |
| Runs as | user `orion` | user `_orion` |

The service logs are JSON lines; `orion-node logs` prints them readably and falls back to
`journalctl -u orion-node` (Linux) or `launchctl print system/com.orion.node` (macOS) when the log file
is not there yet.

## Per-user install

`--user` installs without root for the current user: the binary goes to `~/.local/bin`, config to
`~/.config/orion`, state to `~/.local/state/orion`, and the service is a systemd user unit or a launchd
agent. Use it only where you cannot get root. Add `--user` to the installer and to the uninstall
command. The service commands (`orion-node start`, `stop`, `restart`, `update`) are written for the
system install and ask for `sudo`; for a per-user install manage the unit with `systemctl --user` or
`launchctl`, and to upgrade run the installer again with `--user` (it keeps config and state).

## Docker monitors on Linux

Docker container monitors call the local `docker` command, and the Node runs as the `orion` user, which
cannot use Docker by default. Docker access is root-equivalent, so it is opt-in. To allow it:

```sh
sudo usermod -aG docker orion
orion-node restart
sudo -u orion docker ps   # should list containers
```

## Enrollment secret

When Core sets `ORION_ENROLLMENT_TOKEN`, a machine can also register with that shared secret instead of
a one-time token. Pass it exactly as in the install command above (`ORION_ENROLLMENT_TOKEN='...'`), so
it never appears in a process list. The installer writes it into a newly generated config with
mode `0600`. A Node that is already registered does not need it again, and the secret stops working for
new machines if an administrator replaces it in the Console's settings.

A Node keeps a `machine-identity.json` next to its state database. If that directory is copied to
another host (a cloned VM or image) the machine id no longer matches and the copy registers as a new,
distinct server instead of impersonating the original.

## Token recovery

If Core rotates or reissues a server's token, apply the replacement to the existing state instead of
registering again:

```sh
orion-node token apply --token-file /secure/path/replacement-token
orion-node restart
```

This keeps the server's identity, monitors and queued reports; prefer `--token-file` so the token stays
out of shell history. If Core rejects the stored token, the Node stops reporting and leaves its state
intact; `doctor` then says so and how to fix it. If the local credential was lost entirely, use
`orion-node reconfigure --token-file /secure/path/add-server-token` with a fresh one-time token from
**Add server**.

## Release integrity

Every release publishes `SHA256SUMS` plus a keyless [cosign](https://docs.sigstore.dev/) signature
(`SHA256SUMS.sig`, `SHA256SUMS.pem`) made by the `Node Binaries` release workflow. The installer always
downloads `SHA256SUMS` and refuses to install if a downloaded file is missing from it or does not match.
If `cosign` is installed it also checks that the signature came from this repository's release workflow;
pass `--require-signature` to make that mandatory.

- Pin a version in production: `--version vX.Y.Z`.
- `curl ... | bash` runs the installer before any verification can happen. To close that gap, download
  `orion-node-installer.sh` and `SHA256SUMS`, check them yourself, then run the script.
- `--skip-verify` exists only for releases published before checksums and prints a loud warning.
- Mirrors and air-gapped installs can host the release files and set `ORION_RELEASE_BASE_URL`.
