import { Routes, Route } from "react-router-dom";
import { HomePage } from "./pages/home-page";
import { AgentDetailPage } from "./pages/agent-detail-page";
import { MonitorDetailPage } from "./pages/monitor-detail-page";
import { LoginPage } from "./pages/login-page";
import "./app.css";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<HomePage />} />
      <Route path="/agents/:id" element={<AgentDetailPage />} />
      <Route path="/monitors/:id" element={<MonitorDetailPage />} />
    </Routes>
  );
}

export default App;
