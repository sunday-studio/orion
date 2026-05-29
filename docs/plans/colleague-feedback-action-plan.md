# Colleague Feedback Action Plan

Date: 2026-05-29

This plan turns outside review feedback into concrete Orion follow-up work. The goal is to make the
project easier to evaluate quickly, clearer about its architecture and deployment model, and cleaner
before the public surface gets larger.

## Feedback Themes

- First-run examples should show Orion monitoring something real, not only describe the product.
- The README should make quality signals visible, especially tests and coverage.
- The architecture story should explain which parts are required, which parts are replaceable, and
  how Core, Console, and monitored servers are distributed.
- Deployment docs should explain why Docker Compose is the primary path and when Kubernetes is worth
  supporting.
- The project should state why someone would choose Orion instead of an external uptime SaaS or a
  heavier observability stack.
- Repository hygiene needs a pass for committed `.DS_Store` files and public planning material.
- Product terminology should avoid confusing modern "server" expectations where possible.

## Priorities

### P0: Public Evaluation Basics

These items help a new evaluator understand the project in the first 10 minutes.

| Action | Area | Acceptance |
| --- | --- | --- |
| Add a small Docker Compose example with a basic Python service | `examples/`, `README.md` | A user can run the example, start Orion Core, install or configure a monitored server process, and see a monitor/report in Console. |
| Add README badges for CI and test coverage | `README.md`, CI provider | README shows current build/test status and coverage when coverage publishing exists. |
| Add a concise "Why Orion?" README section | `README.md` | README explains self-hosted ownership, server-local context, simple SQLite/Core deployment, and the difference from external uptime checks. |
| Remove committed `.DS_Store` files and keep ignoring them | repo root, `.gitignore` | `git ls-files '*DS_Store'` returns no tracked files, and new macOS metadata remains ignored. |

### P1: Architecture and Deployment Clarity

These items answer the deeper "how would I operate this?" questions.

| Action | Area | Acceptance |
| --- | --- | --- |
| Document supported headless and alternative UI posture | `README.md`, `docs/architecture/system-overview.md` | Docs say the web Console is the supported UI today, Core is the API boundary, and a TUI or alternative UI is possible later but not a first-class supported mode yet. |
| Clarify component distribution | `README.md`, `docs/architecture/system-overview.md` | Docs show Core API, Core monitor worker, SQLite, Console, and monitored server processes, including which processes share a host or volume. |
| Explain why Core-in-Docker is useful | `docs/deployment/core-docker.md`, `README.md` | Docs position Docker Compose as the simplest homelab/self-hosted path and explain that Core should run somewhere reliably reachable by monitored servers, ideally outside the network being monitored when possible. |
| Add a deployment decision note for Kubernetes | `docs/deployment/` | Docs state Kubernetes is not the first target, list scenarios where it could make sense, and define what a future minikube example should prove. |

### P2: Product Language and Planning Hygiene

These items reduce future confusion and protect project history as Orion matures.

| Action | Area | Acceptance |
| --- | --- | --- |
| Record the Server terminology decision | `docs/architecture/`, `README.md`, Console copy | A decision record names the term, migration scope, and confirms binary/config names stay as `orion-agent` for compatibility. |
| Audit public planning and agent instruction files | `AGENTS.md`, `docs/plans/`, Maat storage | Private strategy, credentials, sensitive operational notes, and non-public planning details are either removed from the repo or intentionally documented as public. |
| Add Maat support for publishable plans | Maat project | Orion can keep private working memory while exporting curated public plans or milestones when useful. |

## Example Scope

The first example should stay intentionally small:

- `python-app`: a tiny Python process or HTTP service that can be made healthy/unhealthy on demand;
- `docker-compose.yml`: runs the Python service and Orion Core locally, or documents how to point a
  local Orion Server at the service;
- `README.md`: includes setup, expected result, reset instructions, and screenshots only if they are
  stable enough to maintain;
- no Kubernetes, cloud provider, or alert-provider setup in the first example.

The example should prove the value proposition: Orion sees useful information from the monitored
host or service, not just whether a public URL responds.

## Kubernetes Position

Kubernetes support should wait until the Docker Compose path is polished. A minikube example is
worth adding only if it proves something Compose does not:

- Core running with persistent storage in a cluster;
- Core monitor worker separated from the API;
- monitored workloads sending data to Core;
- clear operational notes for secrets, ingress, storage, and upgrades.

Until then, describe Kubernetes as plausible but not the recommended first install path.

## Terminology Decision

The current "Server" term is technically accurate, but it now carries AI-product expectations. The
candidate public terms are:

- `Server`: clearest for homelab and infrastructure users, but less accurate for laptops or worker
  machines;
- `Node`: common infrastructure term, but a little more Kubernetes-flavored;
- `Monitor host`: accurate but too clunky for primary UI copy.

Recommended direction: use `Server` in product-facing copy while keeping `orion-agent`, config
paths, and wire concepts stable until a compatibility-safe migration plan exists.

## Backlog Shape

Create follow-up tickets from this plan in this order:

1. Add README "Why Orion?" and architecture clarifications.
2. Add Docker Compose Python example.
3. Add CI and coverage badges.
4. Remove tracked `.DS_Store` files.
5. Draft Kubernetes/minikube decision note.
6. Record Server terminology compatibility boundaries.
7. Audit public planning and AGENTS material.

Each ticket should include an explicit verification command or review check so the work does not
become vague documentation polish.
