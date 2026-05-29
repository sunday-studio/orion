# Maat Setup Friction

This records the friction observed while initializing Maat for the Orion repository on 2026-05-26.

## Context

Command run from the Orion repo root:

```sh
maat initialize
```

Maat registered this repository as project key `orion` and linked storage at:

```txt
/Users/casprine/Desktop/vendor/personal/maat-storage
```

## Friction Points

### Storage Path Requires Broader Filesystem Access

The first run failed because Maat tried to create storage outside the repository sandbox:

```txt
maat: mkdir /Users/casprine/Desktop/vendor/personal/maat-storage/projects: operation not permitted
```

Impact:

- Agents running with workspace-only filesystem access cannot initialize Maat without escalation.
- The error is technically correct, but it does not explain that the storage path is outside the repo write boundary.

Suggested improvement:

- Before writing, print the storage path and explain that Maat needs write access there.
- Provide a repo-local fallback option or explicit `--storage` guidance.

### Setup Prints A Long Generic Agent Guide

After successful initialization, Maat prints a full cross-agent setup guide. It is useful, but long.

Impact:

- The important repo-specific facts are buried:
  - project key: `orion`
  - storage path
  - commands to run next
  - instruction snippet to persist
- The user asked to “follow the steps,” so the agent had to infer which parts were mandatory now versus reference material.

Suggested improvement:

- End with a compact “do these now” checklist.
- Separate the long generic guide into “reference.”

### Storage Repo Had No Upstream Tracking

`maat status` and `maat projects` initially warned:

```txt
git pull --rebase failed: There is no tracking information for the current branch.
```

Impact:

- Maat auto-pull failed before reads.
- The project still appeared to load, so the warning was easy to miss.

Resolution applied:

```sh
git fetch origin main
git branch --set-upstream-to=origin/main main
```

Suggested improvement:

- `maat setup` should detect a Git storage branch without upstream tracking.
- It should either set tracking when safe or print the exact fix command.

### Tracked SQLite Cache Became Dirty

The storage repo had a dirty tracked cache file:

```txt
M .maat/index.sqlite
```

Maat’s own setup text says SQLite is only a local search cache and Markdown is the source of truth.

Impact:

- Auto-pull failed because Git saw unstaged changes.
- This conflicts with the intended “cache only” role of SQLite.

Resolution applied:

```sh
git restore .maat/index.sqlite
```

Suggested improvement:

- Do not track `.maat/index.sqlite`, or make Maat avoid modifying tracked cache files.
- If a tracked cache is intentional, Maat should auto-commit it consistently or explain why it is dirty.

### Auto-Pull Later Failed With Multiple Branches

After upstream tracking was set and Maat commands wrote more state, a later read produced:

```txt
git pull --rebase failed: fatal: Cannot rebase onto multiple branches.
```

Observed branch config looked normal:

```txt
branch.main.remote origin
branch.main.merge refs/heads/main
```

Impact:

- Maat read commands can still proceed, but the auto-sync warning reduces confidence.
- It is not obvious whether Maat, Git config, or the storage repo has the invalid pull state.

Suggested improvement:

- When auto-pull fails, print the storage repo path and the exact Git command Maat ran.
- Add `maat doctor` or `maat storage doctor` to validate remotes, branch tracking, dirty cache files, and push/pull readiness.

### Push Was Blocked By Safety Policy

The explicit command:

```sh
maat sync --storage /Users/casprine/Desktop/vendor/personal/maat-storage --message "status(orion): add maat repo instructions" --push
```

was blocked because it would push project-memory data to a personal GitHub repo.

Impact:

- Local Maat state was written and validated, but explicit push could not be completed by the agent.
- This is a policy boundary, not a Maat bug, but Maat’s setup assumes the agent can push storage changes.

Suggested improvement:

- Treat personal/external storage pushes as a documented approval step.
- Provide a no-push completion mode in the setup instructions.

### Ticket State Looked Inconsistent

After a ticket was completed, later `maat status`/`maat project show` still showed active/open work for the earlier setup ticket.

Impact:

- It was unclear whether the ticket was actually complete.
- The agent created a second ticket for this document to avoid mutating ambiguous state.

Suggested improvement:

- `maat ticket complete` should make the completed state obvious in `maat status` and `maat project show`.
- If status is stale because the search/index cache is stale, the command should say so explicitly.

## Recommended Fix Order

1. Add a Maat storage doctor command for Git remotes, upstream tracking, dirty cache files, and push readiness.
2. Stop tracking or dirtying `.maat/index.sqlite`.
3. Make `maat setup` repair or clearly report missing upstream tracking.
4. Shorten `maat initialize` output into an immediate checklist plus reference section.
5. Update setup instructions to distinguish local sync from push, especially for personal storage repos.
6. Clarify ticket completion state in status and project views.

## Current Working State

As of this note:

- Orion is registered in Maat as `orion`.
- The storage repo is linked at `/Users/casprine/Desktop/vendor/personal/maat-storage`.
- `AGENTS.md` contains the Maat project-memory instruction.
- `maat validate` passed after the instruction update.
- Explicit Maat push from the agent was not performed because of safety policy.
