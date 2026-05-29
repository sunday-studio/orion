# Public Subscriber Mail Sender

Status: accepted

Date: 2026-05-27

Ticket: T-20260527-174212-709a

## Context

Status page subscriptions need confirmation, preference, unsubscribe, and fan-out emails. Orion already has internal alert SMTP services and email destinations, but public subscriber mail has a different trust boundary: it sends customer-facing status updates and must not expose internal alert routing, operator destinations, escalation policy, or alert-channel secrets.

## Decision

Use a dedicated public subscriber mail sender configuration for status page email. It may reuse Orion's low-level SMTP client code, retry primitives, and redaction helpers, but it must not reuse internal alert destinations or alert routes as subscriber sender records.

The first implementation should model one public mail sender per Core instance, with room for per-status-page overrides later. The sender configuration includes:

- enabled flag;
- from name and from address;
- optional reply-to address;
- optional bounce address or provider-specific return-path guidance;
- SMTP service reference or provider reference selected specifically for public subscriber mail;
- public URL origin used to build confirmation, manage, and unsubscribe links;
- per-destination and global rate limits;
- template subject prefixes for confirmation, incident update, maintenance, and unsubscribe messages.

If the sender is not configured, public subscription request routes may create pending subscriber records but must not attempt to send confirmation or fan-out mail. The API should return a safe, generic accepted response and record an internal operational event for the missing sender configuration.

## Boundary Rules

- Public subscriber code may read only the dedicated public sender configuration and sanitized public status page DTOs.
- Public subscriber code must not read alert routes, alert destination recipient lists, internal alert templates, escalation policy, or operator-only incident fields.
- SMTP credentials remain encrypted or otherwise protected using the same secret-handling rules as internal SMTP services.
- Admin APIs may expose public sender readiness and masked addresses, but not raw SMTP passwords or subscriber tokens.
- Delivery records store public subscriber ids, public incident ids, public update ids, provider message ids, status, attempts, and sanitized errors only.

## Token URL Origin

Confirmation, manage, and unsubscribe links use the configured public URL origin. If a status page later has a validated custom domain, that domain may override the instance origin for that page. Core must reject token URL generation when the origin is empty, non-HTTP(S), localhost in production, or otherwise invalid for the configured deployment mode.

The public origin is configuration, not request-derived state. Do not build subscriber token links from arbitrary `Host`, `Forwarded`, or `X-Forwarded-Host` headers unless a later trusted-proxy configuration explicitly validates those headers.

## Reply And Bounce Handling

The default reply-to behavior is no-reply. Operators can configure a support reply-to address, but replies are outside Orion's first subscriber-mail scope.

Bounce handling starts as provider/manual guidance recorded on the sender configuration. Automatic bounce processing can be added later, but fan-out must already suppress subscribers marked `bounced`, `disabled`, or `unsubscribed`.

## Consequences

This keeps public status subscriptions separate from internal alert operations while still allowing implementation reuse at the transport-helper layer. It also gives confirmation-flow work a concrete dependency: create the subscriber lifecycle safely first, then enable actual mail delivery only when a public sender and URL origin are configured.
