# Incident Readiness PR Checklist

Use this checklist for incident readiness branches before opening a PR into `main`.

## Scope

- List the branch name and whether the branch is based on `origin/main` or another integration branch.
- Separate source edits, docs edits, and generated files.
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
- For docs-only changes, run a lightweight validation command such as `rg` over the changed docs.

## PR Body

Include these sections:

```md
## Summary
- ...

## Incident Readiness
- Remaining gaps: ...

## Verification
- ...

## Generated Files
- None, or list generated files separately.
```
