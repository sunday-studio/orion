import { AgentList } from "@/features/home/components/agent-list";
import { AttentionSummary } from "@/features/home/components/attention-summary";
import { HomeHeader } from "@/features/home/components/home-header";
import { IncidentList } from "@/features/home/components/incident-list";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function HomePage() {
  return (
    <div className="space-y-5">
      <HomeHeader />
      <AttentionSummary />
      <Tabs defaultValue="incidents" className="space-y-4">
        <TabsList variant="line">
          <TabsTrigger value="incidents">Incidents</TabsTrigger>
          <TabsTrigger value="agents">Servers</TabsTrigger>
        </TabsList>
        <TabsContent value="incidents">
          <IncidentList />
        </TabsContent>
        <TabsContent value="agents">
          <AgentList />
        </TabsContent>
      </Tabs>
    </div>
  );
}
