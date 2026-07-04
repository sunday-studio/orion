import { StatusBadge, toStatus } from "@/components/shared/status-badges";
import { Button } from "@/components/ui/button";
import {
  type ApiCoreMonitorConfigResponse,
  type ApiIncidentResponse,
  type ApiMonitorReportResponse,
  type ApiMonitorResponse,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { cn } from "@/utils/cn";
import { FileJson } from "lucide-react";
import { Link } from "react-router-dom";
import type { MonitorResultSummary } from "@/features/monitors/monitors.domain";
import {
  DetailGroup,
  DetailItem,
  bucketFillClassName,
  formatUptime,
  heartbeatPayloadContext,
  reportTimestamp,
} from "./monitor-detail-shared";

type UptimeBucket = {
  date?: string;
  total?: number;
  uptime_percent?: number;
};

type MonitorDetailOverviewProps = {
  activeIncidents: ApiIncidentResponse[];
  coreConfig?: ApiCoreMonitorConfigResponse;
  health?: string;
  highlightedIncident?: ApiIncidentResponse;
  incidentContext?: ApiIncidentResponse;
  incidentContextLabel: string;
  isCoreMonitor: boolean;
  latestHeartbeatFailure?: ApiMonitorReportResponse;
  latestHeartbeatReport?: ApiMonitorReportResponse;
  latestReport?: ApiMonitorReportResponse;
  latestSummary: MonitorResultSummary;
  monitor: ApiMonitorResponse;
  recentUptimeBuckets: UptimeBucket[];
  relatedIncidents: ApiIncidentResponse[];
  setSelectedReport: (report: ApiMonitorReportResponse) => void;
  uptimePercent?: number;
  uptimeBucketCount: number;
};

export const HighlightedIncidentBanner = ({ incident }: { incident?: ApiIncidentResponse }) => {
  if (!incident) return null;

  return (
    <section className="flex flex-wrap items-center justify-between gap-3 bg-rose-50 px-3 py-2.5 text-sm">
      <div>
        <div className="font-medium text-rose-900">
          Highlighted incident: {incident.title ?? incident.id}
        </div>
        <div className="text-neutral-600">
          {incident.latest_event ?? "No latest event recorded."}
        </div>
      </div>
      <Link
        className="px-2 py-1.5 font-medium text-rose-900 hover:bg-rose-200"
        to={`/incidents/${incident.id}`}
      >
        View incident
      </Link>
    </section>
  );
};

export const MonitorDetailOverview = ({
  activeIncidents,
  coreConfig,
  health,
  incidentContext,
  incidentContextLabel,
  isCoreMonitor,
  latestHeartbeatFailure,
  latestHeartbeatReport,
  latestReport,
  latestSummary,
  monitor,
  recentUptimeBuckets,
  relatedIncidents,
  setSelectedReport,
  uptimePercent,
  uptimeBucketCount,
}: MonitorDetailOverviewProps) => (
  <section className="space-y-4">
    <div className="grid gap-3 lg:grid-cols-4">
      <DetailGroup title="State" variant={health === "up" ? "emerald" : "indigo"}>
        <DetailItem label="health" value={<StatusBadge value={toStatus(health)} />} />
        <DetailItem label="type" value={latestSummary.kindLabel} />
        <DetailItem label="source" value={isCoreMonitor ? "Core" : "Server"} />
        <DetailItem
          label="owner"
          value={monitor.owner_name ?? monitor.agent_name ?? "Unknown owner"}
        />
        <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
      </DetailGroup>

      <DetailGroup title="Schedule">
        <DetailItem
          label="last checked"
          value={formatDate(reportTimestamp(latestReport), DATE_TIME_FORMAT)}
        />
        <DetailItem
          label="last success"
          value={formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
        />
        <DetailItem
          label="next run"
          value={formatDate(coreConfig?.next_run_at, DATE_TIME_FORMAT)}
        />
        <DetailItem
          label="interval"
          value={`${coreConfig?.interval_seconds ?? monitor.reporting_interval_seconds ?? 0}s`}
        />
        {!isCoreMonitor && monitor.agent_id && (
          <Link
            className="text-sm font-medium hover:text-neutral-600"
            to={`/servers/${monitor.agent_id}?tab=monitors`}
          >
            View server
          </Link>
        )}
      </DetailGroup>

      <DetailGroup
        title="Latest Result"
        variant={latestReport?.health === "down" ? "rose" : "amber"}
      >
        <DetailItem label="summary" value={latestSummary.headline} />
        {latestSummary.items.slice(0, 5).map((detail) => (
          <DetailItem key={detail.label} label={detail.label} value={detail.value} />
        ))}
        {latestReport && (
          <Button size="sm" variant="outline" onClick={() => setSelectedReport(latestReport)}>
            <FileJson />
            Inspect raw report
          </Button>
        )}
      </DetailGroup>

      <DetailGroup
        title="Incident Context"
        variant={activeIncidents.length > 0 ? "rose" : "neutral"}
      >
        <DetailItem label="active" value={activeIncidents.length} />
        <DetailItem label="related" value={relatedIncidents.length} />
        <DetailItem label="focus" value={incidentContextLabel} />
        {incidentContext && (
          <DetailItem
            label="latest event"
            value={incidentContext.latest_event ?? "No latest event recorded."}
          />
        )}
        {incidentContext && (
          <Link
            className="text-sm font-medium hover:text-neutral-600"
            to={`/incidents/${incidentContext.id}`}
          >
            View incident
          </Link>
        )}
      </DetailGroup>
    </div>

    <div className="space-y-1">
      <h2 className="text-sm font-medium">Latest Explanation</h2>
      <p className="max-w-3xl text-sm text-neutral-600">{latestSummary.explanation}</p>
    </div>

    {coreConfig?.kind === "heartbeat" && (
      <div className="grid gap-3 lg:grid-cols-2">
        <DetailGroup title="Latest Heartbeat">
          <DetailItem
            label="status"
            value={<StatusBadge value={toStatus(latestHeartbeatReport?.health)} />}
          />
          <DetailItem
            label="time"
            value={formatDate(reportTimestamp(latestHeartbeatReport), DATE_TIME_FORMAT)}
          />
          <DetailItem label="payload" value={heartbeatPayloadContext(latestHeartbeatReport)} />
        </DetailGroup>
        <DetailGroup title="Latest Heartbeat Failure">
          <DetailItem
            label="status"
            value={<StatusBadge value={toStatus(latestHeartbeatFailure?.health)} />}
          />
          <DetailItem
            label="time"
            value={formatDate(reportTimestamp(latestHeartbeatFailure), DATE_TIME_FORMAT)}
          />
          <DetailItem label="payload" value={heartbeatPayloadContext(latestHeartbeatFailure)} />
        </DetailGroup>
      </div>
    )}

    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <DetailItem label="90d uptime" value={formatUptime(uptimePercent)} />
        <DetailItem label="days sampled" value={uptimeBucketCount} />
      </div>
      {recentUptimeBuckets.length > 0 && (
        <div className="flex gap-0.5">
          {recentUptimeBuckets.map((bucket, index) => (
            <div
              key={`${bucket.date ?? index}-${bucket.uptime_percent ?? "na"}`}
              title={`${bucket.date}: ${formatUptime(bucket.uptime_percent)}`}
              className="flex h-7 w-2 items-end bg-neutral-100"
            >
              <div
                className={cn("mt-auto w-full", bucketFillClassName(bucket))}
                style={{ height: `${Math.max(4, bucket.uptime_percent ?? 0)}%` }}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  </section>
);
