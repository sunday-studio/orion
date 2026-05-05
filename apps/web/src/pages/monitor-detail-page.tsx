import { useInfiniteQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import {
  useGetMonitorDetail,
  useGetMonitorUptime,
  getMonitorHistory,
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

  const { data: detailRes, isLoading: loading, error: detailError } = useGetMonitorDetail(id ?? "", { query: { enabled: !!id } });
  const {
    data: historyData,
    fetchNextPage: fetchMoreHistory,
    hasNextPage: hasMoreHistory,
    isFetchingNextPage: loadingMoreHistory,
  } = useInfiniteQuery({
    queryKey: ["/monitors", id, "history", { limit: historyPage }],
    queryFn: ({ pageParam }) => getMonitorHistory(id!, { limit: historyPage, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const d = dataOf<GetMonitorHistoryResponseData>(lastPage);
      const count = d?.count ?? 0;
      const loaded = allPages.reduce((s, p) => s + (dataOf<GetMonitorHistoryResponseData>(p)?.reports?.length ?? 0), 0);
      return loaded < count ? loaded : undefined;
    },
    enabled: !!id,
  });
  const { data: uptimeRes } = useGetMonitorUptime(id ?? "", { period: "90d" }, { query: { enabled: !!id } });

  const detail = dataOf<GetMonitorDetailResponseData>(detailRes);
  const monitor = detail?.monitor ?? null;
  const recentReports = detail?.recent_reports ?? [];
  const history = historyData?.pages?.flatMap((p) => dataOf<GetMonitorHistoryResponseData>(p)?.reports ?? []) ?? [];
  const uptimeData = dataOf<GetUptimeResponseData>(uptimeRes);
  const uptime = uptimeData ? { daily_buckets: uptimeData.daily_buckets ?? [], uptime_percent: uptimeData.uptime_percent } : null;

  const error = detailError instanceof Error ? detailError.message : detailError ? "Failed to load monitor" : null;

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
            {hasMoreHistory && (
              <button type="button" onClick={() => fetchMoreHistory()} disabled={loadingMoreHistory}>
                {loadingMoreHistory ? "Loading…" : "Load more"}
              </button>
            )}
          </>
        )}
      </section>
    </main>
  );
}
