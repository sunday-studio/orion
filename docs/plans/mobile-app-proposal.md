# Orion Mobile App Proposal

Status: proposal, input for design. Nothing here is built.

## Goal

Mobile is a **new notification channel** in Orion, next to **email**, **webhooks** and the Console
**inbox** (incident triage), all of which stay. It plugs into the same alert rules, routes and label routes, shows up in the same
delivery log, and exists so an operator is reached wherever they are and can respond from the
notification. It is not a second Console. Console stays the place for configuration.

Principles:

1. **Notification first.** Every screen exists to explain, or act on, a notification that arrived.
2. **Same pipeline as everything else.** No mobile-only alerting logic: rules, routes, quiet hours and
   delivery logs are shared with email, webhook and inbox.
3. **Self-hosted first.** The app talks only to the user's Core. No Orion account.
4. **Read-heavy, few safe writes.** Configuration stays on web.
5. **One app, many Cores.**

## Who uses it

- Solo operator / home lab owner: wants a push when the box dies and a glance at what is red.
- Small-team on-call: wants to acknowledge, hand off, and leave a note without opening a laptop.

## Feature scope (v1, includes what was v1.1)

| Area | What the app does | Core API |
|---|---|---|
| Connect | Pair a Core by QR from Console or URL; sign in; credentials in Keychain/Keystore; multi-Core switcher | `/health`, `/v1/auth/*`, new `/v1/devices/*` |
| Push notifications | Incident opened / escalated / resolved, server silent, monitor failing; per-Core mute, quiet hours, optional Critical Alerts. Tap opens the incident | new `mobile_push` channel |
| Notification feed (mobile inbox) | Chronological list of what this device was notified about, with read state and the incident status now. The in-app home screen | `/v1/alerts/deliveries`, `/v1/incidents` |
| Incident detail | Timeline, notes, linked logs. Actions: acknowledge, resolve, reopen, add note, set responder, take/cover handoff, post public status update (confirm step) | `/v1/incidents/*` |
| Overview | Health summary, open incidents, servers down, monitors failing | `/v1/overview`, `/v1/health/*` |
| Servers | List and detail with current metrics and recent reports | `/v1/agents*` |
| Monitors | List, detail, response-time chart, history. Actions: pause, resume, test now | `/v1/monitors*` |
| Logs | Live tail (SSE), search, facets, histogram, related lines | `/v1/logs*` |
| Alert deliveries | See what was sent where, including this device; retry a failed delivery | `/v1/alerts/deliveries*` |
| Search | Global search | `/v1/search` |
| Widgets | Home-screen widget and iOS Live Activity for an open incident; Android widget | local + push |
| Settings | Cores, notification prefs, biometric lock, theme, forget Core | local |

Out of scope: creating/editing monitors, alert rules/routes/channels, status pages, data lifecycle,
API/registration tokens, installing the Orion Server, user admin.

## Navigation (for design)

Bottom tabs: **Notifications · Incidents · Servers · Monitors · More**.

- Notifications (the feed) is the home screen; Overview is a summary card at its top. Header shows the active Core name with a switcher.
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
send to phones directly. Alert channels today are email and webhook. Proposal:

1. **Orion Push Relay**: a tiny stateless service run by Sunday Studio (open source, so people can
   self-host it and build their own app variant). It holds the APNs/FCM keys and does one thing:
   forward a wake-up to a device token.
2. **Privacy by design:** the relay receives only the device push token and an opaque
   `{core_id, incident_id}`. Payload text is a generic "Orion: incident opened". The app wakes,
   fetches the real details from the user's own Core, then shows a local notification (the Matrix
   and Bitwarden pattern). Incident names, hostnames, and log lines never touch Sunday Studio.
3. **Core side:** add a `mobile_push` alert channel type alongside `email` and `webhook` (and the Console inbox),
   reusing the existing rules, routes, label routes, quiet-hours and delivery log. A channel
   targets paired devices (all, or chosen ones), so routes can say "page the on-call phone". Delivery failures show up in
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
2. **Read surfaces:** notification feed, incidents, servers, monitors, logs, deliveries, search.
3. **Pairing and push (the core of the product, build early):** device API in Core, Console pairing UI, push relay, `mobile_push` channel.
4. **Actions:** acknowledge/resolve/note, pause/resume monitor, test run.
5. **Polish and release:** biometric lock, offline cache, multi-Core, App Store / Play review,
   Critical Alerts entitlement.
6. Widgets and Live Activity land last inside v1 but gate the release only if time allows; they
   are the first cut if scope must shrink.

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
