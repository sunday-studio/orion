# Status Page Subscription Infrastructure Reuse

## Status

Accepted for the status page architecture decision track.

## Context

Status page subscriptions will notify public subscribers when administrators publish status page incident updates, maintenance updates, or future feed events. Orion already has internal alert infrastructure for operator notifications: alert channels, SMTP services, email destinations, routes, grouping, delivery attempts, cooldowns, and delivery history.

The open question is whether public status page subscriptions should reuse that alert channel infrastructure directly or use a separate public subscriber system.

## Decision

Use a separate public status page subscriber system, while reusing lower-level delivery primitives where they are already safe and generic.

Status page subscriptions should have their own tables, public confirmation tokens, unsubscribe tokens, preferences, and delivery history. They should not be represented as alert channels, alert routes, or alert email destinations.

Reuse should stop at implementation primitives:

- SMTP service configuration can be reused as an outbound transport source when an administrator chooses which sender a status page uses.
- Email/webhook delivery adapters, retry/backoff helpers, delivery attempt recording patterns, and template rendering utilities can be shared after they are made channel-neutral.
- Internal alert route matching, alert grouping, alert cooldown semantics, alert delivery logs, and operator destination records should remain internal-alert only.

## Comparison

| Concern | Directly reuse alert infrastructure | Separate public subscriber system |
| --- | --- | --- |
| Privacy | Risks mixing public subscriber addresses with operator destinations and exposing public delivery failures in internal alert workflows. Alert routes also key off internal incidents, servers, monitors, severities, and monitor types that should not leak into public subscription behavior. | Keeps subscriber identity, preferences, confirmation state, and public delivery logs behind status page boundaries. Public fan-out only sees public incident/update ids, public component ids, and approved copy. |
| Delivery reliability | Gains current alert retry and attempt behavior quickly, but couples public fan-out volume to operator alert delivery. A public subscriber surge could delay or obscure critical internal alerts unless every queue and worker path is reworked. | Allows separate queue priority, concurrency, retry windows, bounce handling, and operational metrics. Shared delivery helpers can still avoid duplicate transport code without sharing operational capacity. |
| Unsubscribe handling | Alert destinations are admin-managed and event subscription fields are operator controls, not public unsubscribe records. Adding one-click unsubscribe and confirmation tokens would distort the alert destination model. | First-class subscriber records can store pending/confirmed/unsubscribed state, per-component preferences, token hashes, suppression reasons, and audit-safe timestamps without affecting alert channel configuration. |
| Rate limits | Existing alert cooldowns suppress repeated operator notifications by incident/channel. They do not cover anonymous subscription creation, email confirmation, token guessing, public webhook abuse, or per-status-page subscriber fan-out. | Public endpoints can have their own IP and destination rate limits for subscribe, confirm, resend confirmation, unsubscribe, and webhook registration. Fan-out can also enforce per-page and per-destination quotas. |
| Future channel support | Alert channels are built around operator webhooks and email destinations. Extending them to RSS/Atom, public webhooks, Slack-style public subscriptions, or component-scoped notifications would create mixed semantics in alert routes. | The subscriber model can add destination types such as email, webhook, RSS/Atom feed registration, Slack-style targets, and component-scoped preferences without changing internal alert routes. |

## Implementation Notes

Add status page specific data structures when Phase 4 begins:

- `status_page_subscriptions`: page id, destination type, destination address or endpoint reference, confirmation state, token hashes, component scope, timestamps, and disabled/unsubscribed reason.
- `status_page_subscription_deliveries`: public incident/update id, subscription id, destination type, status, attempt counters, next attempt, last attempt, and redacted error summary.
- `status_page_subscription_delivery_attempts`: per-attempt status, stage, started/completed timestamps, and redacted transport error.

Public notification fan-out should enqueue from published `status_page_incident_updates` or scheduled maintenance publication events, not from internal incident events. The payload builder must accept only public DTOs so it cannot accidentally include server ids, monitor ids, private notes, raw reports, internal hostnames, or alert channel internals.

For email, administrators may choose an existing `alert_smtp_services` sender or configure a later status-page-specific sender policy. Subscriber email addresses should not be stored in `alert_email_destinations`.

## Consequences

This creates more schema and service work than direct reuse, but it preserves the privacy boundary that the status page architecture depends on. It also lets public fan-out scale and fail independently from operator alerting.

The alert system remains the internal operational notification system. Status page subscriptions become an external publication notification system that shares only safe transport code and sender configuration.
