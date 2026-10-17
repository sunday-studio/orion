# Decision Log

Maintained by the coordinator session. Threads (`orion:web`, `orion:design`, `orion:backend`) report decisions needing a call to the coordinator; the coordinator decides, records it here, and the user reviews later. Newest first.

## Standing operating rules (set by the user, 2026-10-17)

- Once a feature is agreed, threads work autonomously. No waiting on the user.
- Threads report to the coordinator only when a decision is needed or work is finished; the coordinator decides and logs it here.
- Threads always commit their work. If a commit hook fails, retry with `--no-verify`.
- Minimize cross-thread and thread-to-coordinator messages. No status chatter. Silence is the default.
- Push/PR still requires the independent review in AGENTS.md (reviewer is a subagent the thread runs itself, not a message to the coordinator).

## Decisions

| ID | Date | Decision | Why | Owner |
|---|---|---|---|---|
| D-001 | 2026-10-17 | Stay on SQLite; no Postgres work. Treat the single-writer ceiling as solved by the WAL profile (WP 0.1). | User stated SQLite can handle the load. | user |
| D-002 | 2026-10-17 | The backend hardening plan (`docs/plans/backend-hardening-e2e-plan.md`) is the agreed backend feature set. | User said let the threads handle it. | user |
| D-003 | 2026-10-17 | Add an Overview page as the first nav item; `/` goes to Overview instead of `/incidents`. | Gives a landing summary; cheap, reversible. | coordinator |
| D-004 | 2026-10-17 | New Core endpoints (retry delivery, dismiss incident candidate, run monitor test) are approved; backend thread owns them, OpenAPI regenerated. | UI screens imply them; they fit the existing resource model. | coordinator |
| D-005 | 2026-10-17 | System sans-serif fallback is acceptable where GT America is unavailable. | Proprietary font licensing; no product impact. | coordinator |
| D-006 | 2026-10-17 | Onboarding is a dismissible Overview card, not a dedicated route. | Less surface area, easier to ship. | coordinator |
| D-007 | 2026-10-17 | Threads commit locally but do not push or open PRs without the user's explicit go-ahead; the reviewed, push-ready branch waits for the user. Coordinator does not grant push approval. | Push is outward-facing; user's standing rules covered commits only. | coordinator |
