import { AgentList } from "@/features/home/components/agent-list";
import { IncidentList } from "@/features/home/components/incident-list";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function HomePage() {
  return (
    <Tabs defaultValue="incidents" className="space-y-4">
      <TabsList variant="line">
        <TabsTrigger value="incidents">Incidents</TabsTrigger>
        <TabsTrigger value="agents">Agents</TabsTrigger>
      </TabsList>
      <TabsContent value="incidents">
        <IncidentList />
      </TabsContent>
      <TabsContent value="agents">
        <AgentList />
      </TabsContent>
    </Tabs>
  );
}
