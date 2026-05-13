import { ListPagination } from "@/components/list-pagination";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { Separator } from "@/components/ui/separator";
import {
  type ApiAgentReportResponse,
  type ApiMonitorResponse,
  useGetAgent,
  useGetAgentHealth,
  useGetAgentMonitors,
  useGetAgentReports,
  useGetIncidents,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link, useParams } from "react-router-dom";
import { useState } from "react";

const REPORT_LIMIT = 10;

const asLatestReport = (value: unknown): ApiAgentReportResponse => {
  if (!value || typeof value !== "object") return {};
  return value as ApiAgentReportResponse;
};

const formatPercent = (value?: number) =>
  typeof value === "number" ? `${value.toFixed(1)}%` : "—";

const formatBytes = (value?: number) => {
  if (typeof value !== "number") return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size = size / 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
};

const formatDuration = (seconds?: number) => {
  if (typeof seconds !== "number") return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  return `${hours}h`;
};

const monitorHealth = (monitor: ApiMonitorResponse) =>
  monitor.health ?? monitor.computed_health ?? "unknown";

const monitorPriority = (monitor: ApiMonitorResponse) => {
  const health = monitorHealth(monitor);
  if (health === "down" || health === "degraded") return 0;
  if (health === "unknown" || health === "stale") return 1;
  return 2;
};

const DetailItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);

export const ServerDetailPage = () => {
  const { serverId = "" } = useParams();
  const [reportOffset, setReportOffset] = useState(0);
  const agentResponse = useGetAgent(serverId);
  const healthResponse = useGetAgentHealth(serverId);
  const monitorsResponse = useGetAgentMonitors(serverId, { limit: 100 });
  const reportsResponse = useGetAgentReports(serverId, {
    limit: REPORT_LIMIT,
    offset: reportOffset,
  });
  const incidentsResponse = useGetIncidents({ limit: 100 });

  const agent = agentResponse.data?.agent;
  const latestReport = asLatestReport(agentResponse.data?.latest_report);
  const reports = reportsResponse.data?.reports ?? [];
  const reportCount = reportsResponse.data?.count ?? reports.length;
  const monitors = [...(monitorsResponse.data?.monitors ?? [])].sort(
    (left, right) => monitorPriority(left) - monitorPriority(right),
  );
  const activeIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.agent_id === serverId,
  );
  const status =
    healthResponse.data?.overall_health ??
    agent?.status ??
    (agent?.maintenance_mode ? "maintenance" : "unknown");
  const location = latestReport.location ?? agent?.location;
  const configSummary =
    typeof latestReport.config_summary === "string"
      ? latestReport.config_summary
      : JSON.stringify(latestReport.config_summary ?? {}, null, 2);

  if (agentResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading server...</div>;
  }

  if (agentResponse.error || !agent) {
    return <div className="py-3 text-sm">Unable to load server.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <PageBreadcrumbs
          items={[{ label: "Servers", to: "/servers" }, { label: agent.name ?? "Server" }]}
        />
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-base font-medium">{agent.name ?? agent.id ?? "Unknown server"}</h1>
            <p className="text-sm text-neutral-600">
              {status} · last seen {formatDate(agent.last_seen, DATE_TIME_FORMAT)}
            </p>
          </div>
          {agent.maintenance_mode && <div className="text-sm font-medium">maintenance</div>}
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Health</h2>
        <div className="grid gap-3 sm:grid-cols-4">
          <DetailItem label="overall" value={status} />
          <DetailItem label="up" value={healthResponse.data?.up_count ?? 0} />
          <DetailItem label="down" value={healthResponse.data?.down_count ?? 0} />
          <DetailItem label="degraded" value={healthResponse.data?.degraded_count ?? 0} />
        </div>
        <p className="text-sm text-neutral-600">
          {activeIncidents.length} active incident{activeIncidents.length === 1 ? "" : "s"} on this
          server.
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">System Metrics</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="cpu" value={formatPercent(latestReport.cpu?.usage_percent)} />
          <DetailItem label="memory" value={formatPercent(latestReport.memory?.used_percent)} />
          <DetailItem label="disk" value={formatPercent(latestReport.disk?.used_percent)} />
          <DetailItem label="load" value={latestReport.cpu?.load_1?.toFixed(2) ?? "—"} />
          <DetailItem
            label="uptime"
            value={formatDuration(latestReport.uptime_seconds ?? agent.uptime_seconds)}
          />
          <DetailItem label="ip" value={location?.ip ?? agent.ip ?? "—"} />
          <DetailItem label="memory used" value={formatBytes(latestReport.memory?.used_bytes)} />
          <DetailItem label="disk used" value={formatBytes(latestReport.disk?.used_bytes)} />
        </div>
        <p className="text-sm text-neutral-600">
          {location?.hostname ?? location?.org ?? location?.city ?? "No location metadata"}
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Monitors</h2>
        {monitorsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading monitors...</div>
        )}
        {monitorsResponse.error && <div className="text-sm">Unable to load monitors.</div>}
        {!monitorsResponse.isLoading && !monitorsResponse.error && monitors.length === 0 && (
          <div className="text-sm text-neutral-600">No monitors registered.</div>
        )}
        <div>
          {monitors.map((monitor, index) => (
            <div key={monitor.id}>
              <div className="grid grid-cols-[minmax(0,1fr)_6rem] items-center gap-3 py-2 text-sm sm:grid-cols-[minmax(0,1.4fr)_7rem_7rem_minmax(0,1fr)]">
                <Link
                  to={`/monitors/${monitor.id}`}
                  className="truncate font-medium hover:text-neutral-600"
                >
                  {monitor.name ?? monitor.id}
                </Link>
                <span>{monitorHealth(monitor)}</span>
                <span className="hidden sm:inline">{monitor.type ?? "unknown"}</span>
                <span className="hidden truncate text-neutral-600 sm:inline">
                  last success {formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
                </span>
              </div>
              {index < monitors.length - 1 && <Separator />}
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Report Log</h2>
        {reportsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading reports...</div>
        )}
        {reportsResponse.error && <div className="text-sm">Unable to load reports.</div>}
        {!reportsResponse.isLoading && !reportsResponse.error && reports.length === 0 && (
          <div className="text-sm text-neutral-600">No reports recorded.</div>
        )}
        <div>
          {reports.map((report, index) => (
            <div key={report.id ?? index}>
              <div className="grid grid-cols-[minmax(0,1fr)_6rem] items-center gap-3 py-2 text-sm sm:grid-cols-[minmax(0,1.2fr)_7rem_7rem_7rem_minmax(0,1fr)]">
                <span className="truncate font-medium">
                  {formatDate(report.created_at ?? report.timestamp, DATE_TIME_FORMAT)}
                </span>
                <span>{formatPercent(report.cpu?.usage_percent)}</span>
                <span className="hidden sm:inline">
                  {formatPercent(report.memory?.used_percent)}
                </span>
                <span className="hidden sm:inline">{formatPercent(report.disk?.used_percent)}</span>
                <span className="hidden truncate text-neutral-600 sm:inline">
                  uptime {formatDuration(report.uptime_seconds)}
                </span>
              </div>
              {index < reports.length - 1 && <Separator />}
            </div>
          ))}
        </div>
        <ListPagination
          count={reportCount}
          limit={REPORT_LIMIT}
          offset={reportOffset}
          onOffsetChange={setReportOffset}
        />
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Configuration Snapshot</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="platform" value={agent.platform ?? agent.os ?? "unknown"} />
          <DetailItem label="arch" value={agent.arch ?? "unknown"} />
          <DetailItem label="agent version" value={latestReport.agent_version ?? "—"} />
        </div>
        <pre className="overflow-auto whitespace-pre-wrap py-2 text-sm text-neutral-700">
          {configSummary}
        </pre>
        <p className="text-sm text-neutral-600">
          `config.yaml` is user-facing configuration. Agent `state.db` stores runtime identity,
          tokens, maintenance state, and monitor id mappings.
        </p>
      </section>
    </div>
  );
};
