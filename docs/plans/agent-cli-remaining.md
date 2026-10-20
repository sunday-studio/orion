# Agent CLI: functionality and designs left

Updated 2026-10-20. Method: one functionality at a time, each built to its design board text,
checked by literal-match tests, reviewed by an independent agent, then pushed to `main`.

Note: the public design canvas needs a signed-in session; agents cannot read it. Boards are
taken from `apps/agent/internal/cli/testdata/design/*.txt` (extracted earlier) and the checked-in
`docs/design/artboards/cli-*.html`. Visual acceptance stays with the user.

## Done to the board

| # | Functionality | Evidence |
|---|---|---|
| 1 | Discovery and `discover` | `discover-render_test.go`, `testdata/design/discover*.txt` |
| 2 | `status` (six states, discovery states, `--short`, `--watch`) | `status-render_test.go` |

## Left, in order

| # | Functionality | Left to do |
|---|---|---|
| 3 | `doctor` | Add the missing log key check (sources without an ingest key), literal test against `cli-doctor.html` |
| 4 | `setup` / `enroll` / credentials | Re-check against board; add the scan-and-pick step that uses discovery |
| 5 | Install script output | Match board text for plan, steps, doctor box, failure |
| 6 | `logs` viewer | Match board states (following, paused, snapshot, empty, error) |
| 7 | Help and errors | Match board help grouping and error layout |
| 8 | `service` / `start` / `stop` / `restart` / `run` | Preflight and recovery wording |
| 9 | `maintenance` | Output wording |
| 10 | `update` / `rollback` | Output states |
| 11 | `config show/diff/validate/edit` | Output wording |
| 12 | `emit` / `run-job` | Output wording |
| 13 | `usage --watch` | Live view |
| 14 | Discovery reporting to Core + Console "Detected" panel | Needs Core endpoint |
| 15 | Release pipeline and signing; docs | Later |
| 16 | F2 container image / DaemonSet | Deferred to the very end |
