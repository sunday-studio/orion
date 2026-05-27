import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiAgentReportResponse, ApiMonitorReportResponse } from "@/orion-sdk";
import type { ReactNode } from "react";

type ReportInspectionDrawerProps =
  | {
      kind: "agent";
      report?: ApiAgentReportResponse;
      onOpenChange: (open: boolean) => void;
    }
  | {
      kind: "monitor";
      report?: ApiMonitorReportResponse;
      onOpenChange: (open: boolean) => void;
    };

type DetailItem = {
  label: string;
  value?: ReactNode;
};

type Payload = Record<string, unknown>;

const EMPTY_VALUE = "-";

const isRecord = (value: unknown): value is Payload =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const parsePayload = (payload?: string): Payload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return isRecord(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const formatValue = (value: unknown): string => {
  if (value === null || value === undefined || value === "") return EMPTY_VALUE;
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "number") return Number.isFinite(value) ? String(value) : EMPTY_VALUE;
  if (typeof value === "string") return value;
  return JSON.stringify(value);
};

const readPayloadValue = (payload: Payload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (value !== null && value !== undefined && value !== "") return formatValue(value);
  }
  return EMPTY_VALUE;
};

const readPayloadNumber = (payload: Payload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "number") return value;
    if (typeof value === "string" && value.trim() !== "" && !Number.isNaN(Number(value))) {
      return Number(value);
    }
  }
  return undefined;
};

const formatPercent = (value?: number) =>
  typeof value === "number" ? `${value.toFixed(1)}%` : EMPTY_VALUE;

const formatBytes = (value?: number) => {
  if (typeof value !== "number") return EMPTY_VALUE;
  const units = ["B", "KB", "MB", "GB", "TB"];
  let nextValue = value;
  let unitIndex = 0;
  while (nextValue >= 1024 && unitIndex < units.length - 1) {
    nextValue /= 1024;
    unitIndex += 1;
  }
  return `${nextValue.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
};

const formatLatency = (payload: Payload) => {
  const latency = readPayloadNumber(payload, ["latency_ms", "response_time_ms", "duration_ms"]);
  return typeof latency === "number" ? `${latency} ms` : EMPTY_VALUE;
};

const prettyJson = (value: unknown) => JSON.stringify(value, null, 2);

const metadataEntries = (payload: Payload) =>
  Object.entries(payload).filter(
    ([key]) =>
      ![
        "failure_reason",
        "message",
        "error",
        "summary",
        "status",
        "status_code",
        "latency_ms",
        "response_time_ms",
        "duration_ms",
      ].includes(key),
  );

const isHeartbeatPayload = (payload: Payload) =>
  payload.type === "heartbeat" || payload.runner === "heartbeat";

const DetailGrid = ({ items }: { items: DetailItem[] }) => (
  <div className="grid gap-3 sm:grid-cols-2">
    {items.map((item) => (
      <div key={item.label}>
        <div className="text-xs text-neutral-500">{item.label}</div>
        <div className="break-words text-sm font-medium">{item.value ?? EMPTY_VALUE}</div>
      </div>
    ))}
  </div>
);

const Section = ({ title, children }: { title: string; children: ReactNode }) => (
  <section className="space-y-3 border-t border-neutral-200 pt-4 first:border-t-0 first:pt-0">
    <h3 className="text-sm font-medium">{title}</h3>
    {children}
  </section>
);

const RawJson = ({ value }: { value: unknown }) => (
  <pre className="max-h-80 overflow-auto bg-neutral-950 p-3 text-xs leading-5 whitespace-pre-wrap text-neutral-50">
    {prettyJson(value)}
  </pre>
);

const AgentReportInspection = ({ report }: { report: ApiAgentReportResponse }) => (
  <>
    <Section title="Summary">
      <DetailGrid
        items={[
          { label: "report", value: report.id },
          { label: "agent", value: report.agent_id },
          { label: "reported", value: formatDate(report.timestamp, DATE_TIME_FORMAT) },
          { label: "created", value: formatDate(report.created_at, DATE_TIME_FORMAT) },
          { label: "version", value: report.agent_version },
          { label: "uptime", value: formatValue(report.uptime_seconds) },
        ]}
      />
    </Section>

    <Section title="Metrics">
      <DetailGrid
        items={[
          { label: "cpu usage", value: formatPercent(report.cpu?.usage_percent) },
          { label: "cpu cores", value: formatValue(report.cpu?.cores) },
          { label: "load 1m", value: formatValue(report.cpu?.load_1) },
          { label: "load 5m", value: formatValue(report.cpu?.load_5) },
          { label: "memory used", value: formatBytes(report.memory?.used_bytes) },
          { label: "memory usage", value: formatPercent(report.memory?.used_percent) },
          { label: "disk used", value: formatBytes(report.disk?.used_bytes) },
          { label: "disk usage", value: formatPercent(report.disk?.used_percent) },
        ]}
      />
    </Section>

    <Section title="Metadata">
      <DetailGrid
        items={[
          { label: "report interval", value: report.config_summary?.reporting_interval },
          {
            label: "configured monitors",
            value: formatValue(report.config_summary?.monitor_count),
          },
          { label: "monitor types", value: formatValue(report.config_summary?.monitor_types) },
          { label: "ip", value: report.location?.ip },
          { label: "city", value: report.location?.city },
          { label: "region", value: report.location?.region },
          { label: "country", value: report.location?.country },
          { label: "org", value: report.location?.org },
        ]}
      />
    </Section>

    <Section title="Raw JSON">
      <RawJson value={report} />
    </Section>
  </>
);

const MonitorReportInspection = ({ report }: { report: ApiMonitorReportResponse }) => {
  const payload = parsePayload(report.payload);
  const metadata = metadataEntries(payload);
  const isHeartbeat = isHeartbeatPayload(payload);

  return (
    <>
      <Section title="Summary">
        <DetailGrid
          items={[
            { label: "report", value: report.id },
            { label: "monitor", value: report.monitor_id },
            { label: "health", value: <StatusBadge value={toStatus(report.health)} /> },
            { label: "collected", value: formatDate(report.collected_at, DATE_TIME_FORMAT) },
            { label: "created", value: formatDate(report.created_at, DATE_TIME_FORMAT) },
            { label: "latency", value: formatLatency(payload) },
          ]}
        />
      </Section>

      <Section title="Result">
        <DetailGrid
          items={[
            {
              label: "reason",
              value: readPayloadValue(payload, [
                "failure_reason",
                "message",
                "error",
                "summary",
                "status",
              ]),
            },
            { label: "status code", value: readPayloadValue(payload, ["status_code", "code"]) },
            { label: "resolved ip", value: readPayloadValue(payload, ["resolved_ip", "ip"]) },
            {
              label: "tls expiry",
              value: readPayloadValue(payload, [
                "tls_days_remaining",
                "tls_expiry",
                "certificate_expiry",
              ]),
            },
          ]}
        />
      </Section>

      {isHeartbeat && (
        <Section title="Heartbeat">
          <DetailGrid
            items={[
              { label: "payload", value: readPayloadValue(payload, ["payload"]) },
              { label: "failure stage", value: readPayloadValue(payload, ["failure_stage"]) },
              { label: "truncated", value: readPayloadValue(payload, ["payload_truncated"]) },
              { label: "last signal", value: readPayloadValue(payload, ["last_signal_at"]) },
              { label: "missed after", value: readPayloadValue(payload, ["missed_after"]) },
            ]}
          />
        </Section>
      )}

      <Section title="Metadata">
        {metadata.length > 0 ? (
          <DetailGrid
            items={metadata.map(([label, value]) => ({ label, value: formatValue(value) }))}
          />
        ) : (
          <div className="text-sm text-neutral-600">No additional metadata recorded.</div>
        )}
      </Section>

      <Section title="Raw JSON">
        <RawJson value={Object.keys(payload).length > 0 ? payload : (report.payload ?? {})} />
      </Section>
    </>
  );
};

export const ReportInspectionDrawer = (props: ReportInspectionDrawerProps) => {
  const { report, onOpenChange } = props;
  const open = Boolean(report);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="top-0 left-auto right-0 h-dvh max-h-dvh max-w-full translate-x-0 translate-y-0 overflow-y-auto p-5 sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{props.kind === "agent" ? "Agent Report" : "Monitor Report"}</DialogTitle>
        </DialogHeader>
        {report && (
          <div className="space-y-5">
            {props.kind === "agent" ? (
              <AgentReportInspection report={report} />
            ) : (
              <MonitorReportInspection report={report} />
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};
