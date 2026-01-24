import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { listAgents, listMonitors, type Agent, type Monitor } from "../lib/api";
import { dataOf } from "../lib/custom-instance";
import { formatLastSeen } from "../utils/format";

type TreeAgent = { agent: Agent; monitors: Monitor[] };

export function CanvasView() {
  const [nodes, setNodes] = useState<TreeAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchTree = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const d = await listAgents({ limit: 100 });
      const agents = dataOf(d)?.agents ?? [];
      const withMonitors: TreeAgent[] = await Promise.all(
        agents.map(async (a) => {
          if (!a.id) return { agent: a, monitors: [] };
          try {
            const m = await listMonitors(a.id, { limit: 50 });
            return { agent: a, monitors: dataOf(m)?.monitors ?? [] };
          } catch {
            return { agent: a, monitors: [] };
          }
        })
      );
      setNodes(withMonitors);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load tree");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void fetchTree(); }, [fetchTree]);

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
