# Upgrade notes: 0.4.0

0.4.0 adds the Services area and a sidebar to the Console, metric history in Core, update checks, and
updating a Node from the Console. Nothing is removed from the API. Read the Node section if you run Nodes
from a system install.

## Core

Upgrade Core first, then Nodes, as for any minor release.

```sh
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:0.4.0 docker compose up -d
```

- Take a backup first ([SQLite backup and restore](../sqlite-backup-restore.md)). Core applies migrations
  `000058` to `000061` on first start; they only add tables and columns. Core also copies `orion.db` into
  `backups/` before it migrates, but a copy you made yourself can be restored on any machine.
- Core creates `metrics.db` next to the other databases for resource and app metric history. It is history
  only and is not part of the backup page's `orion.db` and `logs.db` steps. Plan for a little more disk; retention is set in the data
  lifecycle settings (see [Product metrics](../product-metrics.md)).
- Core asks the GitHub releases of `sunday-studio/orion` for the newest release about once a day. It is on
  by default, including after an upgrade. On a host that cannot reach `api.github.com`, turn it off under
  Settings, System. See [Core environment](core-environment.md#update-checks).
- Incidents are now grouped by service. Once monitors and apps are linked to a service, a failed check and a
  log burst on that service page once, and a service group takes new incidents while one is active or its
  last event is under 30 minutes old. Alert counts can drop for linked things.

## Nodes

- A Node can now be updated from its server page in the Console. This is on by default. To keep a Node from
  accepting it, set `remote_update: false` in the Node config
  ([Node config reference](node-config-reference.md)). The Node checks the release signature and checksums
  before it replaces its own binary, then restarts under its service manager.
- New system installs keep the Node in its own folder: `/opt/orion-node` on Linux and
  `/usr/local/lib/orion-node` on macOS. A Node installed before 0.4.0 lives elsewhere, so update it from the
  command line, and run the command **twice**: the first run installs this release, and the second, now
  running the new code, moves the binary into the new folder.

  ```sh
  sudo orion-node update
  sudo orion-node update
  ```

  After that, updates from the Console work. The details, and what happens if a folder holds files that are
  not the Node's, are in [Node install and upgrade](node-install-upgrade.md).
- Run Nodes under systemd or launchd. A Node started by hand, or inside a container, refuses Console updates
  because nothing would restart it.
