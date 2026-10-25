# Multi-agent agreement

How multiple coding agents work together in this repository. Read the "Overrides for this repo"
section first: where it differs from the rules below, the overrides win.

## Overrides for this repo

These are standing decisions (see `docs/plans/decision-log.md`) that replace the matching default rules:

- **No AI attribution in commits.** No `Co-Authored-By:` trailers or "Generated with" lines naming Claude or any AI model, in commits or pull request text, even when a tool suggests one (see AGENTS.md).

- **Direct help is allowed.** Threads (`orion:web`, `orion:design`, `orion:backend`, `orion:agent`)
  may send short, single-question messages straight to the design or backend thread for layout or API
  help on the active feature (D-027). Everything else still goes through the coordinator, and
  decisions a thread cannot settle go to the coordinator, who decides and logs them.
- **Commit and push.** Workers commit as they go and push reviewed work straight to `main` with
  `git push origin HEAD:main` (never force). No pull requests (D-023). Independent review still
  applies before pushing.
- **One feature at a time.** The active feature is a canvas page; the next one does not start until the
  user approves the current one (D-027, D-032).
- **The user does the manual interaction.** Agents do not drive the browser or the live app by hand;
  verification is by automated tests (D-032). Never generate review images (D-033).
- **Long-running coordinator.** The coordinator stays for the whole feature-by-feature build rather than
  one round; it is retired at the end of the build, not after each round.
- **CI is off** until the last step before go-live (D-024). Local checks are the gate.

Everything else below applies as written.

## 1. Roles
- **Coordinator:** plans, writes briefs, spawns agents, merges results. Does not do the implementation work itself.
- **Worker:** does one brief, in its own file set, and returns one short report.
- **Checker:** runs once at the end, after all workers finish.

## 2. Before spawning, the coordinator must
- List every file or directory each worker may touch. No file appears in two lists.
- If two pieces share a file or one needs the other's output, put them in one worker or run them in order.
- Write a complete brief for each worker: area, what "done" looks like, what to leave alone, and what to do if blocked.
- Use fresh contexts by default. Fork only when the subtask truly needs the current conversation's history.

## 3. Worker rules
- Touch only your listed files. If you need to change something outside them, stop and report it. Don't edit it.
- Don't message other workers, except the direct design and backend help allowed in the overrides above. The other messages are the brief in and the report out.
- Put detail in files or commits, not in chat.
- Commit as the overrides say. Don't switch branches.

## 4. Report format (max ~10 lines)
- What changed, as file paths.
- What was verified, and how.
- What was not verified or left undone.
- Anything blocked or needing a decision.

## 5. Coordinator rules after spawning
- Don't poll. Wait for the reports.
- Check a report against the diff before calling the work done.
- A blocked worker gets one answer from the coordinator. If the answer changes another worker's scope, re-brief both.

## 6. Merge and check
- Merge worker results one at a time.
- The Checker runs tests or a review on the combined result once, and says what it could not cover.
- Failures go back to the worker who owned the files, not to a new agent.

## 7. Limits
- At most 3-4 workers at once.
- Close or compact any session that passes about 150 turns.
- Retire the coordinator when the build ends. Start a new one for the next build.

## 8. Coordinator prompt (paste to start a round)
```
You are the coordinator. Follow COORDINATION.md.

Goal: <one sentence>
Repo/area: <path>

Do this:
1. Split the goal into pieces with no shared files. List each piece's files.
2. If any pieces overlap, merge them or order them. Show me the split before spawning.
3. Write a full brief per worker (area, done criteria, off-limits, blocked behavior).
4. Spawn fresh workers, max 4. Do not poll them.
5. Check each report against its diff. Then run one checker on the combined result.
6. Report to me in under 10 lines: done, not verified, anything needing my decision.
```
