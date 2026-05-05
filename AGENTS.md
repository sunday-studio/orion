# AGENTS.md

This file defines the working boundaries for humans and coding agents in this repository. Follow these rules before editing files.

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
- Product plans and milestone records go in `docs/plans/` and `docs/milestones/`.
- Generated Core SPA assets live in `apps/core/web/`; edit `apps/console/` source and run `make build-static` instead of hand-editing `apps/core/web/`.
- OpenAPI source of truth is `apps/core/openapi.yaml`.
- Frontend API client output is `apps/console/src/lib/api.ts`; regenerate it from `apps/console` instead of hand-editing generated API code.

## Contract Rules

- Core route or response changes must update `apps/core/openapi.yaml`.
- Agent/Core behavior changes must update `docs/agent-core-contract.md` when the wire contract or responsibility split changes.
- UI changes that rely on new API fields must land with matching Core/OpenAPI updates.
- Deployment path changes must update `README.md`, `Makefile`, and relevant files under `deploy/`.

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

Use this format for commits:

```txt
conventional-commit(scope): one-line summary

- key point if any exists;
- another key point if any exists;
```

- The first line must be a conventional commit style subject with a scope, for example `chore(repo): group project directories`.
- Keep the subject concise and imperative.
- Add one blank line after the subject only when key points exist.
- Key points are optional; when present, format each as a bullet and end each point with `;`.
- Do not put blank lines between key point bullets.

## Review Checklist

- Does the change stay inside the right app/package/deploy/doc boundary?
- If an API changed, were OpenAPI and generated clients handled?
- If Agent/Core behavior changed, was the contract doc updated?
- If paths changed, were README, Makefile, Docker, and deploy docs updated?
- Are generated files clearly identified?
- Were relevant tests or build commands run, or is the reason they were not run stated clearly?
