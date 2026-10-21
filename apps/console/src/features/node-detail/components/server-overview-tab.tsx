import { Link } from "react-router-dom";
import {
  Box,
  BoxHeader,
  Button,
  Callout,
  Dot,
  FieldPanel,
  GridFooter,
  GridRow,
  GridTable,
  RaisedCard,
  StatBox,
} from "@/design-system";
import { type ServerRow, serverState } from "@/features/nodes/server-format";
import {
  type ApiMonitorResponse,
  type ApiNodeReportResponse,
  useGetNodeMonitors,
  useGetNodeReports,
} from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { monitorPriority } from "../node-detail.domain";
import { CpuChart } from "./cpu-chart";
import { MONITOR_COLUMNS, MonitorCells } from "./monitor-rows";
import { useServerMaintenance } from "./server-header";

const STATS = [
  ["CPU", (r?: ApiNodeReportResponse) => r?.cpu?.usage_percent],
  ["Memory", (r?: ApiNodeReportResponse) => r?.memory?.used_percent],
  ["Disk", (r?: ApiNodeReportResponse) => r?.disk?.used_percent],
] as const;

const whole = (value?: number) => (typeof value === "number" ? `${Math.round(value)}%` : "-");

const CHECKS = [
  ["Is the server running?", "Power, network, provider status"],
  ["Can it reach Core?", `curl -I ${window.location.origin}/v1/health`],
  ["Is the node running?", "systemctl status orion-node"],
  ["Is the token still valid?", "Revoked tokens stop reporting"],
] as const;

type Props = {
  node: ServerRow;
  latest?: ApiNodeReportResponse;
  onOpenMonitors: () => void;
};

export const ServerOverviewTab = ({ node, latest, onOpenMonitors }: Props) => {
  const nodeId = node.id ?? "";
  const state = serverState(node);
  const monitors = useGetNodeMonitors(nodeId, { limit: 100 });
  const reports = useGetNodeReports(nodeId, { limit: 120 });
  const maintenance = useServerMaintenance(node);
  const list: ApiMonitorResponse[] = monitors.data?.monitors ?? [];
  const total = monitors.data?.count ?? list.length;
  const attention = list
    .filter((monitor) => monitorPriority(monitor) < 2)
    .sort((a, b) => monitorPriority(a) - monitorPriority(b));
  const reportList = [...(reports.data?.reports ?? [])].sort(
    (a, b) =>
      new Date(b.timestamp ?? b.created_at ?? 0).getTime() -
      new Date(a.timestamp ?? a.created_at ?? 0).getTime(),
  );
  const lastSeen = node.last_seen ? formatDate(node.last_seen, TIME_FORMAT) : undefined;

  if (state === "stale") {
    return (
      <div className="flex flex-col gap-3">
        <Callout
          tone="down"
          className="flex items-center justify-between gap-3 rounded-[10px] px-4 py-3 text-sm font-medium"
        >
          <span className="flex items-center gap-2.5">
            <Dot color="#a1a1aa" />
            {node.name} has not reported since {lastSeen ?? "an unknown time"}. Its {total}{" "}
            monitors show Unknown.
          </span>
          <Button
            size="sm"
            variant="secondary"
            disabled={maintenance.pending || maintenance.inMaintenance}
            onClick={() => maintenance.set(true)}
          >
            Put in maintenance
          </Button>
        </Callout>
        <FieldPanel title="What to check" aside="Most likely first">
          <RaisedCard>
            {CHECKS.map(([title, hint]) => (
              <div
                key={title}
                className="grid grid-cols-[12px_minmax(0,0.9fr)_minmax(0,1.3fr)] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
              >
                <Dot color="#d4d4d8" />
                <b className="font-medium">{title}</b>
                <span className="truncate font-mono text-xs text-faint">{hint}</span>
              </div>
            ))}
          </RaisedCard>
        </FieldPanel>
        <FieldPanel
          title="Last reports"
          aside={
            node.reporting_interval_seconds
              ? `Reports arrive every ${node.reporting_interval_seconds} s`
              : undefined
          }
        >
          <RaisedCard>
            <GridTable>
              <GridRow columns="120px 1fr 1fr 1fr" header>
                <span>Time</span>
                <span>CPU</span>
                <span>Memory</span>
                <span>Disk</span>
              </GridRow>
              {reportList.slice(0, 3).map((report, index) => (
                <GridRow key={report.id ?? index} columns="120px 1fr 1fr 1fr">
                  <span className="font-mono text-faint">
                    {formatDate(report.timestamp ?? report.created_at, "HH:mm:ss")}
                  </span>
                  <span>{whole(report.cpu?.usage_percent)}</span>
                  <span>{whole(report.memory?.used_percent)}</span>
                  <span>{whole(report.disk?.used_percent)}</span>
                </GridRow>
              ))}
              <GridFooter className="text-left font-mono text-[11px] uppercase tracking-wide">
                No reports after {lastSeen ?? "-"}
              </GridFooter>
            </GridTable>
          </RaisedCard>
        </FieldPanel>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-3 gap-3">
        {STATS.map(([label, pick]) => (
          <StatBox key={label} label={label} value={whole(pick(latest))} />
        ))}
      </div>
      {attention.length > 0 && (
        <FieldPanel
          title="Needs attention"
          aside={`${attention.length} of ${total} monitors on this server`}
        >
          <RaisedCard>
            <GridTable>
              {attention.slice(0, 4).map((monitor) => (
                <GridRow key={monitor.id} columns={MONITOR_COLUMNS}>
                  <MonitorCells monitor={monitor} />
                </GridRow>
              ))}
              <GridFooter>
                <button type="button" className="underline" onClick={onOpenMonitors}>
                  View all {total} monitors on {node.name}
                </button>
              </GridFooter>
            </GridTable>
          </RaisedCard>
        </FieldPanel>
      )}
      <Box>
        <BoxHeader title="Recent CPU" aside={lastSeen ? `as of ${lastSeen}` : undefined} />
        <CpuChart reports={reportList} />
      </Box>
      {total === 0 && !monitors.isLoading && (
        <p className="text-[13px] text-quiet">
          No monitors on this server yet.{" "}
          <Link to="/monitors" className="underline">
            Open Monitors
          </Link>
        </p>
      )}
    </div>
  );
};
