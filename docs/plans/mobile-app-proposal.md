# Orion Mobile App Proposal

Status: proposal, input for design. Nothing here is built.

## Goal

A free app on the App Store and Google Play that any Orion operator installs, points at their own
Core, and uses to **know when something breaks and act on it from their pocket**. It is not a
second Console. Console stays the place for configuration; mobile is for awareness and response.

Principles:

1. **Self-hosted first.** The app talks only to the user's Core. No Orion account, no Sunday Studio
   cloud holding their data.
2. **Alert → look → act in under 30 seconds.** Everything else is secondary.
3. **Read-heavy, few safe writes.** Heavy configuration (monitor editing, status page building,
   alert routing, data lifecycle) stays on web.
4. **One app, many Cores.** Home lab and work Core side by side.

## Who uses it

- Solo operator / home lab owner: wants a push when the box dies and a glance at what is red.
- Small-team on-call: wants to acknowledge, hand off, and leave a note without opening a laptop.

## Feature scope

### v1 (must ship)

| Area | What the app does | Core API it uses today |
|---|---|---|
| Connect | Add a Core by scanning a QR code from Console or typing a URL; sign in; store credentials in Keychain/Keystore; multiple Cores with a switcher | `/health`, `/v1/auth/login`, `/v1/auth/me` |
| Overview | One screen: overall health, open incidents count, servers up/down, monitors failing, last-updated. Pull to refresh | `/v1/overview`, `/v1/health/summary`, `/v1/health/issues` |
| Incidents | List (open first, filter by status/severity), detail with timeline, notes, linked logs. **Actions: acknowledge, resolve, reopen, add note, set responder** | `/v1/incidents*` |
| Push notifications | Incident opened / escalated / resolved, server went silent, per-Core mute and quiet hours. Tap opens the incident | **New**, see "Push" |
| Servers | List with status, CPU/mem/disk summary, last report age; detail with current metrics, monitors on it, recent reports | `/v1/agents*` |
| Monitors | List with status and uptime; detail with response-time chart, history, uptime. **Actions: pause, resume, run test now** | `/v1/monitors*` |
| Search | Global search across servers, monitors, incidents | `/v1/search` |
| Settings | Manage Cores, notification prefs, biometric lock, theme, sign out, "forget this Core" | local |

### v1.1

- **Logs:** live tail (SSE), search, facets, histogram, open a log's related lines. Mobile-adapted:
  big text, filter chips, tap a line for fields. Uses `/v1/logs*`, `/v1/logs/stream`.
- **Alert deliveries:** see what was sent to where, retry a failed one (`/v1/alerts/deliveries*`).
- **Incident cover / handoff:** "I'm taking this" and "cover for me" (`/v1/incidents/{id}/cover`).
- **Public status update:** post an update to the status page from an incident
  (`/v1/incidents/{id}/public-update`). Needs a confirm step; this is outward-facing.
- **Home-screen widgets and lock-screen Live Activity** for an open incident (iOS), Android widget.

### Later / out of scope for mobile

- Creating or editing monitors, alert rules/routes/channels, status pages, data lifecycle, API
  tokens, registration tokens. These stay in Console.
- Installing the Orion Server on a host.
- Multi-user admin and audit log browsing (read-only audit could come later).

## Navigation (for design)

Bottom tabs: **Home · Incidents · Servers · Monitors · More**.

- Home = Overview. Header shows the active Core name with a switcher.
- Logs, Search, Alert deliveries, Settings live under **More** (Search also as a header icon).
- Deep links from push notifications go straight to incident detail.
- Incident detail has a sticky action bar: Acknowledge / Resolve / Add note.

Key design states to cover: first-run (no Core), connect/QR flow, Core unreachable (show cached data
with a clear "offline, last synced 4m ago" banner), session expired, empty states (all healthy is the
happy path and should feel calm), destructive confirm (resolve, public update), biometric lock.

## How to build it

### Stack

**React Native with Expo (TypeScript).**

- The Console is React/TS and `packages/sdk` already generates TS types from the OpenAPI spec, so the
  API client, types, and query logic are shared rather than rewritten. A separate Swift + Kotlin
  pair would double the work for a small team.
- TanStack Query for data and cache (same as Console), Expo Router for navigation, Expo
  SecureStore for credentials, Expo Notifications for push, `react-native-svg`/Skia for charts.
- Lives in a new `apps/mobile/` (kebab-case, 500-line file limit applies). Imports the generated
  client; regenerate with `make generate-sdk` pointed at a second output.

### Connecting and auth

Today Core offers a JWT from `/v1/auth/login` and scoped API tokens from `/v1/settings/api-tokens`.
API tokens are the wrong fit for a phone because they cannot be listed per-device and "not available
to API tokens" blocks the settings surface. Proposal:

1. **Pairing QR in Console** (Settings → Mobile): shows `{core_url, one-time pairing code}`. The app
   scans it, exchanges the code for a **per-device token** (new `POST /v1/devices/pair`).
2. Per-device tokens are listed and **revocable in Console** ("Pixel 8, last seen 2h ago"), so a
   lost phone is a one-click fix. Scope: read + incident/monitor actions, never settings.
3. Username/password login remains as a fallback for people who skip the QR.
4. Tokens live in Keychain/Keystore; optional biometric gate on app open.

### Reaching a self-hosted Core

Core is often on a LAN or behind a reverse proxy. The app must work with:

- A public HTTPS URL (the normal case, behind Caddy/Traefik/nginx).
- A private network via the user's own VPN (Tailscale/WireGuard), where the phone is already on it.
- Plain `http://` on a LAN is allowed only after an explicit warning, with iOS ATS and Android
  cleartext exceptions scoped to user-added hosts.

Core needs CORS only for web; native apps do not need it. Add an `X-Orion-Client: mobile/<version>`
header for audit and support.

### Push notifications (the hard part)

APNs and FCM require credentials owned by whoever publishes the app, so a self-hosted Core cannot
send to phones directly. Alert channels today are webhook-only. Proposal:

1. **Orion Push Relay**: a tiny stateless service run by Sunday Studio (open source, so people can
   self-host it and build their own app variant). It holds the APNs/FCM keys and does one thing:
   forward a wake-up to a device token.
2. **Privacy by design:** the relay receives only the device push token and an opaque
   `{core_id, incident_id}`. Payload text is a generic "Orion: incident opened". The app wakes,
   fetches the real details from the user's own Core, then shows a local notification (the Matrix
   and Bitwarden pattern). Incident names, hostnames, and log lines never touch Sunday Studio.
3. **Core side:** add a `mobile_push` alert channel type alongside `webhook`, reusing the existing
   rules, routes, label routes, quiet-hours and delivery log. Delivery failures show up in
   `/v1/alerts/deliveries` like any other channel. Relay URL is configurable, default to the hosted
   one, with an off switch for people who want zero third-party calls.
4. **Device registration:** the app registers its push token with Core on pair
   (`POST /v1/devices/{id}/push-token`), and re-registers when the OS rotates it.
5. Critical alerts: iOS Critical Alerts entitlement (apply early, Apple approval takes weeks) and an
   Android high-importance channel, so a down production box can break through Do Not Disturb.
   Opt-in per Core.

### Offline and freshness

- Cache the last Overview, incident list, server list in TanStack Query persistence (MMKV/SQLite) so
  the app opens instantly and shows stale-with-banner when Core is unreachable.
- Foreground: poll Overview every 30s and use SSE for open incident detail and logs. No background
  polling; push covers background.

### Core changes required (summary)

| Change | Why |
|---|---|
| `mobile_push` alert channel type | push delivery through existing alert rules |
| `/v1/devices/*` pair, list, revoke, push-token | per-device auth and revocation |
| Pairing UI in Console Settings → Mobile | QR generation, device list |
| Device-token scope on incident/monitor actions | write access without settings access |
| Update `openapi.yaml`, `docs/agent-core-contract.md` where affected | contract rules in AGENTS.md |

### Phasing

1. **Foundation:** `apps/mobile` skeleton, shared SDK, connect/auth with login fallback, Overview.
2. **Read surfaces:** incidents, servers, monitors, search. No push yet; usable by pull.
3. **Pairing and push:** device API in Core, Console pairing UI, push relay, `mobile_push` channel.
4. **Actions:** acknowledge/resolve/note, pause/resume monitor, test run.
5. **Polish and release:** biometric lock, offline cache, multi-Core, App Store / Play review,
   Critical Alerts entitlement.
6. **v1.1:** logs live tail, alert deliveries, widgets, Live Activity.

### Risks and open questions

- **App review:** both stores accept "connect to your own server" apps (Home Assistant, Bitwarden,
  Plex precedent). Provide a public demo Core for reviewers.
- **Relay trust and cost:** hosted relay is a Sunday Studio operational dependency. Needs rate limits
  and a documented self-host path.
- **Cleartext LAN servers** are common in home labs; the UX must be honest without being hostile.
- **Console vs mobile parity drift:** keep the mobile scope table above as the contract, and add
  features to mobile only when they serve alert-to-action.
- Decision needed: ship as one free app only, or also a self-buildable variant for people who will
  not use the hosted relay.
