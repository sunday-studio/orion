import type { ApiIncidentInsightsResponse, ApiIncidentResponse } from "@/orion-sdk";
import { cn } from "@/lib/utils";

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
  insights?: ApiIncidentInsightsResponse;
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

const ditherBackground =
  "after:pointer-events-none after:absolute after:right-0 after:bottom-0 after:bg-[radial-gradient(currentColor_1px,transparent_1px)] after:bg-[size:6px_6px] after:opacity-50";

const secondsLabel = (seconds?: number) => {
  if (!seconds || seconds < 1) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${Math.max(1, minutes)}m`;
};

export const IncidentSummary = ({
  totalCount,
  openCount,
  acknowledgedCount,
  coveredCount,
  resolvedCount,
  visibleIncidents,
  insights,
  selectedStatus,
  onStatusChange,
}: IncidentSummaryProps) => {
  const label = totalCount === 1 ? "incident" : "incidents";
  const visibleErrorCount = visibleIncidents.filter(isErrorIncident).length;
  const lifecycle = insights?.lifecycle_timing;
  const reliability = insights?.notification_reliability;
  const recurring = insights?.recurring_failures?.[0];
  const successRate =
    reliability?.success_rate_percent === undefined
      ? "—"
      : `${Math.round(reliability.success_rate_percent)}%`;
  const items: Array<{
    status: IncidentSummaryStatus;
    label: string;
    value: string | number;
    selectedClassName: string;
    selectedTextClassName: string;
    ditherClassName: string;
  }> = [
    {
      status: "all",
      label: "total",
      value: `${totalCount} ${label}`,
      selectedClassName: "bg-neutral-200 hover:bg-neutral-300",
      selectedTextClassName: "text-neutral-900",
      ditherClassName: "after:h-18 after:w-28 after:[clip-path:ellipse(72%_42%_at_82%_88%)]",
    },
    {
      status: "open",
      label: "open",
      value: openCount,
      selectedClassName: "bg-rose-200",
      selectedTextClassName: "text-rose-900",
      ditherClassName:
        "after:h-24 after:w-20 after:[clip-path:polygon(54%_0,78%_24%,70%_42%,100%_64%,82%_100%,48%_92%,24%_100%,0_70%,18%_44%,16%_22%,38%_34%)]",
    },
    {
      status: "acknowledged",
      label: "acknowledged",
      value: acknowledgedCount,
      selectedClassName: "bg-amber-200",
      selectedTextClassName: "text-amber-900",
      ditherClassName:
        "after:h-20 after:w-28 after:[clip-path:polygon(24%_0,100%_0,100%_42%,38%_42%,38%_100%,0_100%,0_58%,62%_58%,62%_0)]",
    },
    {
      status: "covered",
      label: "covered",
      value: coveredCount,
      selectedClassName: "bg-cyan-200",
      selectedTextClassName: "text-cyan-900",
      ditherClassName:
        "after:h-20 after:w-24 after:[clip-path:polygon(50%_0,100%_22%,86%_82%,50%_100%,14%_82%,0_22%)]",
    },
    {
      status: "resolved",
      label: "resolved",
      value: resolvedCount,
      selectedClassName: "bg-blue-200",
      selectedTextClassName: "text-blue-900",
      ditherClassName:
        "after:h-20 after:w-28 after:[clip-path:polygon(76%_0,100%_18%,44%_100%,0_62%,20%_40%,42%_58%)]",
    },
    {
      status: "errors",
      label: "needs review",
      value: visibleErrorCount,
      selectedClassName: "bg-red-200",
      selectedTextClassName: "text-red-900",
      ditherClassName:
        "after:-right-6 after:-bottom-5 after:h-22 after:w-24 after:[clip-path:polygon(50%_0,62%_34%,100%_34%,70%_56%,82%_100%,50%_72%,18%_100%,30%_56%,0_34%,38%_34%)]",
    },
  ];

  return (
    <div className="space-y-3 py-2 text-sm">
      <div className="grid gap-1 sm:grid-cols-6">
        {items.map((item) => {
          const isSelected = selectedStatus === item.status;

          return (
            <button
              key={item.status}
              type="button"
              className={cn(
                "relative flex h-26 flex-col justify-between overflow-hidden p-3 text-left transition-colors",
                ditherBackground,
                item.ditherClassName,
                isSelected
                  ? cn(item.selectedClassName, item.selectedTextClassName)
                  : "bg-neutral-100 text-neutral-300 hover:bg-neutral-100/90",
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
              <span
                className={cn(
                  "font-medium text-lg text-neutral-600",
                  isSelected && item.selectedTextClassName,
                )}
              >
                {item.value}
              </span>
            </button>
          );
        })}
      </div>
      <div className="grid gap-1 sm:grid-cols-4">
        <InsightMetric
          label="MTTA"
          value={secondsLabel(lifecycle?.mean_time_to_acknowledge_seconds)}
        />
        <InsightMetric label="MTTR" value={secondsLabel(lifecycle?.mean_time_to_resolve_seconds)} />
        <InsightMetric label="delivery success" value={successRate} />
        <InsightMetric
          label="recurring monitor"
          value={recurring ? `${recurring.monitor_name} / ${recurring.incident_count}` : "—"}
        />
      </div>
    </div>
  );
};

const InsightMetric = ({ label, value }: { label: string; value: string | number }) => (
  <div className="bg-neutral-50 px-3 py-2">
    <div className="text-xs text-neutral-500">{label}</div>
    <div className="truncate font-medium text-neutral-800">{value}</div>
  </div>
);
