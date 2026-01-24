import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type Agent, type AgentReport, type Monitor, type UptimeDayBucket } from "../lib/api";
import { formatLastSeen, formatUptime } from "../utils/format";
import { UptimeSLA } from "../components/UptimeSLA";

export function AgentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [latestReport, setLatestReport] = useState<AgentReport | null | undefined>(undefined);
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [reports, setReports] = useState<AgentReport[]>([]);
  const [reportsCount, setReportsCount] = useState(0);
  const [uptime, setUptime] = useState<{ daily_buckets?: UptimeDayBucket[]; uptime_percent?: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reportOffset, setReportOffset] = useState(0);
  const reportPage = 10;

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const [d, mon, rep, up] = await Promise.all([
          api.getAgentDetail(id),
          api.listMonitors(id, { limit: 100 }),
          api.getAgentReports(id, { limit: reportPage, offset: 0 }),
          api.getAgentUptime(id, { period: "90d" }),
        ]);
        setAgent(d.agent ?? null);
        setLatestReport(d.latest_report);
        setMonitors(mon.monitors ?? []);
        setReports(rep.reports ?? []);
        setReportsCount(rep.count ?? 0);
        setUptime(up);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load agent");
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const loadMoreReports = async () => {
    if (!id) return;
    const d = await api.getAgentReports(id, { limit: reportPage, offset: reportOffset + reportPage });
    setReports((r) => [...r, ...(d.reports ?? [])]);
    setReportOffset((o) => o + reportPage);
  };

  if (loading) return <main><p className="muted">Loading…</p></main>;
  if (error) return <main><p className="error">{error}</p></main>;
  if (!agent) return <main><p className="error">Agent not found</p></main>;

  return (
    <main className="detail">
      <nav><Link to="/">← Agents</Link></nav>
      <h1>{agent.name ?? agent.id}</h1>
      <p className="muted">ID: {agent.id} · OS: {agent.os ?? "—"} · Arch: {agent.arch ?? "—"}</p>

      <section>
        <h2>Metadata</h2>
        <pre><code>{JSON.stringify({ agent, latest_report: latestReport }, null, 2)}</code></pre>
      </section>

      {uptime && (uptime.daily_buckets?.length ?? 0) > 0 && (
        <section>
          <h2>Uptime (90d)</h2>
          <UptimeSLA buckets={uptime.daily_buckets ?? []} percent={uptime.uptime_percent} />
        </section>
      )}

      <section>
        <h2>Monitors</h2>
        {monitors.length === 0 ? <p className="muted">No monitors</p> : (
          <table>
            <thead><tr><th>Name</th><th>Type</th><th>Health</th><th>Last seen</th></tr></thead>
            <tbody>
              {monitors.map((m) => (
                <tr key={m.id}>
                  <td><Link to={`/monitors/${m.id}`}>{m.name ?? m.id}</Link></td>
                  <td>{m.type ?? "—"}</td>
                  <td>{m.health ?? m.computed_health ?? "—"}</td>
                  <td>{formatLastSeen(m.last_successful_report_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h2>Agent reports</h2>
        {reports.length === 0 ? <p className="muted">No reports</p> : (
          <>
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Uptime</th>
                  <th>CPU %</th>
                  <th>Mem %</th>
                  <th>Disk %</th>
                </tr>
              </thead>
              <tbody>
                {reports.map((r) => (
                  <tr key={r.id}>
                    <td>{formatLastSeen(r.created_at)}</td>
                    <td>{formatUptime(r.uptime_seconds)}</td>
                    <td>{r.cpu?.usage_percent != null ? r.cpu.usage_percent.toFixed(1) : "—"}</td>
                    <td>{r.memory?.used_percent != null ? r.memory.used_percent.toFixed(1) : "—"}</td>
                    <td>{r.disk?.used_percent != null ? r.disk.used_percent.toFixed(1) : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {reports.length < reportsCount && (
              <button type="button" onClick={loadMoreReports}>Load more</button>
            )}
          </>
        )}
      </section>
    </main>
  );
}
