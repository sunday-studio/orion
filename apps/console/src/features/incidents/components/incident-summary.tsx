import type { ApiIncidentResponse } from "@/orion-sdk";
import { cn } from "@/utils/cn";

export type IncidentSummaryStatus =
  | "all"
  | "open"
  | "acknowledged"
  | "covered"
  | "resolved"
  | "errors";

type IncidentSummaryProps = {
  totalCount: number;
  openCount: number;
  acknowledgedCount: number;
  coveredCount: number;
  resolvedCount: number;
  visibleIncidents: ApiIncidentResponse[];
  selectedStatus: IncidentSummaryStatus;
  onStatusChange: (status: IncidentSummaryStatus) => void;
};

const isErrorIncident = (incident: ApiIncidentResponse) => {
  const notificationStatus = incident.notification_status?.toLowerCase();
  const severity = incident.severity?.toLowerCase();
  return (
    notificationStatus === "failed" ||
    severity === "high" ||
    severity === "error" ||
    severity === "critical"
  );
};

export const IncidentSummary = ({
  totalCount,
  openCount,
  acknowledgedCount,
  coveredCount,
  resolvedCount,
  visibleIncidents,
  selectedStatus,
  onStatusChange,
}: IncidentSummaryProps) => {
  const label = totalCount === 1 ? "incident" : "incidents";
  const visibleErrorCount = visibleIncidents.filter(isErrorIncident).length;
  const items: Array<{
    status: IncidentSummaryStatus;
    label: string;
    value: string | number;
    selectedClassName: string;
  }> = [
    {
      status: "all",
      label: "total",
      value: `${totalCount} ${label}`,
      selectedClassName: "border-neutral-900 bg-neutral-100",
    },
    {
      status: "open",
      label: "open",
      value: openCount,
      selectedClassName: "border-rose-500 bg-rose-50",
    },
    {
      status: "acknowledged",
      label: "acknowledged",
      value: acknowledgedCount,
      selectedClassName: "border-amber-500 bg-amber-50",
    },
    {
      status: "covered",
      label: "covered",
      value: coveredCount,
      selectedClassName: "border-cyan-500 bg-cyan-50",
    },
    {
      status: "resolved",
      label: "resolved",
      value: resolvedCount,
      selectedClassName: "border-blue-500 bg-blue-50",
    },
    {
      status: "errors",
      label: "needs review",
      value: visibleErrorCount,
      selectedClassName: "border-red-500 bg-red-50",
    },
  ];

  return (
    <div className="space-y-3 py-2 text-sm">
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-6">
        {items.map((item) => {
          const isSelected = selectedStatus === item.status;

          return (
            <button
              key={item.status}
              type="button"
              className={cn(
                "flex h-22 flex-col justify-between border bg-white p-3 text-left transition-colors hover:bg-neutral-50",
                isSelected ? item.selectedClassName : "border-neutral-200 text-neutral-500",
              )}
              onClick={() => onStatusChange(item.status)}
            >
              <span className="text-neutral-600 capitalize">{item.label}</span>
              <span className="font-medium text-lg text-neutral-900">{item.value}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
};
