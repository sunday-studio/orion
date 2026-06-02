import { PageHeader } from "@/components/shared/page-header";
import { IncidentList } from "@/features/incidents/components/incident-list";

export const IncidentsPage = () => {
  return (
    <div className="space-y-4">
      <PageHeader title="Incidents" description="Operational history and active issues." />
      <IncidentList />
    </div>
  );
};
