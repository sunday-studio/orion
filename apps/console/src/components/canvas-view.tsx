import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  listAgents,
  listMonitors,
  type Agent,
  type Monitor,
  type ListAgentsResponseData,
  type ListMonitorsResponseData,
} from "../lib/api";
import { dataOf } from "../lib/custom-instance";
import { formatLastSeen } from "../utils/format";

type TreeAgent = { agent: Agent; monitors: Monitor[] };

export function CanvasView() {
  const { data: nodes = [], isLoading: loading, error: queryError } = useQuery({
    queryKey: ["canvas-tree"],
    queryFn: async (): Promise<TreeAgent[]> => {
      const d = await listAgents({ limit: 100 });
      const agents = dataOf<ListAgentsResponseData>(d)?.agents ?? [];
      return Promise.all(
        agents.map(async (a: Agent) => {
          if (!a.id) return { agent: a, monitors: [] };
          try {
            const m = await listMonitors(a.id, { limit: 50 });
            return { agent: a, monitors: dataOf<ListMonitorsResponseData>(m)?.monitors ?? [] };
          } catch {
            return { agent: a, monitors: [] };
          }
        })
      );
    },
  });

  const error = queryError instanceof Error ? queryError.message : queryError ? "Failed to load tree" : null;

  if (loading) return <p className="muted">Loading…</p>;
  if (error) return <p className="error">{error}</p>;

  return (
    <div className="canvas-tree">
      <div className="tree-root">Agents</div>
      {nodes.map(({ agent, monitors }) => (
        <div key={agent.id} className="tree-branch">
          <Link to={`/agents/${agent.id}`} className="tree-node tree-agent">
            <span className="name">{agent.name ?? agent.id}</span>
            <span className="meta">{formatLastSeen(agent.last_seen)} · {monitors.length} monitors</span>
          </Link>
          <div className="tree-children">
            {monitors.map((m) => (
              <Link key={m.id} to={`/monitors/${m.id}`} className="tree-node tree-monitor">
                <span className="name">{m.name ?? m.id}</span>
                <span className="meta">{m.type ?? "—"} · {m.health ?? m.computed_health ?? "—"}</span>
              </Link>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
