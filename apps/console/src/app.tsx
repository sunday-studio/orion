import { Navigate, Routes, Route } from "react-router-dom";
import { LoginPage } from "@/features/auth/login-page";
import { Layout } from "@/components/layout";
import { PlaceholderPage } from "@/components/placeholder-page";
import { AgentDetailPage } from "@/features/server-detail/server-detail.view";
import { MonitorDetailPage } from "@/features/monitor-detail/monitor-detail.view";
import { IncidentsPage } from "@/features/incidents/incidents.view";
import { AgentsPage } from "@/features/servers/servers.view";
import { SettingsPage } from "@/features/settings/settings.view";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/incidents" replace />} />
        <Route path="incidents" element={<IncidentsPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="agents/:agentId" element={<AgentDetailPage />} />
        <Route path="servers" element={<Navigate to="/agents" replace />} />
        <Route path="servers/:serverId" element={<AgentDetailPage />} />
        <Route path="monitors/:monitorId" element={<MonitorDetailPage />} />
        <Route
          path="incidents/:incidentId"
          element={
            <PlaceholderPage
              title="Incident Detail"
              description="Timeline and linked data for one operational event."
              operations={["getIncidents", "future incident detail", "future incident events"]}
            />
          }
        />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;
