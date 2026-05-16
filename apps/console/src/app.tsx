import { Navigate, Route, Routes } from "react-router-dom";
import { LoginPage } from "@/features/auth/login-page";
import { Layout } from "@/components/layout";
import { AgentDetailPage } from "@/features/agent-detail/agent-detail.view";
import { MonitorDetailPage } from "@/features/monitor-detail/monitor-detail.view";
import { IncidentDetailPage } from "@/features/incidents/incident-detail.view";
import { IncidentsPage } from "@/features/incidents/incidents.view";
import { AlertsPage } from "@/features/alerts/alerts.view";
import { LogsPage } from "@/features/event-log/logs.view";
import { AgentsPage } from "@/features/agents/agents.view";
import { SettingsPage } from "@/features/settings/settings.view";
import { MonitorsPage } from "@/features/monitors/monitors.view";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/incidents" replace />} />
        <Route path="incidents" element={<IncidentsPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="logs" element={<LogsPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="agents/:agentId" element={<AgentDetailPage />} />
        <Route path="servers" element={<Navigate to="/agents" replace />} />
        <Route path="servers/:agentId" element={<AgentDetailPage />} />
        <Route path="monitors" element={<MonitorsPage />} />
        <Route path="monitors/:monitorId" element={<MonitorDetailPage />} />
        <Route path="incidents/:incidentId" element={<IncidentDetailPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;
