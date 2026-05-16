import { PageHeader } from "@/components/page-header";
import { MonitorList } from "@/features/monitors/components/monitor-list";

export const MonitorsPage = () => {
  return (
    <div className="space-y-4">
      <PageHeader title="Monitors" description="Registered checks across all agents." />
      <MonitorList />
    </div>
  );
};
