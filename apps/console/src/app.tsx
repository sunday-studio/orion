import { Routes, Route } from "react-router-dom";
import { HomePage } from "./features/home/home.view";
import { LoginPage } from "./features/auth/login-page";
import { Layout } from "./components/layout";

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Layout/>} >
        <Route index element={<HomePage />} />
      </Route>
    </Routes>
  );
}

export default App;
