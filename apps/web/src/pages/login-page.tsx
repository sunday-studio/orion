import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { authLogin } from "../lib/custom-instance";

export function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await authLogin(username, password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ maxWidth: 360, margin: "4rem auto", padding: 24 }}>
      <h1 style={{ marginBottom: 24 }}>Orion</h1>
      <form onSubmit={handleSubmit}>
        <div style={{ marginBottom: 16 }}>
          <label htmlFor="username" style={{ display: "block", marginBottom: 4 }}>
            Username
          </label>
          <input
            id="username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
            style={{ width: "100%", padding: 8, boxSizing: "border-box" }}
          />
        </div>
        <div style={{ marginBottom: 16 }}>
          <label htmlFor="password" style={{ display: "block", marginBottom: 4 }}>
            Password
          </label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
            style={{ width: "100%", padding: 8, boxSizing: "border-box" }}
          />
        </div>
        {error && <p style={{ color: "crimson", marginBottom: 16 }}>{error}</p>}
        <button type="submit" disabled={loading} style={{ padding: "8px 16px" }}>
          {loading ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
