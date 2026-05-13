import { Navigate, Routes, Route } from "react-router-dom";
import { LoginPage } from "@/features/auth/login-page";
import { Layout } from "@/components/layout";
import { PlaceholderPage } from "@/components/placeholder-page";
import { ServerDetailPage } from "@/features/server-detail/server-detail.view";
import { MonitorDetailPage } from "@/features/monitor-detail/monitor-detail.view";
import { IncidentsPage } from "@/features/incidents/incidents.view";
import { ServersPage } from "@/features/servers/servers.view";
import { SettingsPage } from "@/features/settings/settings.view";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/incidents" replace />} />
        <Route path="incidents" element={<IncidentsPage />} />
        <Route path="servers" element={<ServersPage />} />
        <Route path="servers/:serverId" element={<ServerDetailPage />} />
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
        <Route
          path="logs"
          element={
            <PlaceholderPage
              title="Logs"
              description="Orion event logs and future service logs."
              operations={["future Orion event log", "future service logs"]}
            />
          }
        />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}

export default App;
