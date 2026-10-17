import { lazy } from "react";
import { Navigate, Route, Routes, useParams } from "react-router-dom";
import { LoginPage } from "@/features/auth/login.view";
import { Layout } from "@/components/shared/layout";
import { NotFound } from "@/components/shared/not-found";
import { RequireAuth } from "@/components/shared/require-auth";

const AgentDetailPage = lazy(() =>
  import("@/features/agent-detail/agent-detail.view").then((module) => ({
    default: module.AgentDetailPage,
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
const AppsPage = lazy(() =>
  import("@/features/apps/apps.view").then((module) => ({ default: module.AppsPage })),
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
  import("@/features/event-log/logs.view").then((module) => ({ default: module.LogsPage })),
);
const AgentsPage = lazy(() =>
  import("@/features/agents/agents.view").then((module) => ({ default: module.AgentsPage })),
);
const SettingsPage = lazy(() =>
  import("@/features/settings/settings.view").then((module) => ({ default: module.SettingsPage })),
);
const MonitorsPage = lazy(() =>
  import("@/features/monitors/monitors.view").then((module) => ({ default: module.MonitorsPage })),
);
const StatusPagesPage = lazy(() =>
  import("@/features/status-pages/status-pages.view").then((module) => ({
    default: module.StatusPagesPage,
  })),
);

const AgentDetailRedirect = () => {
  const { agentId = "" } = useParams();
  return <Navigate to={`/servers/${agentId}`} replace />;
};

function AppRouter() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Layout />}>
          <Route index element={<OverviewPage />} />
          <Route path="overview" element={<Navigate to="/" replace />} />
          <Route path="incidents" element={<IncidentsPage />} />
          <Route path="alerts" element={<AlertsPage />} />
          <Route path="status-pages" element={<StatusPagesPage />} />
          <Route path="logs" element={<LogsPage />} />
          <Route path="agents" element={<Navigate to="/servers" replace />} />
          <Route path="agents/:agentId" element={<AgentDetailRedirect />} />
          <Route path="servers" element={<AgentsPage />} />
          <Route path="servers/:agentId" element={<AgentDetailPage />} />
          <Route path="monitors" element={<MonitorsPage />} />
          <Route path="monitors/:monitorId" element={<MonitorDetailPage />} />
          <Route path="incidents/:incidentId" element={<IncidentDetailPage />} />
          <Route path="design-system" element={<DesignSystemPage />} />
          <Route path="settings/apps" element={<AppsPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
    </Routes>
  );
}

export default AppRouter;
