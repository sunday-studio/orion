import { useGetAgents } from "@/orion-sdk";
import { AgentRow } from "./agent-row";
import { Separator } from "@/components/ui/separator";
import { Fragment } from "react/jsx-runtime";

export const AgentList = () => {
  const agentsResponse = useGetAgents();
  const agents = agentsResponse.data?.agents ?? [];

  return (
    <div>
      {agents.map((agent, index) => (
        <Fragment key={agent.id}>
          <AgentRow key={agent.id} agent={agent} />
          {index < agents.length - 1 && <Separator />}{" "}
        </Fragment>
      ))}
    </div>
  );
};
