import { lazy } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { LoginPage } from "@/features/auth/login.view";
import { Layout } from "@/components/shared/layout";
import { NotFound } from "@/components/shared/not-found";
import { RequireAuth } from "@/components/shared/require-auth";
import { resolveLegacyRedirect } from "./legacy-redirects";

const NodeDetailPage = lazy(() =>
  import("@/features/node-detail/node-detail.view").then((module) => ({
    default: module.NodeDetailPage,
  })),
);
const MonitorDetailPage = lazy(() =>
  import("@/features/monitor-detail/monitor-detail.view").then((module) => ({
    default: module.MonitorDetailPage,
  })),
);
const IncidentDetailPage = lazy(() =>
  import("@/features/incidents/incident-detail.view").then((module) => ({
    default: module.IncidentDetailPage,
  })),
);
const DesignSystemPage = lazy(() =>
  import("@/design-system/preview/preview-page").then((module) => ({
    default: module.DesignSystemPage,
  })),
);
const AppDetailPage = lazy(() =>
  import("@/features/apps/app-detail.view").then((module) => ({
    default: module.AppDetailPage,
  })),
);
const HealthIssuesPage = lazy(() =>
  import("@/features/health/health-issues.view").then((module) => ({
    default: module.HealthIssuesPage,
  })),
);
const NodeMonitorSetupPage = lazy(() =>
  import("@/features/monitors/node-setup/node-monitor-setup.view").then((module) => ({
    default: module.NodeMonitorSetupPage,
  })),
);
const MonitorEditPage = lazy(() =>
  import("@/features/monitor-detail/edit/monitor-edit.view").then((module) => ({
    default: module.MonitorEditPage,
  })),
);
const ChannelDetailPage = lazy(() =>
  import("@/features/alerts/channels/channel-detail.view").then((module) => ({
    default: module.ChannelDetailPage,
  })),
);
const DeliveryDetailPage = lazy(() =>
  import("@/features/alerts/deliveries/delivery-detail.view").then((module) => ({
    default: module.DeliveryDetailPage,
  })),
);
const IncidentTriagePage = lazy(() =>
  import("@/features/incidents/incident-triage.view").then((module) => ({
    default: module.IncidentTriagePage,
  })),
);
const AddServerPage = lazy(() =>
  import("@/features/nodes/add-server/add-server.view").then((module) => ({
    default: module.AddServerPage,
  })),
);
const ConnectAppPage = lazy(() =>
  import("@/features/apps/connect/connect-app.view").then((module) => ({
    default: module.ConnectAppPage,
  })),
);
const OverviewPage = lazy(() =>
  import("@/features/overview/overview.view").then((module) => ({ default: module.OverviewPage })),
);
const IncidentsPage = lazy(() =>
  import("@/features/incidents/incidents.view").then((module) => ({
    default: module.IncidentsPage,
  })),
);
const AlertsPage = lazy(() =>
  import("@/features/alerts/alerts.view").then((module) => ({ default: module.AlertsPage })),
);
const LogsPage = lazy(() =>
  import("@/features/logs/logs.view").then((module) => ({ default: module.LogsPage })),
);
const NodesPage = lazy(() =>
  import("@/features/nodes/nodes.view").then((module) => ({ default: module.NodesPage })),
);
const AppsPage = lazy(() =>
  import("@/features/apps/apps-list.view").then((module) => ({ default: module.AppsPage })),
);
const SettingsPage = lazy(() =>
  import("@/features/settings/settings.view").then((module) => ({ default: module.SettingsPage })),
);
const MonitorsPage = lazy(() =>
  import("@/features/monitors/monitors.view").then((module) => ({ default: module.MonitorsPage })),
);
const StatusPageDetailPage = lazy(() =>
  import("@/features/status-pages/status-page-detail.view").then((module) => ({
    default: module.StatusPageDetailPage,
  })),
);
const StatusPagesPage = lazy(() =>
  import("@/features/status-pages/status-pages.view").then((module) => ({
    default: module.StatusPagesPage,
  })),
);

function AppRouter() {
  // Old URLs (Settings tabs, /apps/new, /alerts?tab=...) are rewritten before routing.
  const { pathname, search } = useLocation();
  const legacy = resolveLegacyRedirect(pathname, search);
  if (legacy) return <Navigate to={legacy} replace />;
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Layout />}>
          <Route index element={<OverviewPage />} />
          <Route path="overview" element={<Navigate to="/" replace />} />
          <Route path="incidents" element={<IncidentsPage />} />
          <Route path="alerts" element={<AlertsPage tab="logs" />} />
          <Route path="alerts/channels" element={<AlertsPage tab="channels" />} />
          <Route path="alerts/rules" element={<AlertsPage tab="rules" />} />
          <Route path="alerts/routes" element={<AlertsPage tab="routes" />} />
          <Route path="status-pages" element={<StatusPagesPage />} />
          <Route path="status-pages/:pageId" element={<StatusPageDetailPage />} />
          <Route path="logs" element={<LogsPage />} />
          <Route path="servers" element={<NodesPage />} />
          <Route path="servers/new" element={<AddServerPage />} />
          <Route path="servers/:nodeId" element={<NodeDetailPage />} />
          <Route path="monitors" element={<MonitorsPage />} />
          <Route path="monitors/new/node" element={<NodeMonitorSetupPage />} />
          <Route path="monitors/:monitorId" element={<MonitorDetailPage />} />
          <Route path="monitors/:monitorId/edit" element={<MonitorEditPage />} />
          <Route path="health-issues" element={<HealthIssuesPage />} />
          <Route path="incidents/triage" element={<IncidentTriagePage />} />
          <Route path="incidents/:incidentId" element={<IncidentDetailPage />} />
          <Route path="alerts/channels/:channelId" element={<ChannelDetailPage />} />
          <Route path="alerts/deliveries/:deliveryId" element={<DeliveryDetailPage />} />
          <Route path="design-system" element={<DesignSystemPage />} />
          <Route path="logs/apps" element={<AppsPage />} />
          <Route path="logs/apps/new" element={<ConnectAppPage />} />
          <Route path="logs/apps/:appId" element={<AppDetailPage />} />
          <Route path="settings/general" element={<SettingsPage section="general" />} />
          <Route path="settings/lifecycle" element={<SettingsPage section="lifecycle" />} />
          <Route path="settings/security" element={<SettingsPage section="security" />} />
          <Route path="settings/system" element={<SettingsPage section="system" />} />
          <Route path="settings/audit" element={<SettingsPage section="audit" />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
    </Routes>
  );
}

export default AppRouter;
