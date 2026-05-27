# Public Incident Automation Policy

Status: accepted

Date: 2026-05-27

Ticket: T-20260527-112239-ec89

## Context

Status pages are a publication layer over internal Core incidents, not a public mirror of incident reconciliation. Internal incident events can contain operational details that are useful to Orion operators but unsafe or confusing for public readers. Automation should reduce drafting work without publishing unreviewed incident copy by default.

## Decision

Internal incidents are private by default. Core must never publish an internal incident, incident update, or resolution update to a public status page unless a status page policy explicitly allows that automation.

The default policy for every new status page is `manual_review`:

- failing internal incidents may suggest affected public components;
- an administrator must create or approve the public incident before it is published;
- public incident titles, impact summaries, and timeline messages are separate public fields, not copied directly from internal events;
- public updates remain drafts until an administrator publishes them;
- subscriber fan-out only runs for published public incident updates.

## Auto-Draft Behavior

When an internal incident opens or changes, Core may create or update a draft public incident only when all of these conditions are true:

- the status page has incident automation enabled for drafts;
- the internal incident maps to at least one visible public component;
- the mapped component does not have a manual public status override;
- the generated draft uses safe templates and public component names only;
- validation finds no private identifiers, raw payload text, internal hostnames, IP addresses, tokens, stack traces, or Console-only diagnostics.

Auto-drafts must be clearly marked as drafts in Console and must not appear on public routes. Draft creation should be idempotent per linked internal incident and status page. If validation fails, Core records a private admin warning and leaves public publication empty rather than publishing partial content.

## Auto-Resolution Suggestions

When a linked internal incident resolves, Core may suggest a public resolution update. The first release should create a draft update such as "The affected component has recovered and is being monitored" using the public component name and rounded recovery time.

Publishing the final resolution update remains a manual administrator action under the default policy. Console should show the suggested resolution beside the linked internal incident state so the operator can publish it, edit it, or leave the public incident in `monitoring` while they verify recovery.

If a public incident is already published and the linked internal incident reopens before the resolution draft is published, Core should discard or supersede the stale resolution draft and suggest a new public update for the active failure instead.

## Trusted-Component Automation

Later releases may allow a component-level policy named `trusted_auto_publish`. This policy is opt-in per public component and must be disabled by default.

A component can use trusted automation only when:

- it maps to monitors whose public names and failure modes have been reviewed;
- all public copy is generated from approved templates;
- no internal resource identifiers are exposed in the public DTO;
- the status page owner has enabled auto-publication for that component;
- audit events record the policy, generated text, affected components, and resulting published update;
- external subscriber fan-out can still require separate manual approval when configured.

Trusted automation may publish these events without manual review:

- open a public incident for a trusted component with templated safe copy;
- update the public incident status from `investigating` to `monitoring` when the linked internal incident resolves;
- publish a resolved update after a configured recovery confirmation window.

Trusted automation must not publish:

- incident details derived from raw monitor payloads or internal error text;
- custom free-form messages generated from internal notes;
- updates for components with manual status overrides;
- cross-component incidents when any affected component is not trusted;
- notification fan-out if the page or component requires manual notification approval.

## Consequences

The first implementation favors privacy and operator control over fast unattended public communication. Core can still save time by suggesting affected components and drafting safe updates, while public readers only see administrator-approved communication unless the page owner later opts specific reviewed components into trusted automation.
