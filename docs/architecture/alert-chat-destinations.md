# Alert Chat Destinations

Status: superseded

Superseded by: `docs/plans/alert-webhook-rules-cleanup.md`

## Decision

Slack and Discord are not first-class internal alert destination types.

Internal alerting uses generic webhook destinations only. Provider-specific
formats, provider-specific validation, and provider-specific channel settings
belong outside Core's alert destination model. A Slack, Discord, PagerDuty, or
other integration can still receive Orion alerts through a generic webhook URL or
an external adapter that accepts the generic Orion alert payload.

## Boundary

Core alert rules target webhook destinations. The supported delivery object is
an `alert_deliveries` row whose `type` is `webhook`.

The previous chat-destination plan proposed adding `slack` and `discord`
channel types. That would keep provider details inside the alerting model and
conflict with the webhook-only cleanup goal.

## Migration Note

Any stale Slack or Discord alert tickets should point at the webhook-only alert
cleanup goal instead of adding provider-specific Core or Console surfaces.
