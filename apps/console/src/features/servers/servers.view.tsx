import { AgentList } from "@/features/servers/components/agent-list";

export const AgentsPage = () => {
  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-base font-medium">Agents</h1>
        <p className="text-sm text-neutral-600">Installed agents and their current checks.</p>
      </div>
      <AgentList />
    </div>
  );
};
