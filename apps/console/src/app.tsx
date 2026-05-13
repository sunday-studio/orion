import { Routes, Route } from "react-router-dom";
import { HomePage } from "@/features/home/home.view";
import { LoginPage } from "@/features/auth/login-page";
import { Layout } from "@/components/layout";
import { PlaceholderPage } from "@/components/placeholder-page";
import { ServerDetailPage } from "@/features/server-detail/server-detail.view";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="servers/:serverId" element={<ServerDetailPage />} />
        <Route
          path="monitors/:monitorId"
          element={
            <PlaceholderPage
              title="Monitor Detail"
              description="Current result, check history, and configuration for one monitor."
              operations={["getMonitor", "getMonitorHistory", "getIncidents"]}
            />
          }
        />
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
        <Route
          path="settings"
          element={
            <PlaceholderPage
              title="Settings"
              description="Core-owned settings and setup references."
              operations={[
                "getDataLifecycleSettings",
                "updateDataLifecycleSettings",
                "runDataLifecycleRollup",
                "runDataLifecycleArchive",
              ]}
            />
          }
        />
      </Route>
    </Routes>
  );
}

export default App;
