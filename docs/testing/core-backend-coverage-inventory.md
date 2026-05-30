# Core Backend Coverage Inventory

Last audited: 2026-05-30

This inventory maps the current Core backend surface to existing automated coverage and follow-up
tickets. It is intentionally stricter than package line coverage: a row is only `covered` when the
behavior has a happy path plus the relevant boundary, invalid-input, persistence, privacy, or
side-effect assertions for that surface.

## Status Key

| Status | Meaning |
| --- | --- |
| Covered | Existing tests exercise the important behavior and boundaries for the row. |
| Partial | Existing tests exercise some behavior, but direct coverage or a key boundary is missing. |
| Missing | No meaningful automated coverage was found for the row. |
| Deferred | The route or path is intentionally not a backend behavior target right now. |

## Follow-Up Tickets

| Ticket | Scope |
| --- | --- |
| T-20260529-190827-a7bb | Split shared Core API test helpers before expanding route coverage. |
| T-20260529-190828-4b8e | Move agent monitor and settings API tests into navigable files. |
| T-20260529-190828-a8e8 | Move incident alert and Core monitor API tests into navigable files. |
| T-20260529-190857-02c8 | Add auth and token lifecycle backend tests. |
| T-20260529-190857-16a8 | Add target-policy and public privacy tests. |
| T-20260529-190857-bb83 | Add alert data lifecycle and migration tests. |
| T-20260529-190941-2452 | Add Core race and contract gates. |
| T-20260529-190941-4acb | Document Core backend verification commands. |
| T-20260529-191005-6360 | Add Core backend PR coverage checklist. |
| T-20260529-191005-7dbf | Add coverage reporting for Core backend packages. |
| T-20260529-191006-2ea9 | Enforce test expectations for new Core backend work. |
| T-20260529-191329-9a02 | Add direct Core service unit coverage. |
| T-20260529-191339-c514 | Add Core startup and fallback route coverage. |
| T-20260529-191347-87fb | Add Core route coverage drift check. |

## Route Coverage Map

Source routes are registered in `apps/core/internal/api/routes.go` and
`apps/core/internal/api/status_pages.go`.

### Global And Static Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /swagger/*any` | Swagger handler | Deferred | Not covered by backend tests; generated docs drift is covered separately by generation checks rather than request assertions. | T-20260529-190941-2452, T-20260529-191347-87fb |
| `GET /health` | `healthCheck` | Covered | `apps/core/internal/api/integration_test.go` `TestHealthCheckResponse`, `TestCoreWorkerDiagnosticsDoNotAffectAPIHealth`, `TestCoreDiagnosticsReportsIngestionMetrics`. | None |
| `NoRoute` GET/HEAD Console fallback | `serveConsole` | Partial | No focused backend test was found for embedded SPA fallback behavior. | T-20260529-191339-c514 |
| `NoRoute` non-GET rejection | `serveConsole` | Partial | No focused backend test was found for fallback method rejection. | T-20260529-191339-c514 |

### Public Status Page Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /` | `getCustomDomainStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` `TestStatusPageCustomDomainHostRoutingAndIsolation`, `TestPublicStatusPageHTMLRendersSafeMetadataAndTheme`. | None |
| `GET /feed.atom` | `getCustomDomainStatusPageAtomFeed` | Covered | `apps/core/internal/api/status_page_feed_test.go` `TestStatusPageAtomFeedIncludesOnlyPublishedPublicIncidents`, `TestStatusPageAtomFeedOmitsUnpublishedIncidents`, `TestStatusPageAtomFeedDoesNotExposeDraftPages`. | None |
| `GET /status/:slug/feed.atom` | `getStatusPageAtomFeed` | Covered | `apps/core/internal/api/status_page_feed_test.go`. | None |
| `GET /status/:slug/badge.svg` | `getPublicStatusPageBadge` | Covered | `apps/core/internal/api/status_page_badges_test.go` `TestPublicStatusPageBadgeRendersCurrentStatusAndCacheHeaders`, `TestPublicStatusPageBadgesRequirePublicOrUnlistedPage`. | None |
| `GET /status/:slug/history` | `getPublicStatusPageHistory` | Covered | `apps/core/internal/api/status_page_history_test.go` component and incident history coverage plus `apps/core/internal/api/status_page_public_cache_test.go`. | None |
| `GET /status/:slug/components/:component_id/badge.svg` | `getPublicStatusPageComponentBadge` | Covered | `apps/core/internal/api/status_page_badges_test.go` component badge visibility tests. | None |
| `GET /status/:slug/components/:component_id/uptime` | `getPublicStatusPageComponentUptime` | Covered | `apps/core/internal/api/status_page_history_test.go` `TestPublicStatusPageUptimeShowsUnknownAndNoData`. | None |
| `GET /status/:slug/components/:component_id/history` | `getPublicStatusPageComponentHistory` | Covered | `apps/core/internal/api/status_page_history_test.go` `TestPublicStatusPageComponentHistoryAggregatesAndRedactsInternals`. | None |
| `GET /status/:slug/incidents/:incident_id/history` | `getPublicStatusPageIncidentHistory` | Covered | `apps/core/internal/api/status_page_history_test.go` `TestPublicStatusPageIncidentHistoryRoundsAndRedactsUpdates`; cache coverage in `status_page_public_cache_test.go`. | None |
| `POST /status/:slug/subscribers` | `createPublicStatusPageSubscriber` | Covered | `apps/core/internal/api/status_page_subscribers_test.go` subscription storage, masking, and configured sender tests. | None |
| `GET /status/:slug/subscribers/confirm/:token` | `confirmPublicStatusPageSubscriber` | Covered | `apps/core/internal/api/status_page_subscribers_test.go` token hashing, one-time use, expiry, invalid token, and rate limit tests. | None |
| `GET /status/:slug/subscribers/manage/:token` | `getPublicStatusPageSubscriberPreferences` | Covered | `apps/core/internal/api/status_page_subscribers_test.go` self-service and preference visibility tests. | None |
| `PUT /status/:slug/subscribers/manage/:token` | `updatePublicStatusPageSubscriberPreferences` | Covered | `apps/core/internal/api/status_page_subscribers_test.go` preference update and rate limit tests. | None |
| `POST /status/:slug/subscribers/unsubscribe/:token` | `unsubscribePublicStatusPageSubscriber` | Covered | `apps/core/internal/api/status_page_subscribers_test.go` idempotent unsubscribe test. | None |
| `GET /status/:slug/incidents/:incident_id` | `getPublicStatusPageIncident` | Covered | `apps/core/internal/api/status_pages_test.go` public incident detail flow; cache coverage in `status_page_public_cache_test.go`. | None |
| `GET /status/:slug/incidents` | `listPublicStatusPageIncidents` | Covered | `apps/core/internal/api/status_pages_test.go` public incident list flow; cache coverage in `status_page_public_cache_test.go`. | None |
| `GET /status/:slug` | `getPublicStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow, metadata projection, HTML, custom domain, and privacy tests. | None |

### V1 Public Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `POST /v1/register` | `registerAgent` | Covered | `apps/core/internal/api/integration_test.go` `TestRegisterReportListFlow`, token lifecycle coverage, monitor flow tests. | T-20260529-190857-02c8 for deeper service-level token boundaries |
| `POST /v1/auth/login` | `login` | Covered | `apps/core/internal/api/integration_test.go` `TestLoginRequiresConfiguredFrontendAuth`, `TestLoginReturnsTokenForValidConfiguredCredentials`; `apps/core/internal/api/status_pages_test.go` auth boundary for admin routes. | T-20260529-190857-02c8 |
| `POST /v1/heartbeats/:token` | `receiveHeartbeatSuccess` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestHeartbeatMonitorTokenAndIngestRoutes`; worker missed heartbeat tests in `apps/core/internal/worker/app_test.go`. | None |
| `POST /v1/heartbeats/:token/success` | `receiveHeartbeatSuccess` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestHeartbeatMonitorTokenAndIngestRoutes`. | None |
| `POST /v1/heartbeats/:token/failure` | `receiveHeartbeatFailure` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestHeartbeatMonitorTokenAndIngestRoutes`. | None |

### V1 Frontend Agent And Monitor Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /v1/agents` | `listAgents` | Covered | `apps/core/internal/api/integration_test.go` `TestRegisterReportListFlow`, auth middleware coverage via status page admin auth test. | T-20260529-190828-4b8e |
| `GET /v1/agents/summary` | `getAgentSummary` | Covered | `apps/core/internal/api/integration_test.go` `TestRegisterReportListFlow`. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id` | `getAgentDetail` | Covered | `apps/core/internal/api/integration_test.go` `TestRegisterReportListFlow`, not-found tests. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/health` | `getAgentHealth` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` agent health split and stale-agent tests. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/reports` | `getAgentReports` | Covered | `apps/core/internal/api/integration_test.go` register/report/list flow. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/service-logs` | `listAgentServiceLogs` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentServiceLogBatchFlow`. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/uptime` | `getAgentUptime` | Covered | `apps/core/internal/api/integration_test.go` register/report/list flow; service aggregation in `apps/core/internal/service/report-service_test.go`. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/monitors` | `listMonitors` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` list monitor count, filters, and not-found tests. | T-20260529-190828-4b8e |
| `GET /v1/agents/:id/token/status` | `getAgentTokenStatus` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentTokenLifecycleFlow`. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/token/rotate` | `rotateAgentToken` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentTokenLifecycleFlow`. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/token/revoke` | `revokeAgentToken` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentTokenLifecycleFlow`. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/token/reissue` | `reissueAgentToken` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentTokenLifecycleFlow`. | T-20260529-190857-02c8 |
| `GET /v1/monitors` | `listAllMonitors` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` monitor summary, owner/source, alias, and computed health filter tests. | T-20260529-190828-4b8e |
| `POST /v1/monitors` | `createCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` core monitor lifecycle, unsupported kind, invalid config, target policy, catalog config, and heartbeat tests. | T-20260529-190857-16a8 |
| `GET /v1/monitors/summary` | `getMonitorSummary` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` `TestListAllMonitorsAndSummaryUseDerivedStaleState`. | T-20260529-190828-4b8e |
| `PATCH /v1/monitors/:id` | `updateCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`. | T-20260529-190857-16a8 |
| `DELETE /v1/monitors/:id` | `deleteCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`. | T-20260529-190857-16a8 |
| `GET /v1/monitors/:id/config` | `getCoreMonitorConfig` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`. | T-20260529-190857-16a8 |
| `POST /v1/monitors/:id/pause` | `pauseCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`. | T-20260529-190857-16a8 |
| `POST /v1/monitors/:id/resume` | `resumeCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`. | T-20260529-190857-16a8 |
| `POST /v1/monitors/:id/test` | `testCoreMonitor` | Covered | `apps/core/internal/api/core_monitor_api_test.go` `TestCoreMonitorManagementLifecycle`, target policy tests. | T-20260529-190857-16a8 |
| `GET /v1/monitors/:id` | `getMonitorDetail` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` monitor detail consistency and severity override tests. | T-20260529-190828-4b8e |
| `GET /v1/monitors/:id/uptime` | `getMonitorUptime` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` monitor detail/history flow; `apps/core/internal/service/report-service_test.go`. | T-20260529-190828-4b8e |
| `GET /v1/monitors/:id/history` | `getMonitorHistory` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` monitor history not-found and flow tests. | T-20260529-190828-4b8e |

### V1 Frontend Health Diagnostics Incident Alert Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /v1/health/summary` | `getSystemHealth` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` `TestSystemHealthSeparatesStaleMonitorCounts`. | None |
| `GET /v1/health/issues` | `getHealthIssues` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` health issue and stale monitor scenarios. | None |
| `GET /v1/diagnostics/core` | `getCoreDiagnostics` | Covered | `apps/core/internal/api/integration_test.go` `TestCoreDiagnosticsReportsIngestionMetrics`. | T-20260529-191329-9a02 for direct diagnostics service tests |
| `GET /v1/diagnostics/core-worker` | `getCoreWorkerDiagnostics` | Covered | `apps/core/internal/api/integration_test.go` `TestCoreWorkerDiagnosticsDoNotAffectAPIHealth`; `apps/core/internal/service/worker-diagnostics-service_test.go`. | None |
| `GET /v1/incidents` | `listIncidents` | Covered | `apps/core/internal/api/incident_api_test.go` list, detail, timeline, lifecycle, candidates, maintenance, and incident event tests. | T-20260529-190828-a8e8 |
| `GET /v1/incidents/:id` | `getIncidentDetail` | Covered | `apps/core/internal/api/incident_api_test.go` `TestIncidentDetailAndTimelineEndpoints` and severity override tests. | T-20260529-190828-a8e8 |
| `GET /v1/incidents/:id/timeline` | `getIncidentTimeline` | Covered | `apps/core/internal/api/incident_api_test.go` `TestIncidentDetailAndTimelineEndpoints`. | T-20260529-190828-a8e8 |
| `POST /v1/incidents/:id/acknowledge` | `acknowledgeIncident` | Covered | `apps/core/internal/api/incident_api_test.go` `TestManualIncidentActionsAcknowledgeAndResolve`. | T-20260529-190828-a8e8 |
| `POST /v1/incidents/:id/resolve` | `resolveIncident` | Covered | `apps/core/internal/api/incident_api_test.go` `TestManualIncidentActionsAcknowledgeAndResolve`. | T-20260529-190828-a8e8 |
| `POST /v1/incidents/:id/cover` | `coverIncident` | Covered | `apps/core/internal/api/incident_api_test.go` manual and covered incident suppression tests. | T-20260529-190828-a8e8 |
| `POST /v1/incidents/:id/reopen` | `reopenIncident` | Covered | `apps/core/internal/api/incident_api_test.go` manual incident action tests. | T-20260529-190828-a8e8 |
| `GET /v1/incidents/candidates` | `getIncidentCandidates` | Covered | `apps/core/internal/api/incident_api_test.go` candidate and maintenance suppression tests. | T-20260529-190828-a8e8 |
| `GET /v1/alerts/deliveries` | `listAlertDeliveries` | Covered | `apps/core/internal/api/alert_api_test.go` alert delivery list and filter tests. | T-20260529-190857-bb83 |
| `GET /v1/alerts/channels` | `listAlertChannels` | Covered | `apps/core/internal/api/alert_api_test.go` webhook URL redaction, webhook-only filtering, and channel list tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/channels` | `createAlertChannel` | Covered | `apps/core/internal/api/alert_api_test.go` webhook write and non-webhook rejection tests. | T-20260529-190857-bb83 |
| `PATCH /v1/alerts/channels/:id` | `updateAlertChannel` | Covered | `apps/core/internal/api/alert_api_test.go` webhook update tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/channels/:id/test` | `testAlertChannel` | Covered | `apps/core/internal/api/alert_api_test.go` webhook test endpoint; `apps/core/internal/service/alert-service_test.go` channel tests. | T-20260529-190857-bb83 |
| `DELETE /v1/alerts/channels/:id` | `deleteAlertChannel` | Covered | `apps/core/internal/api/alert_api_test.go` channel write endpoint tests. | T-20260529-190857-bb83 |
| Removed SMTP and email destination routes | Router fallback | Covered | `apps/core/internal/api/alert_api_test.go` `TestRemovedAlertDestinationEndpointsAreUnavailable` asserts removed endpoints return 404. | None |
| `GET /v1/alerts/routes` | `listAlertRoutes` | Covered | `apps/core/internal/api/alert_api_test.go` alert route write and dry-run endpoint tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/routes` | `createAlertRoute` | Covered | `apps/core/internal/api/alert_api_test.go` alert route write tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/routes/dry-run` | `dryRunAlertRoutes` | Covered | `apps/core/internal/api/alert_api_test.go` dry-run tests; `apps/core/internal/service/alert-service_test.go` route dry-run explanations. | T-20260529-190857-bb83 |
| `PATCH /v1/alerts/routes/:id` | `updateAlertRoute` | Covered | `apps/core/internal/api/alert_api_test.go` alert route update tests. | T-20260529-190857-bb83 |
| `DELETE /v1/alerts/routes/:id` | `deleteAlertRoute` | Covered | `apps/core/internal/api/alert_api_test.go` alert route delete tests. | T-20260529-190857-bb83 |
| `GET /v1/alerts/rules` | `listAlertRules` | Covered | `apps/core/internal/api/alert_api_test.go` alert rule write, list, enable/disable, update, dry-run, and delete tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/rules` | `createAlertRule` | Covered | `apps/core/internal/api/alert_api_test.go` rule creation and webhook-only channel validation tests. | T-20260529-190857-bb83 |
| `POST /v1/alerts/rules/dry-run` | `dryRunAlertRules` | Covered | `apps/core/internal/api/alert_api_test.go` rule dry-run test covers rule-named response fields and no side effects. | T-20260529-190857-bb83 |
| `PATCH /v1/alerts/rules/:id` | `updateAlertRule` | Covered | `apps/core/internal/api/alert_api_test.go` update and invalid `event_types` rejection tests. | T-20260529-190857-bb83 |
| `DELETE /v1/alerts/rules/:id` | `deleteAlertRule` | Covered | `apps/core/internal/api/alert_api_test.go` rule delete test. | T-20260529-190857-bb83 |
| `POST /v1/alerts/rules/:id/enable` | `enableAlertRule` | Covered | `apps/core/internal/api/alert_api_test.go` rule enable test. | T-20260529-190857-bb83 |
| `POST /v1/alerts/rules/:id/disable` | `disableAlertRule` | Covered | `apps/core/internal/api/alert_api_test.go` rule disable test. | T-20260529-190857-bb83 |

### V1 Status Page Admin Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /v1/status-pages` | `listStatusPages` | Covered | `apps/core/internal/api/status_pages_test.go` admin API flow and JWT boundary tests. | None |
| `POST /v1/status-pages` | `createStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow, validation, custom domain, metadata, and theme tests. | None |
| `GET /v1/status-pages/:id` | `getStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` admin detail flow. | None |
| `PUT /v1/status-pages/:id` | `updateStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` custom domain conflict and metadata update flow. | None |
| `POST /v1/status-pages/:id/publish` | `publishStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` publish validation and admin flow. | None |
| `POST /v1/status-pages/:id/unpublish` | `unpublishStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow and audit event tests. | None |
| `GET /v1/status-pages/:id/preview` | `previewStatusPage` | Covered | `apps/core/internal/api/status_pages_test.go` preview and safe metadata tests. | None |
| `GET /v1/status-pages/:id/sections` | `listStatusPageSections` | Covered | `apps/core/internal/api/status_pages_test.go` admin detail and empty list assertions. | None |
| `POST /v1/status-pages/:id/sections` | `createStatusPageSection` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow and publish validation tests. | None |
| `PUT /v1/status-pages/:id/sections/:section_id` | `updateStatusPageSection` | Covered | `apps/core/internal/api/status_pages_test.go` admin API flow. | None |
| `GET /v1/status-pages/:id/components` | `listStatusPageComponents` | Covered | `apps/core/internal/api/status_pages_test.go` admin detail and empty list assertions. | None |
| `POST /v1/status-pages/:id/components` | `createStatusPageComponent` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow and publish validation tests. | None |
| `PUT /v1/status-pages/:id/components/:component_id` | `updateStatusPageComponent` | Covered | `apps/core/internal/api/status_pages_test.go` admin API flow. | None |
| `GET /v1/status-pages/:id/components/:component_id/mappings` | `listStatusPageComponentMappings` | Covered | `apps/core/internal/api/status_pages_test.go` admin detail assertions. | None |
| `POST /v1/status-pages/:id/components/:component_id/mappings` | `createStatusPageComponentMapping` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow. | None |
| `PUT /v1/status-pages/:id/components/:component_id/mappings/:mapping_id` | `updateStatusPageComponentMapping` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow. | None |
| `GET /v1/status-pages/:id/incidents` | `listStatusPageIncidents` | Covered | `apps/core/internal/api/status_pages_test.go` admin detail and empty list assertions. | None |
| `GET /v1/status-pages/:id/incidents/suggestions` | `suggestStatusPageIncidentComponents` | Covered | `apps/core/internal/api/status_pages_test.go` monitor and agent suggestion privacy tests. | None |
| `POST /v1/status-pages/:id/incidents` | `createStatusPageIncident` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow. | None |
| `PUT /v1/status-pages/:id/incidents/:incident_id` | `updateStatusPageIncident` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow. | None |
| `POST /v1/status-pages/:id/incidents/:incident_id/updates` | `createStatusPageIncidentUpdate` | Covered | `apps/core/internal/api/status_pages_test.go` admin flow and subscriber fan-out tests. | None |

### V1 Frontend Events Logs Settings Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `GET /v1/events` | `listOrionEvents` | Covered | `apps/core/internal/api/integration_test.go` `TestListOrionEvents`. | T-20260529-190828-a8e8 |
| `GET /v1/logs/service` | `listServiceLogs` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentServiceLogBatchFlow`; service filters in `apps/core/internal/service/service-log-service_test.go`. | None |
| `GET /v1/settings/data-lifecycle` | `getDataLifecycleSettings` | Covered | `apps/core/internal/api/settings_api_test.go` `TestDataLifecycleSettingsFlow`; `apps/core/internal/service/settings-service_test.go`. | T-20260529-190828-4b8e |
| `PUT /v1/settings/data-lifecycle` | `updateDataLifecycleSettings` | Covered | `apps/core/internal/api/settings_api_test.go` `TestDataLifecycleSettingsFlow`; service validation tests. | T-20260529-190828-4b8e |
| `POST /v1/settings/data-lifecycle/actions/rollup` | `runDataLifecycleRollup` | Covered | `apps/core/internal/api/settings_api_test.go` `TestDataLifecycleActionsFlow`; `apps/core/internal/service/rollup-service_test.go`. | T-20260529-190857-bb83 |
| `POST /v1/settings/data-lifecycle/actions/archive` | `runDataLifecycleArchive` | Covered | `apps/core/internal/api/settings_api_test.go` `TestDataLifecycleActionsFlow`; `apps/core/internal/service/archive-service_test.go`. | T-20260529-190857-bb83 |

### V1 Agent-Protected Routes

| Route | Handler | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `POST /v1/agents/:agent_id/register-monitor` | `registerMonitor` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` register monitor ownership and token boundary tests. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/unregister-monitor` | `unregisterMonitor` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` unregister cleanup and route agent ID tests. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/report` | `receiveAgentReport` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` report ingestion, diagnostics, stale reconciliation, and auth ownership tests. | T-20260529-190857-02c8 |
| `POST /v1/agents/:agent_id/logs/batch` | `receiveAgentLogBatch` | Covered | `apps/core/internal/api/integration_test.go` `TestAgentServiceLogBatchFlow`; `apps/core/internal/service/service-log-service_test.go`. | None |
| `POST /v1/agents/:agent_id/:monitor_id/report` | `receiveMonitorReport` | Covered | `apps/core/internal/api/agent_monitor_api_test.go` and `apps/core/internal/api/incident_api_test.go` cover monitor report ownership, history, incident, and diagnostics tests. | T-20260529-190857-02c8 |
| `PUT /v1/agents/:agent_id/maintenance` | `setMaintenanceMode` | Covered | `apps/core/internal/api/incident_api_test.go` maintenance suppression tests. | T-20260529-190857-02c8 |

## Service Coverage Map

| Service | Exported methods | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| `AgentService` | `RegisterAgent`, `ValidateAgentToken`, `AgentTokenStatus`, `RotateAgentToken`, `ReissueAgentToken`, `RevokeAgentToken`, `UpdateLastSeen`, `SetMaintenanceMode`, `GetAgent`, `ListAgents`, `GetAgentCount` | Partial | API integration tests cover the main flows and token lifecycle; no focused `agent-service_test.go` was found. | T-20260529-190857-02c8, T-20260529-191329-9a02 |
| `AuthService` | `ValidateToken` | Partial | Covered through protected route integration tests; no direct service test was found. | T-20260529-190857-02c8, T-20260529-191329-9a02 |
| `MonitorService` | `RegisterMonitor`, `UnregisterMonitor`, `ListMonitors`, `ListAllMonitors`, `GetMonitorSummary`, `GetMonitor`, `GetMonitorCount` | Partial | API integration tests cover monitor flows, ownership, filters, and summaries; no focused `monitor-service_test.go` was found. | T-20260529-190828-4b8e, T-20260529-191329-9a02 |
| `ReportService` | `SetDiagnostics`, `StoreMonitorReport`, `StoreAgentReport`, `GetAgentReportsById`, `GetAgentReportCountById`, `GetMonitorReports`, `GetMonitorReportCount`, `GetMonitorUptime`, `GetAgentUptime` | Partial | Uptime aggregation is directly covered in `apps/core/internal/service/report-service_test.go`; ingestion and diagnostics hooks are mostly covered through API integration. | T-20260529-191329-9a02 |
| `HealthService` | `ComputeMonitorHealth`, `ComputeAgentHealthSnapshot`, `ComputeAgentHealth`, `DetectStaleMonitors` | Covered | `apps/core/internal/service/health-service_test.go` covers stale, flapping, summary, and split availability behavior. | None |
| `IncidentService` | `SetDiagnostics`, `ReconcileMonitorReport`, `ReconcileStaleMonitors`, `AcknowledgeIncident`, `ResolveIncident`, `CoverIncident`, `ReopenIncident`, `ResolveMonitorRemoved` | Partial | API integration tests cover incident reconciliation and lifecycle; no focused `incident-service_test.go` was found. | T-20260529-190828-a8e8, T-20260529-191329-9a02 |
| `CoreMonitorManagementService` | `CreateCoreMonitor`, `UpdateCoreMonitor`, `DeleteCoreMonitor`, `PauseCoreMonitor`, `ResumeCoreMonitor`, `GetCoreMonitorConfig`, `GetHeartbeatMonitorByToken`, `RecordHeartbeatSignal`, `ValidateCoreMonitorConfig` | Covered | `apps/core/internal/service/core-monitor-management-service_test.go` covers validation and target policy; API tests cover lifecycle and heartbeat routes. | T-20260529-190857-16a8 for remaining high-risk target/privacy paths |
| `CoreMonitorSchedulerService` | `ClaimDueCoreMonitorConfigs`, `CompleteCoreMonitorCheck` | Covered | `apps/core/internal/service/core-monitor-scheduler-service_test.go` covers due claims, leases, paused/inactive monitors, heartbeat skips, and completion. | None |
| `AlertService` | `QueueIncidentNotifications`, `TestChannel`, `ProcessDueDeliveries`, `LoadAlertRouteContext`, `DryRunRoutes` | Covered | `apps/core/internal/service/alert-service_test.go` covers queueing, retries, payloads, signatures, grouping, cooldowns, webhook tests, retired delivery suppression, and dry-run behavior. | T-20260529-190857-bb83 for data lifecycle and endpoint split work |
| `SettingsService` | `GetDataLifecycleSettings`, `UpdateDataLifecycleSettings` | Covered | `apps/core/internal/service/settings-service_test.go` and API settings flow tests. | None |
| `DataLifecycleSchedulerService` | `Run`, `RunDue` | Covered | `apps/core/internal/service/data-lifecycle-scheduler-service_test.go` covers daily run, manual mode, and once-per-day behavior. | None |
| `ArchiveService` | `RunRawReportArchive` | Covered | `apps/core/internal/service/archive-service_test.go` covers archival and disabled mode. | T-20260529-190857-bb83 for additional migration/data lifecycle high-risk scenarios |
| `RollupService` | `RunDailyMonitorUptimeRollup`, `RollupMonitorUptimeDay` | Covered | `apps/core/internal/service/rollup-service_test.go` covers daily row creation and idempotency. | None |
| `AuditService` | `RecordStatusPageEvent` | Covered | `apps/core/internal/service/audit-service_test.go` and status page audit API tests. | None |
| `ServiceLogService` | `StoreAgentLogBatch`, `ListServiceLogs` | Covered | `apps/core/internal/service/service-log-service_test.go` and API service log flow. | None |
| `WorkerDiagnosticsService` | `RecordHeartbeat`, `GetDiagnostics` | Covered | `apps/core/internal/service/worker-diagnostics-service_test.go` and core-worker diagnostics API tests. | None |
| `RuntimeDiagnosticsService` | `RecordRequest`, `RecordIngestion`, `RecordReportWrite`, `RecordIncidentReconciliation`, `RecordActiveIncidentLookup`, `RecordDatabaseOperation`, `RecordSlowOperation`, `Snapshot` | Partial | API diagnostics test verifies request, ingestion, report write, reconciliation, lookup, and SQLite metrics; no direct runtime diagnostics service test was found for error/slow-operation boundaries. | T-20260529-191329-9a02 |

## Worker And Operational Job Coverage Map

| Worker or job | Runner kinds or methods | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- | --- |
| Worker app lifecycle | `NewApp`, `Run`, database check, heartbeat, default intervals | Covered | `apps/core/internal/worker/app_test.go` lifecycle and database checks. | None |
| Immediate check entry point | `RunImmediateCheck` | Covered | Core monitor API test-now flow and runner tests exercise immediate execution paths. | T-20260529-190857-16a8 for target-policy edge cases |
| HTTP status and keyword runners | `http`, `http_keyword`, `expected_status` | Covered | `apps/core/internal/worker/app_test.go` success, status mismatch, keyword required/forbidden, invalid config, redirect policy, private target policy. | None |
| TCP runner | `tcp` | Covered | `apps/core/internal/worker/app_test.go` TCP success, refused, blocked host, DNS failure, timeout. | None |
| DNS runner | `dns` | Covered | `apps/core/internal/worker/app_test.go` DNS records, expected value miss, lookup failure. | None |
| Ping runner | `ping` | Covered | `apps/core/internal/worker/ping_runner_test.go` reachable, timeout, permission failure. | None |
| TLS certificate runner | `tls_certificate` | Covered | `apps/core/internal/worker/tls_runner_test.go` healthy, expiring, invalid certificate. | None |
| UDP runner | `udp` | Covered | `apps/core/internal/worker/udp_runner_test.go` response, timeout, mismatch, no-response mode. | None |
| Mail runners | `smtp`, `imap`, `pop` | Covered | `apps/core/internal/worker/mail_runner_test.go` protocol success, missing capability, auth-enabled config, timeout. | None |
| API request runner | `api_request` | Covered | `apps/core/internal/worker/api_request_runner_test.go` success, JSON assertion failure, transport failure. | None |
| Synthetic runner | `synthetic` | Covered | `apps/core/internal/worker/synthetic_runner_test.go` API steps, step failure, assertions, unsupported browser step, variables, private target policy, truncation. | None |
| Playwright transaction runner | `playwright` | Covered | `apps/core/internal/worker/playwright_runner_test.go` success, failure artifact, runtime failure, invalid config, private target policy. | T-20260529-190927-caef for validation package extraction |
| Domain expiration runner | `domain_expiration` | Covered | `apps/core/internal/worker/domain_expiration_runner_test.go` healthy, expiring, RDAP unavailable, WHOIS fallback, total failure. | None |
| Missed heartbeat reconciliation | heartbeat monitors | Covered | `apps/core/internal/worker/app_test.go` pending, grace window, incident open/recovery, failed signal preservation, paused/deleted/non-heartbeat skips. | None |
| Alert delivery retry worker path | `AlertService.ProcessDueDeliveries` | Covered | `apps/core/internal/service/alert-service_test.go` due delivery retry tests. | T-20260529-190857-bb83 |
| Data lifecycle scheduler | daily archive and rollup | Covered | `apps/core/internal/service/data-lifecycle-scheduler-service_test.go`. | None |
| Manual archive and rollup API jobs | settings actions | Covered | `apps/core/internal/api/settings_api_test.go` `TestDataLifecycleActionsFlow`; service archive and rollup tests. | T-20260529-190857-bb83 |
| Core worker CLI startup | `apps/core/cmd/worker/main.go` | Missing | No CLI-level worker startup test was found. | T-20260529-191339-c514 |

## Persistence And Migration Coverage Map

| Area | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- |
| Embedded migration execution | Covered | `apps/core/internal/db/migration_test.go` `TestMigrateAppliesEmbeddedMigrations`. | None |
| Migration version contiguity | Covered | `apps/core/internal/db/migration_test.go` `TestEmbeddedMigrationVersionsAreContiguous`. | None |
| Migration idempotency | Covered | `apps/core/internal/db/migration_test.go` `TestMigrateIsIdempotent`. | None |
| Legacy agent report metadata repair | Covered | `apps/core/internal/db/migration_test.go` `TestMigrateRepairsLegacyAgentReportMetadataColumns`. | None |
| Status page schema and unique indexes | Covered | `apps/core/internal/db/migration_test.go` status page schema and unique index tests; `apps/core/internal/db/status_page_models_test.go`. | None |
| Migrations `000001` through `000024` as a set | Partial | The suite verifies full application, contiguity, idempotency, selected status page indexes, and one legacy repair; it does not assert every migration's defaults/indexes/compatibility independently. | T-20260529-190857-bb83 |
| Alert channel and removed SMTP/email destination migrations | Covered | `apps/core/internal/db/alert_migration_test.go` covers webhook defaults and removed destination schema cleanup; API/service alert tests cover current webhook behavior. | T-20260529-190857-bb83 |
| Agent token lifecycle migration | Partial | Covered through API token lifecycle flow and full migration application; no migration-specific compatibility test was found. | T-20260529-190857-02c8 |
| Core monitor config, worker status, heartbeat, and confirmation migrations | Partial | Covered by Core monitor API, scheduler, worker diagnostics, and full migration application; migration-specific default/index assertions are thin. | T-20260529-190857-16a8, T-20260529-190857-bb83 |
| Incident lifecycle and component impact migrations | Partial | Covered by incident lifecycle/status page tests and full migration application; migration-specific compatibility assertions are thin. | T-20260529-190828-a8e8, T-20260529-190857-bb83 |

## Config Startup Utilities Coverage Map

| Area | Status | Existing coverage | Follow-up |
| --- | --- | --- | --- |
| Config env parsing and validation | Covered | `apps/core/internal/config/config_test.go` covers CORS, public status mail, frontend auth, worker diagnostics, data lifecycle scheduler, and `.env` loading. | None |
| Startup config loading | Partial | `startup.LoadConfig` wraps config loading, but no focused `internal/startup` test was found. | T-20260529-191339-c514 |
| Migrated database open/close startup helpers | Missing | No focused test was found for `startup.OpenMigratedDatabase` or `startup.CloseDatabase`. | T-20260529-191339-c514 |
| API response helpers | Covered | `apps/core/internal/utils/response_test.go`. | None |
| ID and token helpers | Partial | Used throughout API and service tests, but no direct unit test was found for helper shape or entropy/error behavior. | T-20260529-191005-6360 |
| Alert event encode/decode helpers | Partial | Used by alert API/service tests, but no direct model helper test was found. | T-20260529-190857-bb83 |

## Current Highest-Risk Gaps

1. Direct service coverage is thinner than route coverage for `AgentService`, `MonitorService`,
   `IncidentService`, `AuthService`, and `RuntimeDiagnosticsService`.
2. Migration coverage proves global application and selected compatibility, but not every migration's
   default/index/old-row contract.
3. Startup, fallback, route drift, and direct runtime diagnostics boundaries remain the highest
   backend gaps after the alert webhook cleanup and high-risk coverage PRs.

## Verification Commands

Run these for changes that touch the Core backend surface:

```sh
cd apps/core && go test ./...
```

For larger backend cleanup PRs, also run:

```sh
cd apps/core && go test -race ./internal/service ./internal/worker
make generate-openapi
git diff --exit-code -- apps/core/docs apps/core/openapi.yaml apps/console/src/orion-sdk
```
