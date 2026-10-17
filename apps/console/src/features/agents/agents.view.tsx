import { PageHeader } from "@/design-system";
import { AgentList } from "@/features/agents/components/agent-list";

export const AgentsPage = () => {
  return (
    <div className="space-y-4">
      <PageHeader title="Servers" description="Installed servers and their current checks." />
      <AgentList />
    </div>
  );
};
