# Issue log

Known problems and follow-ups that are not fixed yet, newest first. Add a row when you find one, and move it to
"Resolved" with the commit when it lands. Product history stays in `docs/milestones/`.

## Open

| ID | Found | Area | Issue | Next step |
|---|---|---|---|---|
| I-011 | 2026-10-30 | node | On macOS the Containers scan says "Nothing to scan" even though Colima is running. The launchd service PATH is `/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`, so a Homebrew `docker` in `/opt/homebrew/bin` is not found, and Colima's socket lives in the user's home, which `_orion` cannot read | Add `/opt/homebrew/bin` to the plist PATH; decide how `_orion` can reach a Colima or Docker Desktop socket, or document that Docker monitoring is Linux only |
| I-010 | 2026-10-30 | console | The status bars in the monitor list are drawn with uneven widths, so some bars look wider than their neighbours (subpixel rounding). The hover tooltip only says "up", with no time or detail | Size the bars from whole pixels or a fixed grid; show the time and status in the tooltip. Screenshot: Monitors list, status column |
| I-009 | 2026-10-30 | node | The Console showed the container scan as off on a Node where nothing had turned it off, so containers were not listed until it was switched on. Cause not found: the Node's default is all kinds on | Check how a Node's scan kinds are stored and what the Console sends; confirm a fresh install lists containers |
| I-008 | 2026-10-30 | node | `EnsureSocketDir` follows symlinks, so an admin could plant a symlink at `/usr/local/var/run/orion` and have root chown its target. Low impact (reviewer P3) | `Lstat` the folder first and skip it when it is a symlink or not a folder |
| I-007 | 2026-10-30 | node | No tests for the root, chown and `_orion`-missing branches of `EnsureSocketDir`, or for the `sudo orion-node restart` doctor hint; the ignored error is not logged (reviewer P3) | Inject the OS, uid and lookups so the branches are testable; log the error at debug level |
| I-006 | 2026-10-30 | deploy | The Node installer runs its final doctor check as the invoking user, so it can report a false failure on macOS where the socket and config belong to `_orion` (`deploy/scripts/node-bootstrap.sh`) | Run that check through the installer's `$SUDO` |
| I-005 | 2026-10-29 | node | About 87% of `service_log_entries` rows are routine per-check INFO lines, which is most of the growth in `orion.db` | Ship only WARN and ERROR from the Node by default |
| I-004 | 2026-10-29 | console | Deleting a Node monitor removes it from Core now, but the dialog wording does not say so | Reword the delete dialog for Node monitors |
| I-003 | 2026-10-29 | core | Slow SQLite was diagnosed right after a restart. Not re-checked once Core had been up for hours | Re-measure after a day of uptime before changing anything |
| I-002 | 2026-10-29 | console | Pages may still flicker on route changes when data is cached and refetched | Set a default query `staleTime` of 15 to 30 s if it shows again after the 150 ms fade |
| I-001 | 2026-10-29 | release | A Core release only publishes to GHCR. The Releases page and `orion-node update` follow the "Node Binaries" workflow, so the page can show an older Latest | Decide whether Core releases should also create a GitHub release without breaking `orion-node update` |

## Resolved

| ID | Fixed | Issue | Fix |
|---|---|---|---|
| R-002 | 2026-10-30 | Badges rendered every word with a capital ("Waiting For The Node") | The badge no longer forces title case and capitalizes only the first letter |
| R-001 | 2026-10-30 | macOS Node never created its control socket, so `status`, `doctor` and `restart` failed or hung | The installer and `start`, `restart` and `update` create the socket folder; restart shows progress (`1a42b0a6`) |
