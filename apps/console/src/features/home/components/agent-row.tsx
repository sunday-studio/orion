import type { ApiAgentResponse } from "@/orion-sdk";
import { useState } from "react";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { MonitorList } from "./monitor-list";

export const AgentRow = ({ agent }: { agent: ApiAgentResponse }) => {
  const [showMonitors, setShowMonitors] = useState(false);

  const handleShowMonitors = () => setShowMonitors((current) => !current);
  const platform = agent.platform ?? agent.os ?? "unknown";
  const monitorCount = agent.monitor_count ?? 0;
  const status = agent.status ?? (agent.maintenance_mode ? "maintenance" : "unknown");

  return (
    <div>
      <button
        className="grid w-full cursor-pointer grid-cols-[1rem_minmax(0,1fr)_auto] items-center gap-3 py-2 text-left text-sm sm:grid-cols-[1rem_minmax(0,1.4fr)_7rem_7rem_minmax(0,1fr)_auto]"
        onKeyDown={(e) => e.key === "Enter" && handleShowMonitors()}
        onClick={handleShowMonitors}
        type="button"
      >
        <span>{showMonitors ? "−" : "+"}</span>
        <span className="truncate font-medium">{agent.name ?? agent.id ?? "Unknown server"}</span>
        <span className="hidden sm:inline">{status}</span>
        <span className="hidden sm:inline">{platform}</span>
        <span className="hidden truncate sm:inline">
          last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
        </span>
        <span className="text-right text-neutral-600">{monitorCount} monitors</span>
      </button>
      {agent.id && showMonitors && <MonitorList agentId={agent.id} />}
    </div>
  );
};
