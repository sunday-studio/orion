import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  useGetAgentDetail,
  useListMonitors,
  useGetAgentReports,
  useGetAgentUptime,
  getAgentReports,
  type AgentReport,
} from "../lib/api";
import { formatLastSeen, formatUptime } from "../utils/format";
import { UptimeSLA } from "../components/uptime-sla";

const reportPage = 10;

export function AgentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [extraReports, setExtraReports] = useState<AgentReport[]>([]);
  const [reportOffset, setReportOffset] = useState(10);

  const { data: detailRes, isLoading: loading, error: detailError } = useGetAgentDetail(id ?? "", { query: { enabled: !!id } });
  const { data: monitorsRes } = useListMonitors(id ?? "", { limit: 100 }, { query: { enabled: !!id } });
  const { data: reportsRes } = useGetAgentReports(id ?? "", { limit: reportPage, offset: 0 }, { query: { enabled: !!id } });
  const { data: uptimeRes } = useGetAgentUptime(id ?? "", { period: "90d" }, { query: { enabled: !!id } });

  const agent = detailRes?.data?.data?.agent ?? null;
  const latestReport = detailRes?.data?.data?.latest_report;
  const monitors = monitorsRes?.data?.data?.monitors ?? [];
  const firstPageReports = reportsRes?.data?.data?.reports ?? [];
  const reportsCount = reportsRes?.data?.data?.count ?? 0;
  const uptime = uptimeRes?.data?.data
    ? { daily_buckets: uptimeRes.data.data.daily_buckets, uptime_percent: uptimeRes.data.data.uptime_percent }
    : null;

  const reports = [...firstPageReports, ...extraReports];
  const error = detailError instanceof Error ? detailError.message : detailError ? "Failed to load agent" : null;

  useEffect(() => {
    setExtraReports([]);
    setReportOffset(10);
  }, [id]);

  const loadMoreReports = async () => {
    if (!id) return;
    const d = await getAgentReports(id, { limit: reportPage, offset: reportOffset });
    const payload = d?.data?.data?.reports ?? [];
    setExtraReports((r) => [...r, ...payload]);
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
