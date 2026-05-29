# AGENTS.md

This file defines the working boundaries for humans and coding agents in this repository. Follow these rules before editing files.

## Project Memory

Use Maat as the canonical project memory for this repo. Markdown plus Git is the source of truth;
SQLite is only a local search cache.

This repo is registered as Maat project `orion`. Use this storage path:

```sh
/Users/casprine/Desktop/vendor/personal/maat-storage
```

Before material work, run the current Maat daily loop commands exactly as needed:

```sh
maat sync --storage /Users/casprine/Desktop/vendor/personal/maat-storage --status
maat status --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat project show orion --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat search "<query>" --storage /Users/casprine/Desktop/vendor/personal/maat-storage
```

Create new goals and tickets with the exact Maat forms that include outcome, description, and
acceptance criteria:

```sh
maat goal create orion "<goal title>" --outcome "the concrete outcome this goal should achieve" --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat ticket create orion "<ticket title>" --goal <goal-id> --description "the concrete work another agent should do" --acceptance "clear completion condition" --storage /Users/casprine/Desktop/vendor/personal/maat-storage
```

Claim, comment, and complete tickets with the exact Maat forms:

```sh
maat ticket claim <ticket-id> --project orion --agent "<agent-id>" --ttl 2h --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat ticket comment <ticket-id> "short factual progress note" --project orion --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat ticket complete <ticket-id> --project orion --evidence "tests, commit, PR, or exact verification" --storage /Users/casprine/Desktop/vendor/personal/maat-storage
```

When finished, validate and sync with the current Maat commands:

```sh
maat validate --storage /Users/casprine/Desktop/vendor/personal/maat-storage
maat sync --storage /Users/casprine/Desktop/vendor/personal/maat-storage --message "status(orion): update maat" --push
```

Create or claim a ticket before working. Never create title-only goals or tickets. Add comments for
meaningful progress, blockers, handoffs, and decisions. Complete tickets only with evidence.
Commit finished product changes in this repository before ending the task unless the user explicitly
asks not to commit or committing would capture unrelated or unsafe changes. Commit and push Maat
storage changes when allowed; if remote push is blocked, say so explicitly.

## Repository Map

- `apps/agent/`: Orion Agent. Go daemon, CLI, config/state handling, collectors, registration, and transport to Core.
- `apps/core/`: Orion Core. Go API server, SQLite models/services, OpenAPI spec, generated Swagger docs, and generated `web/` assets served by Core.
- `apps/console/`: Main editable React/Vite product UI. New frontend product work belongs here.
- `packages/sdk/`: Generated/shared API types. Treat as generated unless the task is specifically about SDK generation or contracts.
- `deploy/`: Runtime/deployment assets: Docker Compose, systemd, launchd, install/uninstall helpers.
- `docs/`: Architecture, contracts, plans, milestones, and operational docs.
- `tmp/`: Scratch/build output. Do not depend on it for product behavior.

## Ownership Rules

- Agent runtime work goes in `apps/agent/`.
- Core API, persistence, health computation, and auth work goes in `apps/core/`.
- Main UI work goes in `apps/console/`.
- Deployment/service integration work goes in `deploy/`.
- Product history, release notes, and completed planning records go in `docs/milestones/`.
- Generated Core SPA assets live in `apps/core/web/`; edit `apps/console/` source and run `make build-static` instead of hand-editing `apps/core/web/`.
- OpenAPI source of truth is Core route annotations; generate `apps/core/openapi.yaml` with `make generate-openapi`.
- Frontend API client output is `apps/console/src/orion-sdk/index.ts`; regenerate it with `make generate-sdk` instead of hand-editing generated API code.

## Naming Rules

- New files and folders must use lowercase kebab-case.
- Keep names descriptive and short enough to scan.
- Follow existing generated-file names when editing generated outputs.
- Do not rename existing files or folders just for style unless the task asks for it.

## App Code Size Rules

- App source files under `apps/` must stay at or below 500 lines.
- This rule applies to product and test code in `apps/agent/`, `apps/core/`, and `apps/console/`.
- This rule does not apply to docs, config files, generated SDK/OpenAPI/Swagger output, built web assets, public assets, or database migrations.
- Before adding a new oversized file or expanding an existing oversized file, split it by responsibility instead.
- Run `make code-line-limit` before handing off app source changes.
- Use `docs/plans/app-code-breakdown.md` as the migration map for existing oversized app files.

## Contract Rules

- Core route or response changes must update route annotations and regenerate `apps/core/openapi.yaml`.
- Agent/Core behavior changes must update `docs/agent-core-contract.md` when the wire contract or responsibility split changes.
- UI changes that rely on new API fields must land with matching Core/OpenAPI updates.
- Deployment path changes must update `README.md`, `Makefile`, and relevant files under `deploy/`.

## Console Table Rules

- Use the OpenStatus data-table pattern for log-like tables in `apps/console/`.
- Log-like tables include Agent report logs, monitor check logs, incident timelines/events, Orion event logs, alert delivery logs, and any future operational history view.
- Prefer schema-driven data-table components, TanStack Table, TanStack Query, and `nuqs` URL-backed state for filters, sorting, and pagination.
- Do not hand-roll one-off log tables unless the OpenStatus data-table components cannot support the required interaction.
- Keep server-side pagination and filtering wired to Core API query params when the API supports them.

## Coordination Rules for Multiple Agents

- Work in disjoint areas when possible:
  - Agent worker: `apps/agent/`
  - Core worker: `apps/core/`
  - Console worker: `apps/console/`
  - Deploy/docs worker: `deploy/`, `docs/`, root docs
- Do not overwrite unrelated edits. If a file has changes you did not make, read it and adapt.
- Avoid broad refactors during feature or bug work unless the task explicitly asks for them.
- Prefer small, contract-aware changes over cross-cutting rewrites.
- Keep generated files separate from source files in summaries and reviews.

## Common Commands

Run Core tests:

```sh
cd apps/core && go test ./...
```

Run Agent tests:

```sh
cd apps/agent && go test ./...
```

Install and build the main console app:

```sh
cd apps/console && npm install && npm run build
```

Regenerate the console API client:

```sh
make generate-sdk
```

Regenerate the OpenAPI contract:

```sh
make generate-openapi
```

Build static web assets into Core:

```sh
make build-static
```

Build the Core Docker image:

```sh
make docker-build
```

Run Core with Docker Compose:

```sh
make docker-up
```

## Commit Message Format

Commit your completed changes before ending the task unless the user explicitly asks you not to
commit, the work is intentionally left incomplete, or the repository is in a state where committing
would capture unrelated or unsafe changes. If you cannot commit, explain why in the final response.

Use this format for commits:

```txt
conventional-commit-type(service/package changed): one liner

-key point if any exists;
-another key point if any exists;
```

- The first line must use a conventional commit type and a scope for the service or package changed.
- Use a concise one-line summary, for example `docs(repo): clarify agent instructions`.
- Add bullet points only when the one-liner does not cover the change.
- When bullets are needed, add one blank line after the subject.
- Bullet points must start with `-` immediately followed by text, with no space after the dash.
- End each bullet with `;`.
- Do not put blank lines between bullet points.

## Review Checklist

- Does the change stay inside the right app/package/deploy/doc boundary?
- If an API changed, were OpenAPI and generated clients handled?
- If Agent/Core behavior changed, was the contract doc updated?
- If paths changed, were README, Makefile, Docker, and deploy docs updated?
- Are generated files clearly identified?
- Were relevant tests or build commands run, or is the reason they were not run stated clearly?
