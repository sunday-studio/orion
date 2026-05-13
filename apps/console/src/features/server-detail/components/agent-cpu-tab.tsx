import type {
  ApiAgentReportResponse,
  ApiAgentResponse,
  ApiUptimeDayBucketResponse,
} from "@/orion-sdk";
import { DetailItem } from "./detail-item";
import { formatBytes, formatDuration, formatPercent } from "./agent-detail-utils";

type AgentCpuTabProps = {
  agent: ApiAgentResponse;
  latestReport: ApiAgentReportResponse;
  status: string;
  upCount: number;
  downCount: number;
  degradedCount: number;
  activeIncidentCount: number;
  configSummary: string;
  uptimePercent?: number;
  uptimeBuckets: ApiUptimeDayBucketResponse[];
};

export const AgentCpuTab = ({
  agent,
  latestReport,
  status,
  upCount,
  downCount,
  degradedCount,
  activeIncidentCount,
  configSummary,
  uptimePercent,
  uptimeBuckets,
}: AgentCpuTabProps) => {
  const location = latestReport.location ?? agent.location;
  const recentBuckets = uptimeBuckets.slice(-7);

  return (
    <div className="space-y-6">
      <section className="space-y-3">
        <h2 className="text-sm font-medium">Health</h2>
        <div className="grid gap-3 sm:grid-cols-4">
          <DetailItem label="overall" value={status} />
          <DetailItem label="up" value={upCount} />
          <DetailItem label="down" value={downCount} />
          <DetailItem label="degraded" value={degradedCount} />
        </div>
        <div className="grid gap-3 sm:grid-cols-4">
          <DetailItem label="90d uptime" value={formatPercent(uptimePercent)} />
        </div>
        <p className="text-sm text-neutral-600">
          {activeIncidentCount} active incident{activeIncidentCount === 1 ? "" : "s"} on this agent.
        </p>
        {recentBuckets.length > 0 && (
          <div className="grid gap-2 sm:grid-cols-7">
            {recentBuckets.map((bucket) => (
              <div key={bucket.date} className="text-sm">
                <div className="text-neutral-600">{bucket.date ?? "—"}</div>
                <div className="font-medium">{formatPercent(bucket.uptime_percent)}</div>
              </div>
            ))}
          </div>
        )}
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
          <DetailItem label="agent version" value={latestReport.agent_version ?? "—"} />
        </div>
        <p className="text-sm text-neutral-600">
          {location?.hostname ?? location?.org ?? location?.city ?? "No location metadata"}
        </p>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Configuration Snapshot</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <DetailItem label="platform" value={agent.platform ?? agent.os ?? "unknown"} />
          <DetailItem label="arch" value={agent.arch ?? "unknown"} />
        </div>
        <pre className="overflow-auto whitespace-pre-wrap py-2 text-sm text-neutral-700">
          {configSummary}
        </pre>
      </section>
    </div>
  );
};
