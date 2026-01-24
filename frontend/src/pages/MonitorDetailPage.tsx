import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type Monitor, type MonitorReport } from "../lib/api";
import { formatLastSeen } from "../utils/format";
import { UptimeSLA } from "../components/UptimeSLA";

export function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [monitor, setMonitor] = useState<Monitor | null>(null);
  const [recentReports, setRecentReports] = useState<MonitorReport[]>([]);
  const [history, setHistory] = useState<MonitorReport[]>([]);
  const [historyCount, setHistoryCount] = useState(0);
  const [uptime, setUptime] = useState<{ daily_buckets?: { date?: string; uptime_percent?: number; total?: number }[]; uptime_percent?: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [historyOffset, setHistoryOffset] = useState(0);
  const historyPage = 10;

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const [d, hist, up] = await Promise.all([
          api.getMonitorDetail(id),
          api.getMonitorHistory(id, { limit: historyPage, offset: 0 }),
          api.getMonitorUptime(id, { period: "90d" }),
        ]);
        setMonitor(d.monitor ?? null);
        setRecentReports(d.recent_reports ?? []);
        setHistory(hist.reports ?? []);
        setHistoryCount(hist.count ?? 0);
        setUptime(up);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load monitor");
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const loadMoreHistory = async () => {
    if (!id) return;
    const d = await api.getMonitorHistory(id, { limit: historyPage, offset: historyOffset + historyPage });
    setHistory((h) => [...h, ...(d.reports ?? [])]);
    setHistoryOffset((o) => o + historyPage);
  };

  if (loading) return <main><p className="muted">Loading…</p></main>;
  if (error) return <main><p className="error">{error}</p></main>;
  if (!monitor) return <main><p className="error">Monitor not found</p></main>;

  const allReports = recentReports.length > 0 ? recentReports : history;

  return (
    <main className="detail">
      <nav><Link to="/">← Agents</Link> {monitor.agent_id && <>· <Link to={`/agents/${monitor.agent_id}`}>Agent</Link></>}</nav>
      <h1>{monitor.name ?? monitor.id}</h1>
      <p className="muted">ID: {monitor.id} · Type: {monitor.type ?? "—"} · Health: {monitor.health ?? monitor.computed_health ?? "—"}</p>
      {monitor.description && <p>{monitor.description}</p>}

      <section>
        <h2>Metadata</h2>
        <pre><code>{JSON.stringify({ monitor, recent_reports: allReports.slice(0, 3) }, null, 2)}</code></pre>
      </section>

      {uptime && (uptime.daily_buckets?.length ?? 0) > 0 && (
        <section>
          <h2>Uptime (90d)</h2>
          <UptimeSLA buckets={uptime.daily_buckets ?? []} percent={uptime.uptime_percent} />
        </section>
      )}

      <section>
        <h2>Monitor reports</h2>
        {history.length === 0 ? <p className="muted">No reports</p> : (
          <>
            <table>
              <thead>
                <tr><th>Collected</th><th>Health</th><th>Payload</th></tr>
              </thead>
              <tbody>
                {history.map((r) => (
                  <tr key={r.id}>
                    <td>{formatLastSeen(r.collected_at || r.created_at)}</td>
                    <td>{r.health ?? "—"}</td>
                    <td><code className="payload">{r.payload ? (r.payload.slice(0, 80) + (r.payload.length > 80 ? "…" : "")) : "—"}</code></td>
                  </tr>
                ))}
              </tbody>
            </table>
            {history.length < historyCount && (
              <button type="button" onClick={loadMoreHistory}>Load more</button>
            )}
          </>
        )}
      </section>
    </main>
  );
}
