import { IncidentList } from "@/features/incidents/components/incident-list";

export const IncidentsPage = () => {
  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-medium">Incidents</h1>
        <p className="text-sm text-neutral-600">Operational history and active issues.</p>
      </div>
      <IncidentList />
    </div>
  );
};
