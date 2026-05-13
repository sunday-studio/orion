import { Routes, Route } from "react-router-dom";
import { HomePage } from "./features/home/home.view";
import { LoginPage } from "./features/auth/login-page";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<HomePage />} />
    </Routes>
  );
}

export default App;
