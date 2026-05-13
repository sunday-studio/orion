import { Routes, Route } from "react-router-dom";
import { HomePage } from "@/features/home/home.view";
import { LoginPage } from "@/features/auth/login-page";
import { Layout } from "@/components/layout";
import { PlaceholderPage } from "@/components/placeholder-page";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route
          path="servers"
          element={
            <PlaceholderPage
              title="Servers"
              description="Inventory and comparison across all monitored servers."
              operations={["getAgents", "getAgent", "getAgentHealth", "getAgentMonitors"]}
            />
          }
        />
        <Route
          path="incidents"
          element={
            <PlaceholderPage
              title="Incidents"
              description="Operational history of things that broke or needed attention."
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
