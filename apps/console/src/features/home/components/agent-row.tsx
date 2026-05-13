import type { ApiAgentResponse } from "@/orion-sdk";
import { useState } from "react";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { MonitorList } from "./monitor-list";

export const AgentRow = ({ agent }: { agent: ApiAgentResponse }) => {
  const [showMonitors, setShowMonitors] = useState(false);

  const handleShowMonitors = () => setShowMonitors((current) => !current);
  const platform = agent.platform ?? agent.os ?? "unknown";
  const monitorCount = agent.monitor_count ?? 0;

  return (
    <div>
      <button
        className="flex w-full cursor-pointer items-center gap-3 py-2 text-left"
        onKeyDown={(e) => e.key === "Enter" && handleShowMonitors()}
        onClick={handleShowMonitors}
        type="button"
      >
        <span>{showMonitors ? "−" : "+"}</span>
        <span className="font-medium">{agent.name ?? agent.id ?? "Unknown server"}</span>
        <span>{platform}</span>
        <span>{monitorCount} monitors</span>
        <span>last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}</span>
        {agent.maintenance_mode && <span>maintenance</span>}
      </button>
      {agent.id && showMonitors && <MonitorList agentId={agent.id} />}
    </div>
  );
};
