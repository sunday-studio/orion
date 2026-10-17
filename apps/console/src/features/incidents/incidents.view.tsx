import { Link } from "react-router-dom";
import { PageHeader } from "@/design-system";
import { IncidentList } from "@/features/incidents/components/incident-list";

export const IncidentsPage = () => {
  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-4">
        <PageHeader title="Incidents" description="Operational history and active issues." />
        <Link className="text-[13px] text-brand hover:underline" to="/incidents/triage">
          Triage candidates
        </Link>
      </div>
      <IncidentList />
    </div>
  );
};
