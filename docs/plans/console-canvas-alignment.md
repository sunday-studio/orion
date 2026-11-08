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

The canvas moved Updates into Settings. None of this exists in Core yet, so it needs the API first.

1. **Core:** a release check (latest version, release date, security flag, last checked), settings
   for auto-check and channel, and a "check now" action. Contract change in
   `docs/node-core-contract.md` and the OpenAPI spec.
2. **Console:** a Version row on General with the new version and its date, the System page's
   Updates section, update states, the sidebar "Update available" pill with its popover.
3. **Nodes:** a Node reports its version already; the Servers list gets an "update available"
   filter and the server page a notice.
4. **Update from the Console:** Core queues an update for a Node, the Node downloads the release,
   checks it, restarts and rolls back if it is unhealthy. Needs an opt-in in the Node config. This
   is a Core and Node change and a security review before any Console work.

## C. Visual pass against the canvas

After A and B, walk the boards one by one against the running Console (light and dark) and list the
differences. Do it per section: Overview and Incidents, Monitoring, Logs, Notify, Settings.
