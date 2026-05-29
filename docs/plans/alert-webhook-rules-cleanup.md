# Alert Webhook Rules Cleanup

## Goal

Maat goal `G-20260529-190307-7def` tracks the alerting cleanup. [Certain]

Outcome: Orion alerting supports only generic webhook destinations, has editable rules with a flowchart Console setup, removes obsolete SMTP, Slack, and Discord alert functionality, and has E2E coverage proving the primary alert workflow. [Certain]

## Product Direction

Internal alert delivery should support generic webhooks only. [Likely]

SMTP should not be user-managed inside internal alerting. [Likely]

Slack and Discord should not remain as first-class alert channel types. [Likely]

Rules should become the user-facing policy object for trigger, filters, suppression, grouping, cooldown, recovery behavior, destination selection, and dry-run explanation. [Likely]

The Console should allow rule creation and editing through a flowchart-style setup. [Likely]

## Tickets

| Ticket | Title | Owner |
|---|---|---|
| `T-20260529-190327-fd04` | Remove non-webhook alert destinations | Faraday |
| `T-20260529-190327-47a9` | Move SMTP alert configuration to environment | Mendel |
| `T-20260529-190328-b618` | Add webhook-only alert rule CRUD API | Avicenna |
| `T-20260529-190328-ec47` | Build Console flowchart alert rule editor | Peirce |
| `T-20260529-190344-2f11` | Add alert rule E2E coverage | Sagan |
| `T-20260529-190344-607a` | Reconcile alert docs Maat and generated contracts | Dewey |
| `T-20260529-190344-6825` | Harden webhook outbound delivery policy | Queued because the current agent thread limit was reached |

## Implementation Sequence

1. Remove unsupported destination types and stale SMTP alert surfaces first, because rule CRUD should target the simplified model. [Likely]
2. Add webhook outbound safety before expanding user-facing rule workflows, because user-supplied webhook URLs are the remaining alert egress path. [Likely]
3. Add Core rule CRUD and dry-run semantics after the destination model is simplified. [Likely]
4. Build the Console flowchart editor against the generated rule SDK. [Likely]
5. Add E2E coverage after the Core and Console contracts stabilize. [Likely]
6. Reconcile docs, Maat, generated contracts, and seed data after implementation is complete. [Likely]

## Review Risks

The main conflict risk is in Core alert service and API files, because destination removal, rule CRUD, and webhook security all touch the alert delivery boundary. [Likely]

The main product risk is replacing a scattered alert model with another scattered model if route, rule, destination, and delivery language are not normalized together. [Likely]

The main security risk is treating generic webhooks as harmless after removing provider-specific integrations. [Likely]
