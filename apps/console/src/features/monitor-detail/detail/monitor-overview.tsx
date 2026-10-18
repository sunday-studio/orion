import { StatBox } from "@/design-system";
import type { ApiMonitorDetailSummary } from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";
import { AvailabilityTable } from "./availability-table";
import { ResponseTimeChart } from "./response-time-chart";

const ago = (iso: string | undefined, now: number) => {
  if (!iso) return "-";
  const seconds = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
  if (seconds < 60) return `${seconds} seconds ago`;
  return `${formatSpan(new Date(iso).getTime(), now, false, true)} ago`;
};

export const MonitorOverview = ({
  monitorId,
  summary,
  now,
}: {
  monitorId: string;
  summary?: ApiMonitorDetailSummary;
  now: number;
}) => {
  const status = summary?.status;
  const since = summary?.current_state_since
    ? formatSpan(new Date(summary.current_state_since).getTime(), now, true)
    : undefined;
  const down = status === "down";
  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-3 gap-3">
        <StatBox
          label={
            down ? "Currently down for" : status === "up" ? "Currently up for" : "Current state"
          }
          value={
            since ? (
              <span className={down ? "text-[#be123c]" : undefined}>{since}</span>
            ) : (
              (summary?.status_text ?? "-")
            )
          }
        />
        <StatBox label="Last checked" value={ago(summary?.last_checked_at, now)} />
        <StatBox label="Incidents, 30 days" value={summary?.incidents_30d ?? "-"} />
      </div>
      <ResponseTimeChart monitorId={monitorId} />
      <AvailabilityTable monitorId={monitorId} />
    </div>
  );
};
