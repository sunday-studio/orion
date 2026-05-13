import type { ApiAgentResponse } from "@/orion-sdk";
import { MonitorList } from "./monitor-list";

export const AgentRow = ({ agent }: { agent: ApiAgentResponse }) => {
  return (
    <div>
      <div className="flex gap-2">
        <span className="font-medium">{agent.name ?? agent.id}</span>
        <span>{agent.os}</span>
        <span>{agent.arch}</span>
        <span>{agent.monitor_count} monitors</span>
        <span>{agent.uptime_seconds} uptime</span>
      </div>
      {agent.id && <MonitorList agentId={agent.id} />}
    </div>
  );
};
