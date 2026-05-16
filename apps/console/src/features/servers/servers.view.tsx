import { PageHeader } from "@/components/page-header";
import { AgentList } from "@/features/servers/components/agent-list";

export const AgentsPage = () => {
  return (
    <div className="space-y-4">
      <PageHeader title="Agents" description="Installed agents and their current checks." />
      <AgentList />
    </div>
  );
};
