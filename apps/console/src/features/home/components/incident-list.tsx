import { IncidentSummary } from "./incident-summary";
import { Separator } from "@/components/ui/separator";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { Fragment } from "react/jsx-runtime";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";

export const IncidentList = () => {
  const incidentsResponse = useGetIncidents();
  const incidents = incidentsResponse.data?.incidents ?? [];
  const count = incidentsResponse.data?.count ?? incidents.length;

  if (incidentsResponse.isLoading) {
    return <div className="py-2">Loading incidents…</div>;
  }

  if (incidentsResponse.error) {
    return <div className="py-2">Unable to load incidents.</div>;
  }

  return (
    <div className="space-y-2">
      <IncidentSummary count={count} />
      <div>
        {incidents.length === 0 && <div className="py-2">No active incidents.</div>}
        {incidents.map((incident: ApiIncidentResponse, index: number) => (
          <Fragment key={incident.id ?? index}>
            <div className="flex items-center gap-3 py-2">
              <span className="font-medium">{incident.title ?? "Untitled incident"}</span>
              <span>{incident.agent_name ?? "Unknown server"}</span>
              <span>{incident.monitor_name ?? "Unknown monitor"}</span>
              <span>{incident.status ?? "unknown"}</span>
              <span>{incident.severity ?? "unknown"}</span>
              <span>opened {formatDate(incident.opened_at, DATE_TIME_FORMAT)}</span>
            </div>
            {index < incidents.length - 1 && <Separator />}
          </Fragment>
        ))}
      </div>
    </div>
  );
};
