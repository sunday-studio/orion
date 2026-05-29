# Incident Readiness PR Checklist

Use this checklist for incident readiness branches before opening a PR into `main`.

## Scope

- Name the Maat goal and ticket ids in the PR body.
- List the branch name and whether the branch is based on `origin/main` or another integration branch.
- Separate source edits, docs edits, generated files, and Maat storage updates.
- Call out any unrelated dirty files or concurrent work intentionally left untouched.

## Product Readiness

- State which incident readiness gap the PR closes.
- Identify any remaining incident readiness gaps, especially lifecycle actor/note metadata, next actions, payload redaction, public draft workflow, or E2E coverage.
- For public incident work, confirm internal incident ids, raw report payloads, alert internals, private notes, and operator-only timeline text are not exposed through public routes.
- For lifecycle work, confirm allowed transitions, repeated actions, covered recovery, and covered expiry behavior are covered or explicitly out of scope.

## Verification

- Record every command that was run, including package or app path.
- Include failing commands with the failure reason if a test could not be made green in this branch.
- For Core route or response changes, run or justify skipping OpenAPI generation and SDK generation.
- For Console incident workflow changes, include browser or E2E evidence when the UI surface changed.
- For docs-only changes, run a lightweight validation command such as `rg` over the changed docs plus `maat validate`.

## Maat Evidence

- Claim every assigned ticket before work starts.
- Add progress comments for material docs, code, or planning-state changes.
- Complete only tickets with concrete evidence: commit hash, PR URL, tests, or exact files and verification commands.
- If reconciling stale Maat rows, cite both the existing completion event or audit note and the current repo evidence that proves the row is complete.
- Run `maat validate --storage /Users/casprine/Desktop/vendor/personal/maat-storage` before final handoff.
- Run `maat sync --storage /Users/casprine/Desktop/vendor/personal/maat-storage --message "status(orion): update maat" --push` after completing Maat updates, or report why sync/push was blocked.

## PR Body

Include these sections:

```md
## Summary
- ...

## Incident Readiness
- Goal: ...
- Tickets: ...
- Remaining gaps: ...

## Verification
- ...

## Generated Files
- None, or list generated files separately.

## Maat
- Claims/comments/completions: ...
- Validation/sync: ...
```
