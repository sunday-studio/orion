import { IncidentSummary } from "./incident-summary";
import { Separator } from "@/components/ui/separator";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { Fragment } from "react/jsx-runtime";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useState } from "react";
import { ListPagination } from "@/components/list-pagination";

const INCIDENT_LIMIT = 20;

export const IncidentList = () => {
  const [offset, setOffset] = useState(0);
  const incidentsResponse = useGetIncidents({ limit: INCIDENT_LIMIT, offset });
  const incidents = incidentsResponse.data?.incidents ?? [];
  const count = incidentsResponse.data?.count ?? incidents.length;

  if (incidentsResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading incidents...</div>;
  }

  if (incidentsResponse.error) {
    return <div className="py-3 text-sm">Unable to load incidents.</div>;
  }

  return (
    <div className="space-y-2">
      <IncidentSummary count={count} />
      <div>
        {incidents.length === 0 && (
          <div className="py-3 text-sm text-neutral-600">No incidents recorded.</div>
        )}
        {incidents.map((incident: ApiIncidentResponse, index: number) => (
          <Fragment key={incident.id ?? index}>
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2 text-sm sm:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_minmax(0,1fr)_auto_auto]">
              <div className="min-w-0">
                <div className="truncate font-medium">{incident.title ?? "Untitled incident"}</div>
                <div className="truncate text-neutral-600">
                  {incident.latest_event ?? "No recent event"}
                </div>
              </div>
              <span className="hidden truncate sm:inline">
                {incident.agent_name ?? "Unknown server"}
              </span>
              <span className="hidden truncate sm:inline">
                {incident.monitor_name ?? "Unknown monitor"}
              </span>
              <span className="hidden sm:inline">{incident.severity ?? "unknown"}</span>
              <div className="text-right">
                <div>{incident.status ?? "unknown"}</div>
                <div className="text-neutral-600">
                  {formatDate(incident.opened_at, DATE_TIME_FORMAT)}
                </div>
              </div>
            </div>
            {index < incidents.length - 1 && <Separator />}
          </Fragment>
        ))}
      </div>
      <ListPagination
        count={count}
        limit={INCIDENT_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
