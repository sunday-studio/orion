import type { ApiIncidentResponse } from "@/orion-sdk";

type IncidentSummaryProps = {
  totalCount: number;
  openCount: number;
  acknowledgedCount: number;
  resolvedCount: number;
  visibleIncidents: ApiIncidentResponse[];
};

const isErrorIncident = (incident: ApiIncidentResponse) => {
  const notificationStatus = incident.notification_status?.toLowerCase();
  const severity = incident.severity?.toLowerCase();
  return notificationStatus === "failed" || severity === "error" || severity === "critical";
};

export const IncidentSummary = ({
  totalCount,
  openCount,
  acknowledgedCount,
  resolvedCount,
  visibleIncidents,
}: IncidentSummaryProps) => {
  const label = totalCount === 1 ? "incident" : "incidents";
  const visibleErrorCount = visibleIncidents.filter(isErrorIncident).length;

  return (
    <div className="grid gap-3 py-2 text-sm sm:grid-cols-5">
      <div>
        <div className="text-neutral-600">total</div>
        <div className="font-medium">
          {totalCount} {label}
        </div>
      </div>
      <div>
        <div className="text-neutral-600">open</div>
        <div className="font-medium">{openCount}</div>
      </div>
      <div>
        <div className="text-neutral-600">acknowledged</div>
        <div className="font-medium">{acknowledgedCount}</div>
      </div>
      <div>
        <div className="text-neutral-600">resolved</div>
        <div className="font-medium">{resolvedCount}</div>
      </div>
      <div>
        <div className="text-neutral-600">errors shown</div>
        <div className="font-medium">{visibleErrorCount}</div>
      </div>
    </div>
  );
};
