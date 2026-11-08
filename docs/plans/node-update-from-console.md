# Updating a Node from the Console

Status: approach B chosen, on by default (owner decision). All phases are built (Node, installer, Core,
Console, release signature check). It still needs a security review before it is pushed or
announced.

## What exists

- `orion-node update` downloads the release for the machine, checks its SHA-256 against `SHA256SUMS`,
  swaps the binary, restarts the service and rolls back if the Node is not healthy within 45 seconds.
  It needs root, because the binary lives in `/usr/local/bin` and the service is stopped and started
  with `systemctl` or `launchctl`.
- The Console can already reach a Node through the request queue (`discovery/requests`): the Console
  queues a request, the Node pulls it every minute, applies it and posts a result. A Node says what it
  can take in its `capabilities`.
- Core knows the newest release (`GET /v1/settings/updates`) and each Node's `node_version`.

## Why it cannot be built as is

The daemon runs as the unprivileged user `orion` (`_orion` on macOS) under a hardened unit:
`ProtectSystem=strict`, `NoNewPrivileges=true`, no capabilities, and write access only to
`/var/lib/orion`, `/etc/orion` and `/var/log/orion`. It cannot write `/usr/local/bin/orion-node` or
restart itself with `systemctl`. `UpdateNode` is also tied to a terminal (it prints progress and
re-runs itself under `sudo`).

## Two ways to get permission

**A. A root helper.** A systemd path unit (a launchd `WatchPaths` job on macOS) watches a request file
in `/var/lib/orion`. The daemon writes the wanted version there and a root one-shot service runs
`orion-node update --version X`.

- The daemon, which parses network input, can make a root process run. Everything depends on the
  helper validating that file.
- Adds a unit per platform to install, upgrade and uninstall.

**B. The Node owns its binary (recommended).** Move the program file to a directory the service user
owns, for example `/var/lib/orion/bin/orion-node`, and point the unit at it. `/usr/local/bin/orion-node`
becomes a symlink so the CLI still works. The daemon downloads, verifies and swaps the file itself and
exits so the service manager restarts it (`Restart=on-failure` already does, `launchd` `KeepAlive`
too).

- No privilege for the daemon itself. It already runs as `orion`; rewriting its own binary adds
  persistence under that user. But see "Accepted risk" below: the root CLI also runs this file.
- `ReadWritePaths` already includes `/var/lib/orion`, so the unit needs no loosening.
- Cost: existing installs have to move once (see Phase 0), and the unit and launchd plist change.

The rest of this plan assumes B. If B is rejected, A reuses everything except Phase 1.

## Design (B)

**Opt out, not opt in.** On by default, decided by the owner. `remote_update: false` in the Node's
config turns it off. The Node advertises the capability `self_update` only when it is on, the
binary is in a folder it can write and it is a release build. A container or `ORION_CONTAINER` Node
never advertises it.

**Request.** `POST /v1/nodes/{id}/update` (Console, admin) queues a request of type `update_node`.
Core checks that the Node advertises `self_update`, that the requested version is the one Core found
in its own release check (the Console cannot name an arbitrary version), and that it is newer than the
Node's. Core records the checksum it read from the release's `SHA256SUMS` in the request. The action is
audited and limited to one open update per Node.

**On the Node.**
1. Refuse unless `remote_update` is on and the version is newer than the running one.
2. Download the asset for its own OS and architecture from the GitHub release, never from a URL in the
   request. The download must match the checksum in the request **and** the one in the release's
   `SHA256SUMS`. Two sources have to agree.
3. Stage next to the current file, run `<new> version` and check it prints the requested version.
4. Keep the old binary as `orion-node.prev`, write an `update-pending` marker, rename the new file in,
   report `applied` as soon as it is running, and exit so the supervisor restarts it.
5. On start, if the marker exists the new process must complete one report to Core within two
   minutes, then clears the marker. If it does not, the next start restores `.prev`, clears the marker
   and reports `failed` with the reason.

**Idempotence.** Requests are redelivered after five minutes. A Node already on the requested version
reports `applied` without doing anything, so a restart mid-update never loops.

**Console.**
- The Node dialog offers **Update from here** (when the Node advertises `self_update`) and **Run it on
  the server** (the command, always). A Node without the capability says how to turn it on.
- The server page shows the request state: queued, updating, updated to X, failed with the reason.
  The Servers list "Update available" filter keeps working while a Node updates.
- The update notice goes away once the Node reports the new version.

## Phases

0. **Layout.** Installer puts new installs in the owned directory with the symlink. `orion-node update`
   (the root CLI) gains a one-time migration for existing installs and rewrites the unit and plist.
   Docs: `node-install-upgrade.md`, `docs/deployment/release-packaging.md`.
1. **Node.** An update routine that runs inside the daemon, split out of `UpdateNode` (no terminal, no
   sudo), with the checks above, the marker and the restore. Tests with the existing injected
   `updateEnv`: success, checksum mismatch, wrong version, unhealthy start, restart mid-update.
2. **Core.** Request type and validation, `POST /v1/nodes/{id}/update`, audit, the `self_update`
   capability, OpenAPI, `docs/node-core-contract.md`.
3. **Console.** The dialog option, request state on the server page, tests.
4. **Signatures (built).** The Node verifies the Sigstore keyless signature on `SHA256SUMS` itself,
   offline: the Fulcio certificate chain against embedded roots, the OIDC issuer, the workflow
   identity and the signature. It does not consult the transparency log; the certificate is checked as
   of its issue time, and a replayed older release is refused by the newer-than-running rule. The
   roots are in `apps/node/internal/selfupdate/fulcio-roots.pem`; refresh them with
   `go run ./internal/selfupdate/gen-roots` when Sigstore rotates its CA, or checks fail closed.
   `orion-node update` (the root CLI) still checks only the checksum.

Phases 1 to 3 can be built in that order and merged in that order. Phase 4 should land before the
feature is announced.

## Threats the review should cover

| Threat | Mitigation |
|---|---|
| A compromised Core names a malicious release | Version must be newer; the file comes from the GitHub release and must match its signed checksums (phase 4); the Node opts in |
| A compromised GitHub release | The signature on `SHA256SUMS` has to verify as the release workflow, and Core's checksum has to agree with the file |
| A Console login updates every Node | On by default (owner decision); `remote_update: false` per Node; audited; one open update per Node |
| Downgrade to a vulnerable version | Only newer versions; rollback stays a root CLI action |
| A bad release takes Nodes offline | Unhealthy start restores the previous binary; Core sees the failure |
| The daemon rewrites its own binary | **Accepted risk, see below.** It can no longer be said that this adds no privilege |

## Open questions

- Is moving the binary acceptable for packaged installs (a future `.deb` would put it elsewhere)?
- Should Core update every Node behind in one action? Proposed: not in the first version.
- Do we want a maintenance window or "update when idle"? Proposed: no, the restart takes seconds.

## Accepted risk: root commands run a binary the service user can replace

With this layout `/usr/local/bin/orion-node` is a link to `/opt/orion/orion-node` (macOS:
`/usr/local/lib/orion/orion-node`), and that file is owned by the service user. An administrator who
runs `sudo orion-node ...` (`update`, `start`, `stop`, `restart`, the migration step) executes it as
root. A compromised daemon could plant a binary there and get root the next time someone does.
Before this change the file was root-owned, so that path did not exist.

This was missed in the first version of this plan and found in review. It is accepted for now because
the daemon is already the thing being defended, and because the alternative is a root helper, which
has its own cost (see approach A). Mitigations in place: root code never opens an existing file for
writing (it writes a fresh file and renames it, so it cannot be tricked into writing through a link),
the migration refuses a link where the folder should be, and a Node updates only to a release whose
signature verifies. A cheap further mitigation, not built, is a root-owned copy of the binary used by
`sudo` commands.
