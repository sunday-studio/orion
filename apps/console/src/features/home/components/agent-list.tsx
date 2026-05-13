import { useGetAgents } from "@/orion-sdk";
import { AgentRow } from "./agent-row";

export const AgentList = () => {
  const agentsResponse = useGetAgents();
  const agents = agentsResponse.data?.agents ?? [];

  return (
    <div>
      {agents.map((agent) => (
        <AgentRow key={agent.id} agent={agent} />
      ))}
    </div>
  );
};
