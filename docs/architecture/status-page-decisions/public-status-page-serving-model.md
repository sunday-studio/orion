# Public Status Page Serving Model

Status: accepted

Date: 2026-05-27

Ticket: T-20260527-112218-7a97

## Context

`docs/architecture/status-pages.md` defines status pages as a public projection over Core data and leaves one serving question open: whether public pages should be served from Core's main binary or as a separate static/public frontend bundle.

The first status page release needs a simple public page, sanitized public DTOs, admin-controlled publishing, and minimal deployment work for self-hosted installs.

## Decision

Serve public status pages from the Core main binary, using a dedicated public static bundle packaged with Core.

The public status page bundle is separate from the authenticated Console SPA. It may share frontend tooling and components, but it must have its own entry point and must only read public status page DTOs. Do not introduce a second web service, Node server, CDN dependency, or separate deployment artifact for the first release.

## Routing Implications

- Core registers explicit public status routes before the Console SPA fallback.
- `GET /status/:slug` serves the public status page HTML shell.
- Public read endpoints stay outside `/v1`, for example `GET /status/:slug/payload`, `GET /status/:slug/history`, `GET /status/:slug/incidents`, and `GET /status/:slug/incidents/:incident_id`.
- Console admin routes stay under `/v1/status-pages` and remain separate from public routes.
- The Console fallback must not claim `/status/*`; unknown status slugs should return public `404` responses, not the Console app.
- Future custom domains should resolve host plus path to the same Core public route handlers, with strict host and slug validation.

## Asset Build Path

- Public status page source belongs in `apps/console/` as a dedicated public entry point, not inside the admin Console route tree.
- The existing `make build-static` path remains the packaging path: build frontend assets from `apps/console`, copy the generated output into `apps/core/web/`, and serve it from Core.
- The public bundle should emit assets under a distinct generated subpath such as `apps/core/web/status/` or `apps/core/web/status-assets/` so it does not depend on the Console SPA's `index.html`.
- Do not hand-edit generated files under `apps/core/web/`; edit the source in `apps/console/` and rebuild.

## Authentication Boundary

- Public status routes are unauthenticated, read-only, and backed only by sanitized public DTOs.
- Public payloads must not expose internal server ids, monitor ids, alert channels, secrets, raw report payloads, internal incident notes, or Console diagnostics.
- Admin creation, editing, preview, publish, unpublish, and incident update workflows remain behind `/v1/status-pages` and the existing frontend auth boundary when auth is configured.
- Public preview from the Console should use an authenticated admin endpoint or a short-lived preview token; draft content is never exposed through normal public slug routes.

## Deployment Impact

- No new runtime process, port, reverse proxy upstream, database, or background worker is required for the first serving model.
- Existing Core Docker and self-hosted deployments continue to serve one HTTP application. Operators only need to expose `/status/*` on the same Core origin.
- Cache headers, public route rate limits, and custom-domain host routing can be added inside Core without changing the Server/Core contract.
- Later CDN or separate static hosting remains possible, but it would be an optimization over the same public DTO boundary rather than a first-release requirement.

## Consequences

This model keeps deployment simple and keeps the public/private boundary in Core, where the sanitizer and publication rules already live. The main implementation cost is splitting the public status page assets from the Console SPA so public routes do not load authenticated Console code or rely on Console navigation.
