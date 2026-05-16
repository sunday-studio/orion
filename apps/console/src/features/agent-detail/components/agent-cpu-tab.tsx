import type { ApiAgentReportResponse, ApiAgentResponse } from "@/orion-sdk";
import { DetailItem } from "./detail-item";
import { formatBytes, formatDuration, formatPercent } from "./agent-detail-utils";

type AgentCpuTabProps = {
  agent: ApiAgentResponse;
  latestReport: ApiAgentReportResponse;
};

export const AgentCpuTab = ({ agent, latestReport }: AgentCpuTabProps) => {
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
          <DetailItem label="IP address" value={location?.ip ?? agent.ip ?? "—"} />
          <DetailItem label="memory used" value={formatBytes(latestReport.memory?.used_bytes)} />
          <DetailItem label="disk used" value={formatBytes(latestReport.disk?.used_bytes)} />
          <DetailItem label="agent version" value={latestReport.agent_version ?? "—"} />
          <DetailItem label="hostname" value={location?.hostname ?? "—"} />
          <DetailItem label="platform" value={agent.platform ?? agent.os ?? "unknown"} />
          <DetailItem label="arch" value={agent.arch ?? "unknown"} />
        </div>
      </section>
    </div>
  );
};

const MetricCard = ({ label, value }: { label: string; value: string | number }) => (
  <div className="flex min-h-24 flex-col justify-between bg-neutral-100 px-3 py-2">
    <div className="text-neutral-600 text-sm capitalize">{label}</div>
    <div className="font-medium text-2xl text-neutral-950">{value}</div>
  </div>
);
