# First-Release Status Page Cardinality

## Status

Accepted for first release.

## Context

`docs/architecture/status-pages.md` defines status pages as a public publication layer over Core data, but earlier planning left open whether the first release ships with one default page or multiple independently managed pages. The implemented Core and Console shape already supports multiple pages: Core exposes plural admin routes, stores pages in `status_pages`, enforces slug uniqueness, and the Console exposes a page list plus create flow.

## Decision

The first release supports multiple status pages per Orion Core instance.

Each page can contain multiple sections and components, and those components can map to any supported internal servers or monitors. Multiple top-level pages are in scope for the first release. Specialized status page hierarchy, such as customer-specific sub-pages, regional sub-pages, or environment sub-pages inside a parent page, remains deferred.

The implementation keeps the plural status page data model and route vocabulary from the architecture plan:

- `status_pages` remains a table, not a singleton settings row.
- Child tables keep `status_page_id`.
- Admin API routes remain under `/v1/status-pages`.
- Public routes keep slug-based addressing.

Core does not enforce a one-page-per-instance limit. It enforces unique slugs and custom-domain ownership so each public page has an unambiguous public address.

## Slug Behavior

Every page has a slug because the slug is the canonical public address.

Slug rules:

- Store a required slug on `status_pages`.
- Generate the initial slug from the page title, with a fallback such as `main`.
- Accept only lowercase URL-safe slugs, preferably kebab-case.
- Keep slugs unique in storage across active pages.
- Allow editing the slug while the page is a draft.
- Treat the slug as stable after first publish. A post-publish slug change should be deferred unless redirect support is implemented.

Public routing:

- `GET /status/:slug` is the canonical public page route.
- `GET /status/:slug?format=json` is the canonical public JSON payload route.
- `GET /status` is not a canonical public page route until a default-page setting exists.
- Unknown slugs return `404`.

## Console Navigation

Console exposes a navigation entry named `Status Pages` for the first release.

Behavior:

- If no page exists, the entry opens an empty state and create-draft flow.
- If pages exist, the entry opens a list and lets the administrator select the page to edit.
- The editor can still show the public URL, draft or published state, validation, preview, components, incidents, and settings.
- The first release may expose page creation and page selection, but should not expose sub-pages, audience segmentation, or automatic page duplication until those concepts have product rules.

This matches the current Console implementation and avoids a misleading singleton UI over a multi-page API.

## API Shape

Use plural admin routes and plural behavior:

- `GET /v1/status-pages` returns an array of pages.
- `POST /v1/status-pages` creates a draft page.
- `POST /v1/status-pages` returns `409 Conflict` when the requested slug already exists.
- `GET /v1/status-pages/:id`, `PUT /v1/status-pages/:id`, `POST /v1/status-pages/:id/publish`, and `GET /v1/status-pages/:id/preview` operate on one page by id.
- Section, component, incident, and update routes remain nested under `/v1/status-pages/:id`.

Public routes should remain slug-based:

- `GET /status/:slug`
- `GET /status/:slug?format=json`
- `GET /status/:slug/history`
- `GET /status/:slug/incidents`
- `GET /status/:slug/incidents/:incident_id`

Do not add a singleton-only public DTO or singleton-only admin route.

## Migration Path

No migration is needed to enable multiple top-level pages because the first release already uses real page rows with stable page ids and slug public routes.

Future hierarchy or default-page work should be additive:

1. Add an explicit default page setting if `/status` should redirect somewhere.
2. Keep existing page ids, slugs, and child rows unchanged.
3. Keep `GET /status/:slug` behavior unchanged.
4. Add parent/sub-page or audience-targeting tables only after their product rules are defined.
5. Keep slug uniqueness and custom-domain ownership constraints in place.

## Consequences

Benefits:

- Matches shipped Core and Console behavior.
- Supports separate public pages for different products, environments, or brands without waiting for another schema migration.
- Keeps unresolved customer page, regional page, and sub-page questions out of the first release.
- Preserves the planned plural schema and API shape without artificial application limits.
- Keeps public URLs slug-based from the start.

Tradeoffs:

- Administrators can create more public pages than they may actually need if Console guidance is weak.
- Operators must understand that pages are independent top-level publications, not nested sub-pages or customer-specific views.
- Slug redirect behavior after publish is deferred.
