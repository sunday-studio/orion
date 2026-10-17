# Dead Code Audit

Status: audit complete, 2026-10-18. Owner: `orion:cleanup`. Maat ticket T-20261018-002550-515c.

Method: `go vet`, `staticcheck` (U1000/SA/S1), `deadcode -test`, `go mod tidy` on Core; `knip` on the
Console (with the generated SDK present); tracked-but-gitignored check; markdown link and orphan
scan; repo-wide grep (Go, TS, Makefile, workflows, Docker, scripts, docs) for every candidate.

Scope notes:

- `apps/agent/` is **deferred**. The `orion:agent` thread still has unmerged commits
  (`origin/main..claude/trusting-kowalevski-a0ef06`), so nothing there is audited or touched yet.
- Generated output (`apps/core/docs`, `apps/core/openapi.yaml`, `apps/console/src/orion-sdk`) is
  never hand-edited; `docs/docs.go` `init#1` flagged by `deadcode` is swag output and stays.
- Unused exported types/consts/vars in Go are not covered by `deadcode`/`staticcheck`; not audited.

## Delete (high confidence)

| Path or symbol | Evidence of non-use | Confidence | Action | Risk |
|---|---|---|---|---|
| `apps/core/web/assets/index-COhy5PCK.js`, `index-ScSJFViZ.css` | Tracked although `apps/core/web/assets/` is gitignored; `index.html` is not tracked, so they are an orphaned stale build; `make build-static` wipes and regenerates `apps/core/web`; nothing references the hashed names | High | delete (untrack) | Low |
| `deploy/examples/core-console-compose` (symlink) | Symlink alias to `core-console-compose.yaml` (git mode 120000); CI path filter and docs use the `.yaml`; one doc line uses the extensionless name | High | delete, point doc at `.yaml` | Low |
| `internal/service` `core-monitor-validation-service.go`: `validateCoreManagedMonitorConfig`, `ValidateCoreManagedMonitorConfig` and 20 `validateCore*MonitorConfig`/`*WithPolicy` wrappers | `deadcode` + `staticcheck U1000`: unreachable, only referenced by each other; the live path is `validateCoreManagedMonitorConfigWithPolicy` | High | delete wrappers (keep ones still referenced) | Low |
| `internal/api` `Server.alertDryRunEventFromRequest` | U1000 + deadcode, no callers | High | delete | Low |
| `internal/api` `startAPITestSMTPServer` (test helper) | U1000 + deadcode, no callers | High | delete | Low |
| `internal/api` `Server.ProductLogStore` | deadcode, no callers (comment claims retention use; retention is started elsewhere) | High | delete | Low |
| `internal/service` `AgentService.GetAgentCount` | deadcode, no callers; only a coverage inventory doc lists it | High | delete, update inventory row | Low |
| `internal/testkit` `SMTPServer.Addr`, `Webhook.RedirectTo` | deadcode, no callers | High | delete | Low |
| `internal/utils` `IsValidID` | deadcode, no callers | High | delete (remove file if empty) | Low |
| `internal/worker` `defaultTLSCheck` | U1000 + deadcode, no callers | High | delete | Low |
| `apps/console/src/components/shared/placeholder-page.tsx` | knip unused file; only a name match in its own file | High | delete | Low |
| `apps/console/src/features/agents/components/agent-filters.tsx` | knip unused file; only referenced from a commented-out JSX block in `agent-list.tsx` | High | delete file and the commented block | Low |
| `apps/console/src/hooks/use-debounce.ts` | knip unused file; no importers | High | delete | Low |
| `apps/console/src/assets/react.svg` | Vite template leftover; no references in `src`, `index.html`, CSS | High | delete | Low |
| `ServerDetailPage` alias in `agent-detail.view.tsx` | knip duplicate export; no importer uses the alias | High | delete alias | Low |

## Keep

| Path or symbol | Evidence | Action |
|---|---|---|
| `apps/core/docs/docs.go` `init#1` | swag-generated, produced by `make generate-openapi` | keep |
| `deploy/scripts/*.sh` | each referenced by Makefile, docs, or other scripts | keep |
| `apps/core/dockerfile.dev`, `dockerfile.prebuilt-web`, `docker-compose.dev.yml`, `docker-entrypoint.sh` | referenced by compose, `core-image.yml`, Makefile | keep |
| `.github/scripts/check-core-backend-tests.sh` | used by `ci.yml` | keep |
| `assets/*.png` | embedded in `README.md` | keep |
| `examples/python-sleep-compose/` | referenced by README and first-run docs | keep |
| `.agents/skills/golang-modernize`, `skills-lock.json` | agent skill lock, used by tooling | keep |
| `ORION_CROSS_*`, `ORION_LOGS_PERF_*` env vars | soak/perf test knobs, used by `make core-soak-*` | keep |
| `go.mod`/`go.sum` (Core) | `go mod tidy` produces no diff | keep |
| Console `package.json` dependencies | knip reports no unused dependencies or devDependencies | keep |
| Console design-system exports (`DialogClose`, `DrawerClose`, `PopoverClose`, `TooltipProvider`, `BreadcrumbEllipsis`, `toneForStatus`, `disabledStyle`, date format constants, `Select`/data-table types) | knip "unused exports", but they are the design-system public surface and `orion:web` is actively building screens on it | keep |
| Console feature helpers exported but used in-file (`actorLabel`, `parsePayload`, `normalizeKind`, ...) | knip flags only the `export` keyword; removing it is churn in files the web thread edits | keep |

## Uncertain (left in place)

| Path | Evidence | Why uncertain |
|---|---|---|
| `docs/plans/agent-health-debugging-next-release.md`, `colleague-feedback-action-plan.md`, `core-codebase-cleanup-proposal.md`, `incident-detail-information-architecture.md`, `status-page-public-design-breakdown.md`, `settings-pr-closeout-checklist.md` | No inbound links; dated May 2026; likely superseded | Historical decision records. AGENTS.md says completed plans belong in `docs/milestones/`; the owner should decide archive vs delete |
| `docs/development/maat-setup-friction.md`, `settings-readiness-closeout.md`, `docs/milestones/first-run-deployment-checklist-2026-05-27.md` | No inbound links | Evidence/history records, not code |
| `docs/ci.md`, `docs/deployment/upgrade-notes-backend-hardening.md`, `docs/web/design-system.md`, `docs/web/web-status-map.md` | No inbound links | Current reference docs that just lack an index entry; link them instead |
| `apps/agent/**` | Not audited | Blocked on `orion:agent` merge |
| Unused exported Go types/consts, struct fields, unused config keys in Core | Not covered by tools run | Needs a second pass |
