# First-Release Status Page Cardinality

## Status

Accepted for first release.

## Context

`docs/architecture/status-pages.md` defines status pages as a public publication layer over Core data, but leaves open whether the first release ships with one default page or multiple independently managed pages. The first release needs enough structure to support components, incidents, public payloads, and future subscriptions without forcing administrators to understand sub-pages, customer pages, or segmented audience routing on day one.

## Decision

The first release supports one status page per Orion Core instance.

This page is the instance's default public status page. It can contain multiple sections and components, and those components can map to any supported internal servers or monitors. Multiple public pages, customer-specific pages, environment pages, and regional sub-pages are deferred.

The implementation should keep the plural status page data model and route vocabulary from the architecture plan:

- `status_pages` remains a table, not a singleton settings row.
- Child tables keep `status_page_id`.
- Admin API routes remain under `/v1/status-pages`.
- Public routes keep slug-based addressing.

For the first release, Core enforces cardinality with application validation: at most one non-deleted status page can exist.

## Slug Behavior

The singleton page still has a slug because the slug is the stable public address and the migration path to multiple pages.

Slug rules:

- Store a required slug on `status_pages`.
- Generate the initial slug from the page title, with a fallback such as `main`.
- Accept only lowercase URL-safe slugs, preferably kebab-case.
- Keep slugs unique in storage even while only one page is allowed.
- Allow editing the slug while the page is a draft.
- Treat the slug as stable after first publish. A post-publish slug change should be deferred unless redirect support is implemented.

Public routing:

- `GET /status/:slug` is the canonical public page route.
- `GET /status/:slug/payload` is the canonical public JSON payload route.
- `GET /status` may redirect to the singleton published page, or return the singleton public page, but public clients should be told to use the slug URL.
- Unknown slugs return `404`, even if a singleton page exists.

## Console Navigation

Console should expose a single navigation entry named `Status page` for the first release.

Behavior:

- If no page exists, the entry opens the create-draft flow.
- If a page exists, the entry opens that page's editor directly.
- The editor can still show the public URL, draft or published state, validation, preview, components, incidents, and settings.
- Do not expose a page index, page switcher, duplicate action, or `New status page` command in the first release.

When multiple pages are later enabled, the same navigation entry can be renamed to `Status pages` and open a list view. The existing singleton page becomes the first row in that list.

## API Shape

Use plural admin routes, but enforce singleton cardinality:

- `GET /v1/status-pages` returns an array with zero or one page.
- `POST /v1/status-pages` creates the singleton page when none exists.
- `POST /v1/status-pages` returns `409 Conflict` with a stable error code such as `status_page_limit_reached` when a page already exists.
- `GET /v1/status-pages/:id`, `PUT /v1/status-pages/:id`, `POST /v1/status-pages/:id/publish`, and `GET /v1/status-pages/:id/preview` operate on the singleton by id.
- Section, component, incident, and update routes remain nested under `/v1/status-pages/:id`.

Public routes should remain slug-based:

- `GET /status/:slug`
- `GET /status/:slug/payload`
- `GET /status/:slug/history`
- `GET /status/:slug/incidents`
- `GET /status/:slug/incidents/:incident_id`

Do not add a special singleton-only public DTO or singleton-only admin route that would need to be removed for multiple pages.

## Migration Path

When multiple pages are enabled later:

1. Remove the application-level singleton creation guard.
2. Add Console list and create actions under the existing `Status pages` area.
3. Keep existing page ids and slugs unchanged.
4. Keep child rows attached through their existing `status_page_id` values.
5. Keep `GET /status/:slug` behavior unchanged.
6. Reinterpret `GET /status` as either a redirect to a configured default page or a not-found/help response if no default exists.
7. Add unique constraints needed for multiple pages, such as slug uniqueness across all active pages and custom-domain ownership when custom domains exist.

This path avoids a data migration from singleton settings into page tables. The first page is already a real page; the future work is mostly lifting the creation limit and adding Console navigation for page selection.

## Consequences

Benefits:

- Simplifies first-release Console UX.
- Avoids unresolved product questions about customer pages, regional pages, and sub-pages.
- Preserves the planned plural schema and API shape.
- Keeps public URLs slug-based from the start.

Tradeoffs:

- Administrators cannot publish separate audience-specific pages in the first release.
- The admin API exposes plural resource names while returning at most one page.
- Slug redirect behavior after publish is deferred.
