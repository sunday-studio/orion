import { useState, Fragment } from "react";
import { Link } from "react-router-dom";
import {
  useListAgents,
  useListMonitors,
  type Agent,
  type ListAgentsStatus,
  type Monitor,
} from "../lib/api";
import { useDebounce } from "../hooks/useDebounce";
import { formatLastSeen, formatUptime, descriptionFromMeta } from "../utils/format";
import { CanvasView } from "../components/canvas-view";

const PAGE = 20;
type ViewMode = "list" | "canvas";

export function HomePage() {
  const [view, setView] = useState<ViewMode>("list");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<"" | ListAgentsStatus>("");
  const [offset, setOffset] = useState(0);
  const [expanded, setExpanded] = useState<string | null>(null);

  const debouncedSearch = useDebounce(search, 300);

  const {
    data: agentsRes,
    isLoading: loading,
    error: agentsError,
  } = useListAgents(
    {
      search: debouncedSearch || undefined,
      status: status || undefined,
      limit: PAGE,
      offset,
    },
    { query: { enabled: view === "list" } }
  );

  const agents = agentsRes?.data?.data?.agents ?? [];
  const count = agentsRes?.data?.data?.count ?? 0;
  const error = agentsError instanceof Error ? agentsError.message : agentsError ? "Failed to load agents" : null;

  const { data: monitorsRes, isFetching: monitorsLoading } = useListMonitors(
    expanded ?? "",
    { limit: 50 },
    { query: { enabled: !!expanded } }
  );
  const expandedMonitors = monitorsRes?.data?.data?.monitors ?? [];

  const toggleExpand = (agentId: string) => {
    setExpanded((e) => (e === agentId ? null : agentId));
  };

  const statusLabel = (a: Agent): string => {
    if (a.maintenance_mode) return "maintenance";
    return "—";
  };

  return (
    <main className="home">
      <h1>Orion</h1>
      <div className="toolbar">
        <div className="view-toggle">
          <button type="button" onClick={() => setView("list")} className={view === "list" ? "active" : ""}>List</button>
          <button type="button" onClick={() => setView("canvas")} className={view === "canvas" ? "active" : ""}>Canvas</button>
        </div>
        {view === "list" && (
          <>
            <input
              type="search"
              placeholder="Search…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              aria-label="Search agents"
            />
            <select value={status} onChange={(e) => { setStatus((e.target.value || "") as "" | ListAgentsStatus); setOffset(0); }} aria-label="Filter by status">
              <option value="">All statuses</option>
              <option value="up">Up</option>
              <option value="down">Down</option>
              <option value="degraded">Degraded</option>
              <option value="unknown">Unknown</option>
            </select>
          </>
        )}
      </div>

      {view === "list" && error && <p className="error">{error}</p>}
      {view === "list" && loading && <p className="muted">Loading…</p>}

      {view === "canvas" && <CanvasView />}

      {view === "list" && !loading && !error && (
        <>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th aria-label="expand" />
                  <th>Name</th>
                  <th>IP</th>
                  <th>Description</th>
                  <th>Status</th>
                  <th>Last seen</th>
                  <th>Uptime</th>
                  <th>Monitors</th>
                </tr>
              </thead>
              <tbody>
                {agents.map((a) => (
                  <Fragment key={a.id}>
                    <tr>
                      <td>
                        <button
                          type="button"
                          onClick={() => a.id && toggleExpand(a.id)}
                          aria-expanded={expanded === a.id}
                          className="expand-btn"
                        >
                          {expanded === a.id ? "−" : "+"}
                        </button>
                      </td>
                      <td>
                        {a.id ? <Link to={`/agents/${a.id}`}>{a.name ?? a.id}</Link> : (a.name ?? "—")}
                      </td>
                      <td>{a.ip ?? (a.location as { ip?: string } | undefined)?.ip ?? "—"}</td>
                      <td>{descriptionFromMeta(a.meta)}</td>
                      <td>{statusLabel(a)}</td>
                      <td>{formatLastSeen(a.last_seen)}</td>
                      <td>{formatUptime(a.uptime_seconds)}</td>
                      <td>{a.monitor_count != null ? a.monitor_count : "—"}</td>
                    </tr>
                    {expanded === a.id && (
                      <tr>
                        <td colSpan={8} className="expanded">
                          <MonitorsSubTable list={expanded === a.id ? expandedMonitors : []} loading={monitorsLoading} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>

          <div className="pagination">
            <button type="button" disabled={offset === 0} onClick={() => setOffset((o) => Math.max(0, o - PAGE))}>
              Previous
            </button>
            <span>
              {offset + 1}–{Math.min(offset + PAGE, count)} of {count}
            </span>
            <button
              type="button"
              disabled={offset + PAGE >= count}
              onClick={() => setOffset((o) => o + PAGE)}
            >
              Next
            </button>
          </div>
        </>
      )}
    </main>
  );
}

function MonitorsSubTable({ list, loading }: { list: Monitor[]; loading: boolean }) {
  if (loading) return <p className="muted">Loading monitors…</p>;
  if (list.length === 0) return <p className="muted">No monitors</p>;
  return (
    <table className="sub-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Type</th>
          <th>Health</th>
          <th>Last seen</th>
        </tr>
      </thead>
      <tbody>
        {list.map((m) => (
          <tr key={m.id}>
            <td>{m.id ? <Link to={`/monitors/${m.id}`}>{m.name ?? m.id}</Link> : (m.name ?? "—")}</td>
            <td>{m.type ?? "—"}</td>
            <td>{m.health ?? m.computed_health ?? "—"}</td>
            <td>{formatLastSeen(m.last_successful_report_at)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
