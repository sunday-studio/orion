# Artboards

Static exports of every canvas artboard (116), so the web and agent threads can build 1:1 without access to the canvas artifact.

Source of truth is the canvas (https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY); these files are a snapshot taken 2026-10-18 and mirror `docs/design/design-language.md`. If they disagree, `design-language.md` wins.

## Files

- `<name>.html`: self-contained artboard, markup plus the shared `orion.css` in this folder. Open it in a browser at 1280px width. It has no scripts.
- `png/<name>-light.png`: render at 1280px width, height as listed below.
- `png/<name>-dark.png`: dark render. Only Overview has a hand-drawn dark design (`dashboard-dark`). Every other dark image is a derived preview using the dark tokens from `design-language.md` section 11 and may need small adjustments. Login, public pages and CLI screens have no separate dark design, so their dark image matches the light one (CLI terminals are dark by nature).
- `orion.css`: shared stylesheet, later rules override earlier ones. It is the canvas stylesheet, so class names are canvas class names, not production names: build components from `design-language.md` section 14 and use this file only for exact values.

## Fonts

The PNGs were rendered with TWK Lausanne (trial build). The font files are not in the repo (decision D-010); `orion.css` falls back to the system sans, so the HTML renders in system sans unless the licensed font is loaded. Metrics differ slightly; match the PNGs for layout.

## Index

Screen-map ref is the row number in `../screen-map.md` (`9.x` is section 9, product logs; `7.10` is section 7 to 10 additions). Console column is the proposed route and file in `apps/console/src`.

| Artboard | Height | Screen map | Route | Console file |
|---|---|---|---|---|
| [Main](Main.html) ([light](png/Main-light.png), [dark](png/Main-dark.png)) | 779 | 38 | / | apps/console/src/features/overview/ (new) |
| [add-server](add-server.html) ([light](png/add-server-light.png), [dark](png/add-server-dark.png)) | 623 | 37 | /servers/new | new |
| [add-server-connected](add-server-connected.html) ([light](png/add-server-connected-light.png), [dark](png/add-server-connected-dark.png)) | 708 | 37 | /servers/new | new |
| [add-server-failed](add-server-failed.html) ([light](png/add-server-failed-light.png), [dark](png/add-server-failed-dark.png)) | 523 | 37 | /servers/new | new |
| [add-server-waiting](add-server-waiting.html) ([light](png/add-server-waiting-light.png), [dark](png/add-server-waiting-dark.png)) | 481 | 37 | /servers/new | new |
| [alerts-channels](alerts-channels.html) ([light](png/alerts-channels-light.png), [dark](png/alerts-channels-dark.png)) | 460 | 8 | /alerts/channels | apps/console/src/features/alerts/ |
| [alerts-log](alerts-log.html) ([light](png/alerts-log-light.png), [dark](png/alerts-log-dark.png)) | 527 | 8 | /alerts | apps/console/src/features/alerts/alerts.view.tsx |
| [alerts-routes](alerts-routes.html) ([light](png/alerts-routes-light.png), [dark](png/alerts-routes-dark.png)) | 592 | 20 | /alerts/routes | apps/console/src/features/alerts/ |
| [alerts-rules](alerts-rules.html) ([light](png/alerts-rules-light.png), [dark](png/alerts-rules-dark.png)) | 578 | 19 | /alerts/rules | apps/console/src/features/alerts/ |
| [app-connect-create](app-connect-create.html) ([light](png/app-connect-create-light.png), [dark](png/app-connect-create-dark.png)) | 477 | 9.x | /apps/new | new |
| [app-connect-received](app-connect-received.html) ([light](png/app-connect-received-light.png), [dark](png/app-connect-received-dark.png)) | 460 | 9.x | /apps/new | new |
| [app-connect-snippet](app-connect-snippet.html) ([light](png/app-connect-snippet-light.png), [dark](png/app-connect-snippet-dark.png)) | 629 | 9.x | /apps/new | new |
| [app-connect-waiting](app-connect-waiting.html) ([light](png/app-connect-waiting-light.png), [dark](png/app-connect-waiting-dark.png)) | 460 | 9.x | /apps/new | new |
| [app-display](app-display.html) ([light](png/app-display-light.png), [dark](png/app-display-dark.png)) | 898 | 9.x | /settings/apps/:id | apps/console/src/features/settings/ |
| [app-key-new](app-key-new.html) ([light](png/app-key-new-light.png), [dark](png/app-key-new-dark.png)) | 640 | 9.x | dialog | apps/console/src/features/settings/ |
| [app-keys](app-keys.html) ([light](png/app-keys-light.png), [dark](png/app-keys-dark.png)) | 763 | 9.x | /settings/apps/:id | apps/console/src/features/settings/ |
| [app-redaction](app-redaction.html) ([light](png/app-redaction-light.png), [dark](png/app-redaction-dark.png)) | 907 | 9.x | /settings/apps/:id | apps/console/src/features/settings/ |
| [app-retention](app-retention.html) ([light](png/app-retention-light.png), [dark](png/app-retention-dark.png)) | 654 | 9.x | /settings/apps/:id | apps/console/src/features/settings/ |
| [app-send-test](app-send-test.html) ([light](png/app-send-test-light.png), [dark](png/app-send-test-dark.png)) | 992 | 9.x | /settings/apps/:id | apps/console/src/features/settings/ |
| [avatar-menu](avatar-menu.html) ([light](png/avatar-menu-light.png), [dark](png/avatar-menu-dark.png)) | 460 | 49 | global header | components/shared/app-header.tsx |
| [channel-detail](channel-detail.html) ([light](png/channel-detail-light.png), [dark](png/channel-detail-dark.png)) | 994 | 7 | /alerts/channels/:id | apps/console/src/features/alerts/ |
| [channel-dialog](channel-dialog.html) ([light](png/channel-dialog-light.png), [dark](png/channel-dialog-dark.png)) | 640 | 18 | dialog | apps/console/src/features/alerts/ |
| [channel-email](channel-email.html) ([light](png/channel-email-light.png), [dark](png/channel-email-dark.png)) | 640 | 18 | dialog | apps/console/src/features/alerts/ |
| [channel-test](channel-test.html) ([light](png/channel-test-light.png), [dark](png/channel-test-dark.png)) | 640 | 18 | dialogs | apps/console/src/features/alerts/ |
| [channel-webhook](channel-webhook.html) ([light](png/channel-webhook-light.png), [dark](png/channel-webhook-dark.png)) | 697 | 18 | dialog | apps/console/src/features/alerts/ |
| [cli-doctor](cli-doctor.html) ([light](png/cli-doctor-light.png), [dark](png/cli-doctor-dark.png)) | 749 | 10 (CLI) | terminal | apps/agent |
| [cli-help-error](cli-help-error.html) ([light](png/cli-help-error-light.png), [dark](png/cli-help-error-dark.png)) | 1098 | 10 (CLI) | terminal | apps/agent |
| [cli-install](cli-install.html) ([light](png/cli-install-light.png), [dark](png/cli-install-dark.png)) | 781 | 10 (CLI) | terminal | apps/agent |
| [cli-logs](cli-logs.html) ([light](png/cli-logs-light.png), [dark](png/cli-logs-dark.png)) | 659 | 10 (CLI) | terminal | apps/agent |
| [cli-setup](cli-setup.html) ([light](png/cli-setup-light.png), [dark](png/cli-setup-dark.png)) | 1206 | 10 (CLI) | terminal | apps/agent |
| [cli-status](cli-status.html) ([light](png/cli-status-light.png), [dark](png/cli-status-dark.png)) | 747 | 10 (CLI) | terminal | apps/agent |
| [command-palette](command-palette.html) ([light](png/command-palette-light.png), [dark](png/command-palette-dark.png)) | 640 | 52 | global dialog | components/shared/ (new) |
| [dashboard-dark](dashboard-dark.html) ([light](png/dashboard-dark-light.png), [dark](png/dashboard-dark-dark.png)) | 783 | 38 | / | dark reference |
| [delete-confirm](delete-confirm.html) ([light](png/delete-confirm-light.png), [dark](png/delete-confirm-dark.png)) | 640 | 44 | dialogs | components/ui/dialog.tsx |
| [delivery-detail](delivery-detail.html) ([light](png/delivery-detail-light.png), [dark](png/delivery-detail-dark.png)) | 511 | 47 | /alerts/deliveries/:id | apps/console/src/features/alerts/ |
| [design-language](design-language.html) ([light](png/design-language-light.png), [dark](png/design-language-dark.png)) | 4500 | reference | component library | docs/design/design-language.md |
| [email-confirm](email-confirm.html) ([light](png/email-confirm-light.png), [dark](png/email-confirm-dark.png)) | 700 | 7 | public (Core) | Core-served |
| [flows](flows.html) ([light](png/flows-light.png), [dark](png/flows-dark.png)) | 2684 | 5 (flows) | reference | none |
| [health-issues](health-issues.html) ([light](png/health-issues-light.png), [dark](png/health-issues-dark.png)) | 460 | 39 | /health | new |
| [heartbeat-setup](heartbeat-setup.html) ([light](png/heartbeat-setup-light.png), [dark](png/heartbeat-setup-dark.png)) | 774 | 14 | panel | apps/console/src/features/monitors/ |
| [incident-actions](incident-actions.html) ([light](png/incident-actions-light.png), [dark](png/incident-actions-dark.png)) | 640 | 17 | dialogs | apps/console/src/features/incidents/ |
| [incident-candidates](incident-candidates.html) ([light](png/incident-candidates-light.png), [dark](png/incident-candidates-dark.png)) | 460 | 40 | /incidents/triage | apps/console/src/features/incidents/ (new) |
| [incident-declare](incident-declare.html) ([light](png/incident-declare-light.png), [dark](png/incident-declare-dark.png)) | 640 | 7 | dialog | apps/console/src/features/incidents/ |
| [incident-detail](incident-detail.html) ([light](png/incident-detail-light.png), [dark](png/incident-detail-dark.png)) | 848 | 3 | /incidents/:id | apps/console/src/features/incidents/incident-detail.view.tsx |
| [incident-public-update](incident-public-update.html) ([light](png/incident-public-update-light.png), [dark](png/incident-public-update-dark.png)) | 640 | 7 | dialog | apps/console/src/features/incidents/ |
| [incident-resolved](incident-resolved.html) ([light](png/incident-resolved-light.png), [dark](png/incident-resolved-dark.png)) | 854 | 7 | /incidents/:id | apps/console/src/features/incidents/incident-detail.view.tsx |
| [incidents-bulk](incidents-bulk.html) ([light](png/incidents-bulk-light.png), [dark](png/incidents-bulk-dark.png)) | 460 | 7 | /incidents | apps/console/src/features/incidents/ |
| [incidents-empty](incidents-empty.html) ([light](png/incidents-empty-light.png), [dark](png/incidents-empty-dark.png)) | 460 | 7 | /incidents | apps/console/src/features/incidents/ |
| [incidents-list](incidents-list.html) ([light](png/incidents-list-light.png), [dark](png/incidents-list-dark.png)) | 508 | 2 | /incidents | apps/console/src/features/incidents/incidents.view.tsx |
| [login](login.html) ([light](png/login-light.png), [dark](png/login-dark.png)) | 720 | 1 | /login | apps/console/src/features/auth/login.view.tsx |
| [logs](logs.html) ([light](png/logs-light.png), [dark](png/logs-dark.png)) | 1285 | 10 | /logs (Service logs) | apps/console/src/features/event-log/logs.view.tsx |
| [logs-detail](logs-detail.html) ([light](png/logs-detail-light.png), [dark](png/logs-detail-dark.png)) | 522 | 48 | drawer | apps/console/src/features/event-log/ |
| [logs-detail-plain](logs-detail-plain.html) ([light](png/logs-detail-plain-light.png), [dark](png/logs-detail-plain-dark.png)) | 556 | 48 | drawer | apps/console/src/features/event-log/ |
| [logs-error](logs-error.html) ([light](png/logs-error-light.png), [dark](png/logs-error-dark.png)) | 576 | 9.x | /logs | apps/console/src/features/event-log/ |
| [logs-events](logs-events.html) ([light](png/logs-events-light.png), [dark](png/logs-events-dark.png)) | 1063 | 10 | /logs (Status logs) | apps/console/src/features/event-log/ |
| [logs-export](logs-export.html) ([light](png/logs-export-light.png), [dark](png/logs-export-dark.png)) | 640 | 9.x | dialog | apps/console/src/features/event-log/ |
| [logs-live](logs-live.html) ([light](png/logs-live-light.png), [dark](png/logs-live-dark.png)) | 720 | 9.x | /logs (live) | apps/console/src/features/event-log/ |
| [logs-no-apps](logs-no-apps.html) ([light](png/logs-no-apps-light.png), [dark](png/logs-no-apps-dark.png)) | 464 | 9.x | /logs | apps/console/src/features/event-log/ |
| [logs-no-results](logs-no-results.html) ([light](png/logs-no-results-light.png), [dark](png/logs-no-results-dark.png)) | 574 | 9.x | /logs | apps/console/src/features/event-log/ |
| [logs-patterns](logs-patterns.html) ([light](png/logs-patterns-light.png), [dark](png/logs-patterns-dark.png)) | 540 | 9.x | /logs (patterns) | apps/console/src/features/event-log/ |
| [logs-plain](logs-plain.html) ([light](png/logs-plain-light.png), [dark](png/logs-plain-dark.png)) | 859 | 9.x | /logs | apps/console/src/features/event-log/ |
| [logs-rich](logs-rich.html) ([light](png/logs-rich-light.png), [dark](png/logs-rich-dark.png)) | 1016 | 9.x | /logs | apps/console/src/features/event-log/ |
| [logs-service](logs-service.html) ([light](png/logs-service-light.png), [dark](png/logs-service-dark.png)) | 567 | 10 | /logs (Internal logs) | apps/console/src/features/event-log/ |
| [logs-views](logs-views.html) ([light](png/logs-views-light.png), [dark](png/logs-views-dark.png)) | 460 | 9.x | menus | apps/console/src/features/event-log/ |
| [monitor-agent-setup](monitor-agent-setup.html) ([light](png/monitor-agent-setup-light.png), [dark](png/monitor-agent-setup-dark.png)) | 775 | 13 | dialog | apps/console/src/features/monitors/ |
| [monitor-create](monitor-create.html) ([light](png/monitor-create-light.png), [dark](png/monitor-create-dark.png)) | 640 | 13 | dialog | apps/console/src/features/monitors/ |
| [monitor-create-tcp](monitor-create-tcp.html) ([light](png/monitor-create-tcp-light.png), [dark](png/monitor-create-tcp-dark.png)) | 640 | 13 | dialog | apps/console/src/features/monitors/ |
| [monitor-detail](monitor-detail.html) ([light](png/monitor-detail-light.png), [dark](png/monitor-detail-dark.png)) | 804 | 7 | /monitors/:id | apps/console/src/features/monitor-detail/monitor-detail.view.tsx |
| [monitor-edit](monitor-edit.html) ([light](png/monitor-edit-light.png), [dark](png/monitor-edit-dark.png)) | 948 | 43 | /monitors/:id/edit | apps/console/src/features/monitor-detail/ |
| [monitor-maintenance](monitor-maintenance.html) ([light](png/monitor-maintenance-light.png), [dark](png/monitor-maintenance-dark.png)) | 640 | 42 | dialog | apps/console/src/features/monitor-detail/ |
| [monitor-paused](monitor-paused.html) ([light](png/monitor-paused-light.png), [dark](png/monitor-paused-dark.png)) | 469 | 42 | /monitors/:id | apps/console/src/features/monitor-detail/ |
| [monitor-test](monitor-test.html) ([light](png/monitor-test-light.png), [dark](png/monitor-test-dark.png)) | 640 | 41 | dialog | apps/console/src/features/monitor-detail/ |
| [monitors-bulk](monitors-bulk.html) ([light](png/monitors-bulk-light.png), [dark](png/monitors-bulk-dark.png)) | 460 | 7 | /monitors | apps/console/src/features/monitors/ |
| [monitors-empty](monitors-empty.html) ([light](png/monitors-empty-light.png), [dark](png/monitors-empty-dark.png)) | 460 | 7 | /monitors | apps/console/src/features/monitors/ |
| [monitors-list](monitors-list.html) ([light](png/monitors-list-light.png), [dark](png/monitors-list-dark.png)) | 547 | 6 | /monitors | apps/console/src/features/monitors/monitors.view.tsx |
| [onboarding](onboarding.html) ([light](png/onboarding-light.png), [dark](png/onboarding-dark.png)) | 460 | 36 | / (first run card) | apps/console/src/features/overview/ (new) |
| [overview-all-good](overview-all-good.html) ([light](png/overview-all-good-light.png), [dark](png/overview-all-good-dark.png)) | 640 | 38 | / | apps/console/src/features/overview/ (new) |
| [public-history](public-history.html) ([light](png/public-history-light.png), [dark](png/public-history-dark.png)) | 680 | 28 | /status/:slug/history | Core-served |
| [public-incident](public-incident.html) ([light](png/public-incident-light.png), [dark](png/public-incident-dark.png)) | 620 | 30 | /status/:slug/incidents/:id | Core-served |
| [public-manage](public-manage.html) ([light](png/public-manage-light.png), [dark](png/public-manage-dark.png)) | 560 | 33 | .../subscribers/manage/:token | Core-served |
| [public-status](public-status.html) ([light](png/public-status-light.png), [dark](png/public-status-dark.png)) | 920 | 27 | /status/:slug | Core-served |
| [public-subscribe](public-subscribe.html) ([light](png/public-subscribe-light.png), [dark](png/public-subscribe-dark.png)) | 660 | 31 | /status/:slug/subscribe | Core-served |
| [public-unsubscribe](public-unsubscribe.html) ([light](png/public-unsubscribe-light.png), [dark](png/public-unsubscribe-dark.png)) | 560 | 34 | .../subscribers/unsubscribe/:token | Core-served |
| [report-drawer](report-drawer.html) ([light](png/report-drawer-light.png), [dark](png/report-drawer-dark.png)) | 460 | 16 | drawer | apps/console/src/features/report-inspection/ |
| [server-decommission](server-decommission.html) ([light](png/server-decommission-light.png), [dark](png/server-decommission-dark.png)) | 652 | 45 | dialog | apps/console/src/features/agent-detail/ |
| [server-detail](server-detail.html) ([light](png/server-detail-light.png), [dark](png/server-detail-dark.png)) | 831 | 5 | /servers/:id | apps/console/src/features/agent-detail/agent-detail.view.tsx |
| [server-edit](server-edit.html) ([light](png/server-edit-light.png), [dark](png/server-edit-dark.png)) | 640 | 7 | dialog | apps/console/src/features/agent-detail/ |
| [server-metrics](server-metrics.html) ([light](png/server-metrics-light.png), [dark](png/server-metrics-dark.png)) | 528 | 5 | /servers/:id (Reports tab) | apps/console/src/features/agent-detail/ |
| [server-monitors](server-monitors.html) ([light](png/server-monitors-light.png), [dark](png/server-monitors-dark.png)) | 528 | 7 | /servers/:id (Monitors tab) | apps/console/src/features/agent-detail/ |
| [server-stale](server-stale.html) ([light](png/server-stale-light.png), [dark](png/server-stale-dark.png)) | 766 | 7 | /servers/:id | apps/console/src/features/agent-detail/ |
| [server-token](server-token.html) ([light](png/server-token-light.png), [dark](png/server-token-dark.png)) | 640 | 21 | /servers/:id (Token tab) | apps/console/src/features/agent-detail/ |
| [server-token-new](server-token-new.html) ([light](png/server-token-new-light.png), [dark](png/server-token-new-dark.png)) | 640 | 7 | dialog | apps/console/src/features/agent-detail/ |
| [servers-empty](servers-empty.html) ([light](png/servers-empty-light.png), [dark](png/servers-empty-dark.png)) | 460 | 7 | /servers | apps/console/src/features/agents/ |
| [servers-list](servers-list.html) ([light](png/servers-list-light.png), [dark](png/servers-list-dark.png)) | 474 | 4 | /servers | apps/console/src/features/agents/agents.view.tsx |
| [session-expired](session-expired.html) ([light](png/session-expired-light.png), [dark](png/session-expired-dark.png)) | 720 | 49 | /login (401) | apps/console/src/features/auth/ |
| [settings-app-new](settings-app-new.html) ([light](png/settings-app-new-light.png), [dark](png/settings-app-new-dark.png)) | 640 | 9.x | dialog | apps/console/src/features/settings/ |
| [settings-apps](settings-apps.html) ([light](png/settings-apps-light.png), [dark](png/settings-apps-dark.png)) | 632 | 9.x | /settings/apps | apps/console/src/features/settings/ |
| [settings-audit](settings-audit.html) ([light](png/settings-audit-light.png), [dark](png/settings-audit-dark.png)) | 565 | 50 | /settings/audit | apps/console/src/features/settings/ |
| [settings-general](settings-general.html) ([light](png/settings-general-light.png), [dark](png/settings-general-dark.png)) | 624 | 50 | /settings/general | apps/console/src/features/settings/ |
| [settings-lifecycle](settings-lifecycle.html) ([light](png/settings-lifecycle-light.png), [dark](png/settings-lifecycle-dark.png)) | 703 | 11 | /settings | apps/console/src/features/settings/settings.view.tsx |
| [settings-system](settings-system.html) ([light](png/settings-system-light.png), [dark](png/settings-system-dark.png)) | 730 | 50 | /settings/system | apps/console/src/features/settings/ |
| [settings-token-new](settings-token-new.html) ([light](png/settings-token-new-light.png), [dark](png/settings-token-new-dark.png)) | 640 | 50 | dialog | apps/console/src/features/settings/ |
| [settings-tokens](settings-tokens.html) ([light](png/settings-tokens-light.png), [dark](png/settings-tokens-dark.png)) | 472 | 50 | /settings/security | apps/console/src/features/settings/ |
| [shortcuts](shortcuts.html) ([light](png/shortcuts-light.png), [dark](png/shortcuts-dark.png)) | 640 | 7.10 | global dialog | components/shared/ (new) |
| [states-gallery](states-gallery.html) ([light](png/states-gallery-light.png), [dark](png/states-gallery-dark.png)) | 1100 | 54 | components/shared/ | empty-state.tsx and others |
| [status-badge](status-badge.html) ([light](png/status-badge-light.png), [dark](png/status-badge-dark.png)) | 560 | 35 | badge SVG | Core-served |
| [status-components](status-components.html) ([light](png/status-components-light.png), [dark](png/status-components-dark.png)) | 658 | 24 | (Components) | apps/console/src/features/status-pages/ |
| [status-create](status-create.html) ([light](png/status-create-light.png), [dark](png/status-create-dark.png)) | 640 | 9 | dialog | apps/console/src/features/status-pages/ |
| [status-domain](status-domain.html) ([light](png/status-domain-light.png), [dark](png/status-domain-dark.png)) | 716 | 26 | (Setup) | apps/console/src/features/status-pages/ |
| [status-incidents](status-incidents.html) ([light](png/status-incidents-light.png), [dark](png/status-incidents-dark.png)) | 807 | 22 | (Incidents) | apps/console/src/features/status-pages/ |
| [status-maintenance](status-maintenance.html) ([light](png/status-maintenance-light.png), [dark](png/status-maintenance-dark.png)) | 640 | 7 | dialog | apps/console/src/features/status-pages/ |
| [status-pages-list](status-pages-list.html) ([light](png/status-pages-list-light.png), [dark](png/status-pages-list-dark.png)) | 460 | 9 | /status-pages | apps/console/src/features/status-pages/status-pages.view.tsx |
| [status-preview](status-preview.html) ([light](png/status-preview-light.png), [dark](png/status-preview-dark.png)) | 623 | 9 | (Preview) | apps/console/src/features/status-pages/ |
| [status-publish](status-publish.html) ([light](png/status-publish-light.png), [dark](png/status-publish-dark.png)) | 640 | 26 | dialogs | apps/console/src/features/status-pages/ |
| [status-setup](status-setup.html) ([light](png/status-setup-light.png), [dark](png/status-setup-dark.png)) | 929 | 26 | /status-pages/:id (Setup) | apps/console/src/features/status-pages/ |
| [status-subscribers](status-subscribers.html) ([light](png/status-subscribers-light.png), [dark](png/status-subscribers-dark.png)) | 497 | 25 | (Subscribers) | apps/console/src/features/status-pages/ |
