# Console and the design canvas

The canvas is settled enough to build from. This groups what is left to bring `apps/console/` in
line with it, and says what each group needs from Core and the Node. Done work is marked.

## Already on `main`

- The sidebar shell (open, collapsed, closed), sibling pages on their own routes, Apps under Logs.
- Services: list, service page, link dialogs, incident and monitor pages, Core API behind them.

## A. Card design and Settings (Console only)

| Item | State |
|---|---|
| `Box` renders the canvas card (pale tray, title row, white raised card) | done |
| Settings pages use flat sections: title and hint on the left, controls on the right | done |
| Settings rows put the detail under the label | done |
| Logs fields list: no type tags, values as indented rows | done |
| Send a log examples: curl and OpenTelemetry only, since Orion has no SDKs | done |
| Empty states: centred, narrower description | done |
| Cards that hold only a table or list use the compact padding (12px top, 4px elsewhere) | done |
| Apps: send a test log as one card with curl and OpenTelemetry tabs | done |
| Detected: log key dialog as option cards, states card without the label column | check against the Console |
| Incident page: the log filter row flat and aligned with the table | check against the Console |

## B. Updates (Core, Console, Node)

The canvas moved Updates into Settings.

| Item | State |
|---|---|
| Core: release check, auto-check and channel settings, check now (`/v1/settings/updates`) | done |
| Console: Version row on General and System, Updates section on System | done |
| Console: Core update dialog (notes, Compose command, skip, remind me later) | done |
| Console: "Update available" pill and popover in the sidebar | done |
| Console: security notice on the Overview | done |
| Console: Servers list "Update available" filter; Node update notice and dialog on the server page | done |
| Update a Node from the Console | not built: needs a decision, see below |

**Update a Node from the Console.** The Node already has `orion-node update`, with checksum
verification and rollback, and the Console can already reach a Node through the request queue
(`discovery/requests`). What is missing is permission. The daemon runs as an unprivileged user under a
hardened systemd unit (`ProtectSystem=strict`, `NoNewPrivileges`), so it cannot replace
`/usr/local/bin/orion-node` or restart itself. It needs a small root helper that the daemon can
trigger, for example a systemd path unit that runs `orion-node update` when the daemon writes a flag
file. That widens what a Console login can do to a host, so it needs an explicit decision and a
security review first. The plan, with the two approaches and a recommendation, is in
[node-update-from-console.md](node-update-from-console.md). Until then the Node dialog shows the
command to run on the server.

## C. Visual pass against the canvas

After A and B, walk the boards one by one against the running Console (light and dark) and list the
differences. Do it per section: Overview and Incidents, Monitoring, Logs, Notify, Settings.
