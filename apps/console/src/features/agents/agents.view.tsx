import { Plus } from "lucide-react";
import { Link } from "react-router-dom";
import { Button, PageHeader } from "@/design-system";
import { AgentList } from "@/features/agents/components/agent-list";
import { useGetAgentSummary } from "@/orion-sdk";
import { pluralize } from "@/utils/plural";

export const AgentsPage = () => {
  const summary = useGetAgentSummary();
  const total = summary.data?.total;
  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Servers"
        description={!total ? undefined : `${pluralize(total, "agent")} connected to Core.`}
        actions={
          <Link to="/servers/new">
            <Button size="sm">
              <Plus /> Add server
            </Button>
          </Link>
        }
      />
      <AgentList />
    </div>
  );
};
