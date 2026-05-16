import type { ApiAgentReportResponse, ApiAgentResponse } from "@/orion-sdk";
import { DetailItem } from "./detail-item";
import { formatBytes, formatDuration, formatPercent } from "./agent-detail-utils";

type AgentCpuTabProps = {
  agent: ApiAgentResponse;
  latestReport: ApiAgentReportResponse;
  configSummary: string;
};

export const AgentCpuTab = ({ agent, latestReport, configSummary }: AgentCpuTabProps) => {
  const location = latestReport.location ?? agent.location;

  return (
    <div className="space-y-6">
      <section className="space-y-3">
        <h2 className="text-sm font-medium">System Metrics</h2>
        <div className="grid gap-1 sm:grid-cols-3">
          <MetricCard label="cpu" value={formatPercent(latestReport.cpu?.usage_percent)} />
          <MetricCard label="memory" value={formatPercent(latestReport.memory?.used_percent)} />
          <MetricCard label="disk" value={formatPercent(latestReport.disk?.used_percent)} />
        </div>
        <div className="grid gap-3 pt-2 sm:grid-cols-3">
          <DetailItem label="system load" value={latestReport.cpu?.load_1?.toFixed(2) ?? "—"} />
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

const MetricCard = ({ label, value }: { label: string; value: string | number }) => (
  <div className="flex min-h-24 flex-col justify-between bg-neutral-100 px-3 py-2">
    <div className="text-neutral-600 text-sm">{label}</div>
    <div className="font-medium text-2xl text-neutral-950">{value}</div>
  </div>
);
