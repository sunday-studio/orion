# Alert Chat Destinations

This plan defines first-class Slack and Discord alert destinations without
colliding with the active signed-webhook and HTML email payload work.

## Decision

Slack and Discord should be modeled as alert channel types, not as reusable SMTP
destinations and not as generic outbound webhooks. Core already has the routing,
subscription, delivery history, test action, cooldown, and retry machinery around
`alert_channels`; adding chat destinations there keeps route behavior identical
for webhook, email, Slack, and Discord delivery.

The implementation should add these `alert_channels.type` values:

- `slack`
- `discord`

No new table is required for the first implementation slice. Both providers use
incoming webhook URLs, so the existing `webhook_url` column is the secret-bearing
endpoint field. Existing `enabled`, `subscribed_events`, delivery attempts,
route `channel_ids`, and last delivery status behavior continue to apply.

## Configuration Fields

Create and update payloads for `POST /v1/alerts/channels` and
`PATCH /v1/alerts/channels/{id}` should support:

- `name`: required display name, unique across alert channels.
- `type`: `slack` or `discord`.
- `enabled`: optional boolean, default `true`.
- `webhook_url`: required incoming webhook URL.
- `subscribed_events`: optional alert event list, defaulting to incident opened
  and incident resolved.

The API response can keep using `webhook_configured` and may return
`webhook_url` for now because existing webhook channels already expose it. If the
signed-webhook worker changes secret redaction semantics, Slack and Discord
should follow that settled response policy.

Provider-specific fields such as Slack channel override, username, icon, Discord
username, avatar URL, mentions, or thread behavior should be deferred until a
real use case requires them. Incoming webhook defaults are enough for first-class
delivery.

## Adapter Contract

Chat adapters should render from `service.AlertPayload`, not from database rows
directly. The contract is:

```go
type ChatDestination string

const (
	ChatDestinationSlack   ChatDestination = "slack"
	ChatDestinationDiscord ChatDestination = "discord"
)

func RenderChatAlert(destination ChatDestination, payload AlertPayload) ([]byte, string, error)
```

The returned byte slice is the provider JSON body. The returned string is the
HTTP `Content-Type`; both first providers should use `application/json`.

Delivery should share the same outer path as current webhooks:

1. Queue creates a pending `alert_deliveries` row with `type` set to `slack` or
   `discord`.
2. Retry lookup resolves the delivery back to the matching channel by
   `channel` and `type`.
3. `deliver` dispatches to a chat webhook sender for `slack` and `discord`.
4. The sender validates `webhook_url`, builds `AlertPayload`, renders provider
   JSON, posts to the incoming webhook URL, and marks non-2xx responses as
   delivery failures.

Slack and Discord incoming webhooks do not need Orion signed-webhook headers.
The signed generic webhook path should stay specific to `type = webhook`.

## Provider Payload Shape

Both providers should carry the same information from `AlertPayload`:

- summary title and text;
- event type;
- severity;
- incident id and status;
- agent name or id when present;
- monitor name, id, and type when present;
- delivery timestamp;
- payload version `orion.alert.v1`;
- visible test marker when `payload.Test` is true.

Slack payload:

```json
{
  "text": "Orion alert: Alert channel test",
  "blocks": [
    {
      "type": "header",
      "text": { "type": "plain_text", "text": "Alert channel test" }
    },
    {
      "type": "section",
      "text": { "type": "mrkdwn", "text": "Manual alert channel test" }
    },
    {
      "type": "context",
      "elements": [
        { "type": "mrkdwn", "text": "event: test | severity: info | version: orion.alert.v1" }
      ]
    }
  ]
}
```

Discord payload:

```json
{
  "content": "Orion alert: Alert channel test",
  "embeds": [
    {
      "title": "Alert channel test",
      "description": "Manual alert channel test",
      "fields": [
        { "name": "Event", "value": "test", "inline": true },
        { "name": "Severity", "value": "info", "inline": true },
        { "name": "Incident", "value": "alert-channel-test", "inline": false }
      ],
      "footer": { "text": "orion.alert.v1" }
    }
  ]
}
```

The final renderer should truncate or omit fields to stay within provider limits:
Slack header text up to 150 characters, Slack block text up to 3000 characters,
Discord content up to 2000 characters, Discord title up to 256 characters,
Discord description up to 4096 characters, and Discord field values up to 1024
characters.

## Tests

The implementation should add focused Core tests:

- API create/update accepts `type = slack` and `type = discord` with
  `webhook_url`, stores them as alert channels, and rejects missing
  `webhook_url`.
- Manual channel test sends provider-shaped JSON to an HTTP test server and
  records delivery `type` as `slack` or `discord`.
- Incident notification routing can target Slack and Discord channel IDs and
  records sent deliveries through the existing route path.
- Renderer tests cover test payloads, incident payloads with agent and monitor
  context, and provider field truncation.

## Phased Implementation Tickets

1. Add Slack and Discord alert channel validation and API coverage.
2. Add chat renderer adapters and delivery tests after signed-webhook payload
   changes land.
3. Add Console destination affordances once the Core contract and generated SDK
   are stable.
