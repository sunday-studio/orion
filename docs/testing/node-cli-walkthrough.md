# Node CLI walkthrough

One command walks every screen on the design canvas (CLI and node page) against the real binary:

```sh
make node-walkthrough
```

For each screen it prints the board's text, waits for Enter, runs the real command against a real
local Core and an enrolled node, and asks "Same as the board?". Nothing is installed and no root is
needed; everything lives in `/tmp/orion-walk-<your uid>` and is stopped when you quit.

- Core runs on `http://127.0.0.1:8999`. For the Console, run `make console-dev` in another terminal,
  open the address it prints (usually `http://localhost:5173`; the dev server moves to the next port if another project holds 5173, and Core allows 5173 to 5180) and log in with the username and password the script prints (new on every
  run; the Console always asks for one). Core is bound to `127.0.0.1` only.
- `make node-walkthrough ONLY=status` shows only the screens whose board or name contains "status".
- Each screen puts the sandbox in the state it needs (a fresh host and empty Core for setup and enrol,
  the node stopped for "not running"). Interactive screens
  (setup, maintenance form, `--watch`, `config edit`) run on your terminal; press `q` or Ctrl+C to
  leave them. For anything asking for a one-time token, type `play`.
- `docs/testing/node-cli-boards.json` holds the board text, taken from the canvas. Regenerate it when
  the boards change.

## What it cannot show

Marked and skipped, because they need a real server:

- the install script, `start`, `stop`, `restart`, `service install`, `update` and rollback (service
  manager, root, a published release);
- the log viewer (needs log shipping with an ingest key from the Console);
- the "Not enrolled" status and "No recent report" (the first needs a running daemon with no
  registration, the second an 11 minute wait).

In the sandbox `doctor` reports "Service is not installed" and exits 1, and `emit` and `run-job` say
there is no ingest key. Both are expected there.

## Known differences from the boards

- `config show` follows the Config board; the auto-detect board's "hand-written config wins" screen
  draws separate `discovery` and `monitors` panels. Undecided which to follow.
- A continuous `orion-node run` prints one dim line after "Registered"; the board prints nothing.
- `help topics` and the release notes before `update` are additions, not on the boards.
