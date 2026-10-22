# Upgrade notes: 0.2.0

0.2.0 is the first public release. If you ran an earlier build, **read this page before you upgrade**:
the rename of the agent to the **node** is a breaking change, and the database is migrated in place.

Starting fresh? You can skip this page and follow the [first run checklist](first-run-checklist.md).

## 1. Back up first

Take a backup of both databases before you pull the new image. Migration `000055` renames tables and
columns, and while Core also copies `orion.db` to `/data/backups` before it migrates, a copy you made
yourself is the one you can restore from anywhere.

```sh
docker compose exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker compose exec orion-core sqlite3 /data/logs.db ".backup '/data/logs-backup.db'"
docker cp "$(docker compose ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
docker cp "$(docker compose ps -q orion-core):/data/logs-backup.db" ./logs-backup.db
```

Details and the restore steps are in [SQLite backup and restore](../sqlite-backup-restore.md).

## 2. Upgrade Core

Set the image to the new tag and restart both services:

```sh
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:0.2.0 docker compose up -d
```

On first start Core:

1. writes `/data/backups/orion-pre-migrate-000055-<timestamp>-<pid>.db` (a copy of `orion.db`, newest three kept);
2. applies migration `000055` in one transaction: tables `agents`, `agent_reports` and the other `agent_*` tables
   become `nodes`, `node_reports` and so on, `agent_id` columns become `node_id`, and the stored names that
   carried the old word are rewritten. Rows keep their ids, so a server that was `agent-1234` stays
   `agent-1234`, with the same token. Nothing is deleted;
3. serves the API under `/v1/nodes`. **`/v1/agents` and everything under it now returns 404.**

Check it came up: `curl http://localhost:8999/health` returns `"status":"healthy"`, and **Servers** in the Console
lists the same servers as before.

## 3. Re-install the node on every server

Servers that still run the old agent are not served until you do this. Run the installer again on each one; the
Console's **Servers, Add server** shows the command:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh \
  | sudo ORION_ENROLLMENT_TOKEN='<token>' bash -s -- --core-url https://orion.example.com
```

The installer stops and removes the old `orion-agent` service and binary, installs `orion-node`, and keeps
`/etc/orion/config.yaml`, the state database and the `orion` service user. The node finds its existing
registration, so it does not enrol again. The old agent cannot update itself, because the assets it looks for
are no longer published.

## What breaks

| If you used | Now |
|---|---|
| `orion-agent`, `orion-agent-installer.sh`, the `orion-agent` image, `orion-agent.service`, `com.orion.agent` | `orion-node` and the matching names. The installer removes the old service. |
| `/v1/agents/...`, fields `agent_id`, `agent_*` | `/v1/nodes/...`, `node_id`, `node_*`. The old paths are 404. |
| `ORION_AGENT_*` environment variables | `ORION_NODE_*` |
| `orion-agent ... --json` with `agent_id` | `node_id`; `schema_version` is 3 |
| Alert, event and audit names beginning `agent_` or `agent.` | `node_` and `node.`, including those already recorded |

The full table, the notes for people who call the API, and Docker node instructions are in
[Upgrade notes: the agent is now the node](upgrade-notes-node-rename.md). The behaviour changes that came with
the backend hardening work (private webhooks blocked by default, the Worker delivering alerts, checksummed
installers) are in [Upgrade notes: backend hardening release](upgrade-notes-backend-hardening.md).

## Settings to check

- **Alert email needs a shared secret.** If you set `ORION_ALERT_SMTP_HOST`, also set `ORION_ALERT_EMAIL_SECRET`,
  the same value for `orion-core` and `orion-core-worker`, and set `ORION_PUBLIC_URL`. The Worker has no
  `ORION_JWT_SECRET`, so without the secret every email delivery fails with "email alerts need
  ORION_PUBLIC_URL and ORION_ALERT_EMAIL_SECRET"; monitoring and webhook alerts are not affected.
- **Open node registration.** With `ORION_ENROLLMENT_TOKEN` unset, anyone who can reach Core can register a node
  (rate limited). Set a token, or use only the Console's one-time Add server commands, if Core is reachable from
  the internet.
- **Public status pages are server rendered.** `/status/<slug>` now serves HTML to browsers and keeps its JSON for
  API clients; badges and feeds are unchanged. Pages saved before keep their theme settings.

The complete variable reference is [Core environment variables](core-environment.md).

## Roll back

Stop Core and the Worker, restore the pre-upgrade `orion.db` (your own backup, or the newest
`/data/backups/orion-pre-migrate-000055-*.db`) as described in
[SQLite backup and restore](../sqlite-backup-restore.md), and start the previous image. Servers you already
re-installed as `orion-node` cannot talk to the previous Core, so roll back before you re-install them, or
re-install the old agent.
