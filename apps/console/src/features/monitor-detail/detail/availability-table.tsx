import { GridRow, GridTable, Skeleton } from "@/design-system";
import { formatSpan } from "@/features/overview/overview-time";
import { useGetMonitorAvailability } from "@/orion-sdk";

const LABEL: Record<string, string> = {
  today: "Today",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  "365d": "Last 365 days",
};

const COLUMNS = "1.2fr 1fr 1fr 0.8fr 1fr";

const span = (seconds?: number) =>
  !seconds ? "-" : formatSpan(0, seconds * 1000, false, true).replace(/ min$/, "m");

export const AvailabilityTable = ({ monitorId }: { monitorId: string }) => {
  const query = useGetMonitorAvailability(monitorId);
  const periods = query.data?.periods ?? [];
  if (query.isPending) return <Skeleton className="h-40 w-full rounded-xl" />;
  if (periods.length === 0) return null;
  return (
    <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
      <GridTable>
        <GridRow columns={COLUMNS} header>
          <span>Period</span>
          <span>Availability</span>
          <span>Downtime</span>
          <span>Incidents</span>
          <span>Longest</span>
        </GridRow>
        {periods.map((period) => (
          <GridRow key={period.period} columns={COLUMNS}>
            <span>{LABEL[period.period ?? ""] ?? period.period}</span>
            <span>
              {typeof period.availability_percent === "number"
                ? `${period.availability_percent.toFixed(2)}%`
                : "-"}
            </span>
            <span>{span(period.downtime_seconds)}</span>
            <span>{period.incidents ?? 0}</span>
            <span>
              {span(period.longest_seconds)}
              {period.longest_ongoing ? ", ongoing" : ""}
            </span>
          </GridRow>
        ))}
      </GridTable>
    </div>
  );
};
