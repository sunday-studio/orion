import type { ApiIncidentResponse } from "@/orion-sdk";
import { cn } from "@/lib/utils";

export type IncidentSummaryStatus = "all" | "open" | "acknowledged" | "resolved" | "errors";

type IncidentSummaryProps = {
  totalCount: number;
  openCount: number;
  acknowledgedCount: number;
  resolvedCount: number;
  visibleIncidents: ApiIncidentResponse[];
  selectedStatus: IncidentSummaryStatus;
  onStatusChange: (status: IncidentSummaryStatus) => void;
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
    selectedTextClassName: string;
  }> = [
    {
      status: "all",
      label: "total",
      value: `${totalCount} ${label}`,
      selectedClassName: "bg-neutral-200 hover:bg-neutral-300",
      selectedTextClassName: "text-neutral-900",
    },
    {
      status: "open",
      label: "open",
      value: openCount,
      selectedClassName: "bg-rose-200",
      selectedTextClassName: "text-rose-900",
    },
    {
      status: "acknowledged",
      label: "acknowledged",
      value: acknowledgedCount,
      selectedClassName: "bg-amber-200",
      selectedTextClassName: "text-amber-900",
    },
    {
      status: "resolved",
      label: "resolved",
      value: resolvedCount,
      selectedClassName: "bg-blue-200",
      selectedTextClassName: "text-blue-900",
    },
    {
      status: "errors",
      label: "errors shown",
      value: visibleErrorCount,
      selectedClassName: "bg-red-200",
      selectedTextClassName: "text-red-900",
    },
  ];

  return (
    <div className="grid gap-px py-2 text-sm sm:grid-cols-5">
      {items.map((item) => {
        const isSelected = selectedStatus === item.status;

        return (
          <button
            key={item.status}
            type="button"
            className={cn(
              "flex h-26 flex-col justify-between bg-neutral-100 p-3 text-left transition-colors",
              isSelected ? item.selectedClassName : "hover:bg-neutral-100/90",
            )}
            onClick={() => onStatusChange(item.status)}
          >
            <span
              className={cn(
                "text-neutral-700 capitalize",
                isSelected && item.selectedTextClassName,
              )}
            >
              {item.label}
            </span>
            <span className={cn("font-medium text-lg", isSelected && item.selectedTextClassName)}>
              {item.value}
            </span>
          </button>
        );
      })}
    </div>
  );
};
