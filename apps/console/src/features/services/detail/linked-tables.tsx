import { useQueries } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Button, GridRow, GridTable, StatusBadge } from "@/design-system";
import { relativeAgo } from "@/features/incidents/incident-format";
import {
  type ServiceServiceLinkedApp,
  type ServiceServiceLinkedMonitor,
  getGetMonitorAvailabilityQueryOptions,
  useGetLogHistogram,
} from "@/orion-sdk";
import { HealthDot } from "../components/service-health";
import { healthLabel, serviceHealth } from "../services.domain";
import { formatAvailability, histogramRates } from "../timeline-model";

/** 30-day availability for each monitor id, from the same cache the monitor page uses. */
export const useAvailability30d = (monitorIds: readonly string[]) => {
  const results = useQueries({
    queries: monitorIds.map((id) => getGetMonitorAvailabilityQueryOptions(id)),
  });
  return new Map(
    monitorIds.map((id, index) => [
      id,
      results[index]?.data?.periods?.find((period) => period.period === "30d")
        ?.availability_percent,
    ]),
  );
};

const MONITOR_COLUMNS = "18px minmax(0,1fr) 90px 110px 90px";

export const MonitorsTable = ({
  monitors,
}: {
  monitors: readonly ServiceServiceLinkedMonitor[];
}) => {
  const availability = useAvailability30d(monitors.map((monitor) => monitor.id ?? ""));
  return (
    <GridTable className="overflow-hidden rounded-xl border border-hairline">
      <GridRow columns={MONITOR_COLUMNS} header>
        <span />
        <span>Name</span>
        <span>Type</span>
        <span>Status</span>
        <span>30 days</span>
      </GridRow>
      {monitors.map((monitor) => (
        <GridRow key={monitor.id} columns={MONITOR_COLUMNS}>
          <HealthDot health={monitor.health} />
          <Link to={`/monitors/${monitor.id}`} className="truncate font-medium hover:underline">
            {monitor.name}
          </Link>
          <span className="text-faint">{monitor.type}</span>
          <span className="flex items-center gap-2">
            <StatusBadge status={serviceHealth(monitor.health)}>
              {healthLabel(serviceHealth(monitor.health))}
            </StatusBadge>
            {monitor.active_incident_id && (
              <Link to={`/incidents/${monitor.active_incident_id}`} className="text-xs underline">
                Incident
              </Link>
            )}
          </span>
          <span className="tabular-nums text-quiet">
            {typeof availability.get(monitor.id ?? "") === "number"
              ? formatAvailability(availability.get(monitor.id ?? "") ?? 0)
              : "-"}
          </span>
        </GridRow>
      ))}
    </GridTable>
  );
};

const APP_COLUMNS = "minmax(0,1fr) 100px 90px 90px 120px";

/** Lines per second and errors per minute over the last five minutes. */
const AppRates = ({ appId }: { appId: string }) => {
  const histogram = useGetLogHistogram(
    { app_id: appId, from: "-5m", buckets: 5 },
    { query: { staleTime: 30_000 } },
  );
  if (!histogram.data)
    return (
      <>
        <span className="text-faint">-</span>
        <span className="text-faint">-</span>
      </>
    );
  const rates = histogramRates(histogram.data.buckets ?? [], 300);
  return (
    <>
      <span className="tabular-nums text-quiet">{rates.linesPerSecond}</span>
      <span
        className={
          rates.errorsPerMinute > 0 ? "tabular-nums text-[#be123c]" : "tabular-nums text-quiet"
        }
      >
        {rates.errorsPerMinute}
      </span>
    </>
  );
};

export const AppsTable = ({
  apps,
  now,
}: {
  apps: readonly ServiceServiceLinkedApp[];
  now: number;
}) => (
  <GridTable className="overflow-hidden rounded-xl border border-hairline">
    <GridRow columns={APP_COLUMNS} header>
      <span>App</span>
      <span>Environment</span>
      <span>Lines/s</span>
      <span>Errors/min</span>
      <span>Last log</span>
    </GridRow>
    {apps.map((app) => (
      <GridRow key={app.id} columns={APP_COLUMNS}>
        <Link to={`/settings/apps/${app.id}`} className="truncate font-medium hover:underline">
          {app.name ?? app.slug}
        </Link>
        <span className="text-faint">{app.environment ?? "-"}</span>
        <AppRates appId={app.id ?? ""} />
        <span className="text-faint">
          {app.last_ingest_at ? relativeAgo(app.last_ingest_at, now) : "Never"}
        </span>
      </GridRow>
    ))}
  </GridTable>
);

export const SectionHead = ({
  title,
  action,
  onAction,
}: {
  title: string;
  action: string;
  onAction: () => void;
}) => (
  <div className="mb-2 flex items-center justify-between">
    <h2 className="text-sm font-medium">{title}</h2>
    <Button size="sm" variant="secondary" onClick={onAction}>
      {action}
    </Button>
  </div>
);
