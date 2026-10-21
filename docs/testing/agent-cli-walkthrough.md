# Agent CLI walkthrough

One command walks every screen on the design canvas (CLI and agent page) against the real binary:

```sh
make agent-walkthrough
```

For each screen it prints the board's text, waits for Enter, runs the real command against a real
local Core and an enrolled agent, and asks "Same as the board?". Nothing is installed and no root is
needed; everything lives in `/tmp/orion-walk-<your uid>` and is stopped when you quit.

- Core runs on `http://127.0.0.1:8999` with authentication off, so there is no login. For the Console,
  run `make console-dev` in another terminal and open `http://localhost:5173`.
- `make agent-walkthrough ONLY=status` shows only the screens whose board or name contains "status".
- Each screen puts the sandbox in the state it needs (a fresh host and empty Core for setup and enrol,
  Core stopped for the offline screen, the agent stopped for "not running"). Interactive screens
  (setup, maintenance form, `--watch`, `config edit`) run on your terminal; press `q` or Ctrl+C to
  leave them. For anything asking for a one-time token, type `play`.
- `docs/testing/agent-cli-boards.json` holds the board text, taken from the canvas. Regenerate it when
  the boards change.

## What it cannot show

Marked and skipped, because they need a real server:

- the install script, `start`, `stop`, `restart`, `service install`, `update` and rollback (service
  manager, root, a published release);
- the log viewer (needs log shipping with an ingest key from the Console);
- the "Not enrolled" status, a rejected token and "No recent report" (need Core with authentication on,
  or an 11 minute wait).

In the sandbox `doctor` reports "Service is not installed" and exits 1, and `emit` and `run-job` say
there is no ingest key. Both are expected there.

## Known differences from the boards

- `config show` follows the Config board; the auto-detect board's "hand-written config wins" screen
  draws separate `discovery` and `monitors` panels. Undecided which to follow.
- A continuous `orion-agent run` prints one dim line after "Registered"; the board prints nothing.
- `help topics` and the release notes before `update` are additions, not on the boards.
