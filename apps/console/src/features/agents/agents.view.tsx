import { Link } from "react-router-dom";
import { Button, PageHeader } from "@/design-system";
import { AgentList } from "@/features/agents/components/agent-list";

export const AgentsPage = () => {
  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-4">
        <PageHeader title="Servers" description="Installed servers and their current checks." />
        <Link to="/servers/new">
          <Button>Add server</Button>
        </Link>
      </div>
      <AgentList />
    </div>
  );
};
