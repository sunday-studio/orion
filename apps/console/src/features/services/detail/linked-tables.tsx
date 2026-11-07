import { Link } from "react-router-dom";
import { Button, GridRow, GridTable, StatusBadge } from "@/design-system";
import { relativeAgo } from "@/features/incidents/incident-format";
import type { ServiceServiceLinkedApp, ServiceServiceLinkedMonitor } from "@/orion-sdk";
import { HealthDot } from "../components/service-health";
import { healthLabel, serviceHealth } from "../services.domain";

const MONITOR_COLUMNS = "18px minmax(0,1fr) 90px 110px";

export const MonitorsTable = ({
  monitors,
}: {
  monitors: readonly ServiceServiceLinkedMonitor[];
}) => (
  <GridTable className="overflow-hidden rounded-xl border border-hairline">
    <GridRow columns={MONITOR_COLUMNS} header>
      <span />
      <span>Name</span>
      <span>Type</span>
      <span>Status</span>
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
      </GridRow>
    ))}
  </GridTable>
);

const APP_COLUMNS = "minmax(0,1fr) 110px 130px";

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
      <span>Last log</span>
    </GridRow>
    {apps.map((app) => (
      <GridRow key={app.id} columns={APP_COLUMNS}>
        <Link to={`/settings/apps/${app.id}`} className="truncate font-medium hover:underline">
          {app.name ?? app.slug}
        </Link>
        <span className="text-faint">{app.environment ?? "-"}</span>
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
