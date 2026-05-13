import { AgentList } from "@/features/servers/components/agent-list";

export const ServersPage = () => {
  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-base font-medium">Servers</h1>
        <p className="text-sm text-neutral-600">Monitored servers and their current checks.</p>
      </div>
      <AgentList />
    </div>
  );
};
