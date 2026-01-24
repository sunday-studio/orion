import { Routes, Route } from "react-router-dom";
import { HomePage } from "./pages/HomePage";
import { AgentDetailPage } from "./pages/AgentDetailPage";
import { MonitorDetailPage } from "./pages/MonitorDetailPage";
import { LoginPage } from "./pages/LoginPage";
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
