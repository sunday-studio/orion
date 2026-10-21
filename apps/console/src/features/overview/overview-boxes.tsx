import { Link } from "react-router-dom";
import {
  Box,
  BoxHeader,
  Dot,
  GridFooter,
  GridRow,
  GridTable,
  StatBox,
  StatusBadge,
} from "@/design-system";
import type { ApiHealthIssue, ApiOverviewIncidentResponse } from "@/orion-sdk";
import { pluralize } from "@/utils/plural";
import { formatSpan, formatWhen } from "./overview-time";

const healthDot = (health?: string) =>
  health === "down" ? "#e5484d" : health === "degraded" ? "#f59e0b" : "#a1a1aa";

const incidentDot = (severity?: string) =>
  severity === "critical" || severity === "high"
    ? "#e5484d"
    : severity === "low"
      ? "#a1a1aa"
      : "#f59e0b";

export const AttentionBox = ({ issues, total }: { issues: ApiHealthIssue[]; total: number }) => {
  const columns = "12px minmax(0,1fr) 110px 80px 100px";
  const monitorCount = new Set(issues.map((issue) => issue.monitor_id)).size;
  return (
    <Box aria-label="Needs attention">
      <BoxHeader
        title="Needs attention"
        aside={`from health issues, ${pluralize(total || monitorCount, "monitor")}`}
      />
      <GridTable>
        {issues.slice(0, 3).map((issue, index) => (
          <GridRow columns={columns} key={issue.monitor_id ?? index}>
            <span>
              <Dot color={healthDot(issue.health)} />
            </span>
            <span>
              {issue.monitor_id ? (
                <Link className="font-medium hover:underline" to={`/monitors/${issue.monitor_id}`}>
                  {issue.monitor_name ?? issue.monitor_id}
                </Link>
              ) : (
                <b className="font-medium">{issue.monitor_name ?? "Monitor"}</b>
              )}
            </span>
            <span className="text-faint">{issue.node_name ?? "Core"}</span>
            <span className="font-mono text-xs text-faint">
              {issue.monitor_type ? issue.monitor_type.toUpperCase() : "Node"}
            </span>
            <span>
              <StatusBadge status={issue.health}>{issue.health}</StatusBadge>
            </span>
          </GridRow>
        ))}
        <GridFooter>
          <Link className="underline" to="/health-issues">
            View all health issues
          </Link>
        </GridFooter>
      </GridTable>
    </Box>
  );
};

export const OngoingBox = ({
  incidents,
  count,
  now,
  topId,
}: {
  incidents: ApiOverviewIncidentResponse[];
  count: number;
  now: number;
  /** The incident shown in "Needs you now" reads "Ongoing 30m" in red; the others show a plain span. */
  topId?: string;
}) => {
  const columns = "12px minmax(0,1fr) 110px 110px";
  return (
    <Box aria-label="Ongoing incidents">
      <BoxHeader title="Ongoing incidents" aside={count} />
      <GridTable>
        {incidents.map((incident, index) => {
          const openedMs = incident.opened_at ? new Date(incident.opened_at).getTime() : now;
          const isTop = incident.id === topId;
          return (
            <GridRow columns={columns} key={incident.id ?? index}>
              <span>
                <Dot color={incidentDot(incident.severity)} />
              </span>
              <span>
                <Link className="font-medium hover:underline" to={`/incidents/${incident.id}`}>
                  {incident.title ?? incident.id}
                </Link>
              </span>
              <span className="text-faint">{formatWhen(incident.opened_at, now)}</span>
              <span className={isTop ? "text-[#be123c]" : "text-faint"}>
                {isTop
                  ? `Ongoing ${formatSpan(openedMs, now, false, true)}`
                  : formatSpan(openedMs, now)}
              </span>
            </GridRow>
          );
        })}
      </GridTable>
    </Box>
  );
};

export const RecentIncidentsBox = ({
  incidents,
  now,
}: {
  incidents: ApiOverviewIncidentResponse[];
  now: number;
}) => (
  <Box aria-label="Recent incidents">
    <BoxHeader
      title="Recent incidents"
      aside={
        <Link className="font-normal text-quiet" to="/incidents">
          View all
        </Link>
      }
    />
    <div>
      {incidents.map((incident, index) => {
        const start = 0;
        const end = (incident.duration_seconds ?? 0) * 1000;
        return (
          <div
            key={incident.id ?? index}
            className="grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-2.5 border-b border-line-soft px-3.5 py-[9px] text-[12.5px] last:border-b-0"
          >
            <time className="truncate text-faint">{formatWhen(incident.opened_at, now)}</time>
            <Link className="truncate font-medium hover:underline" to={`/incidents/${incident.id}`}>
              {incident.title}
            </Link>
            <span className="text-quiet">{formatSpan(start, end)}</span>
          </div>
        );
      })}
    </div>
  </Box>
);

export const StatColumn = ({
  monitorsUp,
  monitorsTotal,
  serversUp,
  serversTotal,
  openIncidents,
}: {
  monitorsUp: number;
  monitorsTotal: number;
  serversUp: number;
  serversTotal: number;
  openIncidents: number;
}) => (
  <>
    <StatBox label="Monitors up" value={monitorsUp} of={monitorsTotal} />
    <StatBox label="Servers up" value={serversUp} of={serversTotal} />
    <StatBox label="Open incidents" value={openIncidents} />
  </>
);
