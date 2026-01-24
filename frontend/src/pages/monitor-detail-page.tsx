import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  useGetMonitorDetail,
  useGetMonitorHistory,
  useGetMonitorUptime,
  getMonitorHistory,
  type MonitorReport,
  type GetMonitorDetailResponseData,
  type GetMonitorHistoryResponseData,
  type GetUptimeResponseData,
} from "../lib/api";
import { dataOf } from "../lib/custom-instance";
import { formatLastSeen } from "../utils/format";
import { UptimeSLA } from "../components/uptime-sla";

const historyPage = 10;

export function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [extraHistory, setExtraHistory] = useState<MonitorReport[]>([]);
  const [historyOffset, setHistoryOffset] = useState(10);

  const { data: detailRes, isLoading: loading, error: detailError } = useGetMonitorDetail(id ?? "", { query: { enabled: !!id } });
  const { data: historyRes } = useGetMonitorHistory(id ?? "", { limit: historyPage, offset: 0 }, { query: { enabled: !!id } });
  const { data: uptimeRes } = useGetMonitorUptime(id ?? "", { period: "90d" }, { query: { enabled: !!id } });

  const detail = dataOf<GetMonitorDetailResponseData>(detailRes);
  const monitor = detail?.monitor ?? null;
  const recentReports = detail?.recent_reports ?? [];
  const firstPageHistory = dataOf<GetMonitorHistoryResponseData>(historyRes)?.reports ?? [];
  const historyCount = dataOf<GetMonitorHistoryResponseData>(historyRes)?.count ?? 0;
  const uptimeData = dataOf<GetUptimeResponseData>(uptimeRes);
  const uptime = uptimeData ? { daily_buckets: uptimeData.daily_buckets ?? [], uptime_percent: uptimeData.uptime_percent } : null;

  const history = [...firstPageHistory, ...extraHistory];
  const error = detailError instanceof Error ? detailError.message : detailError ? "Failed to load monitor" : null;

  useEffect(() => {
    setExtraHistory([]);
    setHistoryOffset(10);
  }, [id]);

  const loadMoreHistory = async () => {
    if (!id) return;
    const d = await getMonitorHistory(id, { limit: historyPage, offset: historyOffset });
    const payload = dataOf<GetMonitorHistoryResponseData>(d)?.reports ?? [];
    setExtraHistory((h) => [...h, ...payload]);
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
              <thead><tr><th>Collected</th><th>Health</th><th>Payload</th></tr></thead>
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
