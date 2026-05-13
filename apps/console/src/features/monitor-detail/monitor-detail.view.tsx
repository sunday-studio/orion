import { ListPagination } from "@/components/list-pagination";
import { Separator } from "@/components/ui/separator";
import {
  type ApiMonitorReportResponse,
  useGetAgent,
  useGetIncidents,
  useGetMonitor,
  useGetMonitorHistory,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link, useParams } from "react-router-dom";
import { useState } from "react";

const HISTORY_LIMIT = 20;

type MonitorPayload = Record<string, unknown>;

const parsePayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const readString = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number") return String(value);
  }
  return "—";
};

const readNumber = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "number") return value;
    if (typeof value === "string" && value.trim() !== "" && !Number.isNaN(Number(value))) {
      return Number(value);
    }
  }
  return undefined;
};

const formatLatency = (payload: MonitorPayload) => {
  const latency = readNumber(payload, ["latency_ms", "response_time_ms", "duration_ms"]);
  return typeof latency === "number" ? `${latency} ms` : "—";
};

const payloadSummary = (report: ApiMonitorReportResponse) => {
  const payload = parsePayload(report.payload);
  return (
    readString(payload, [
      "failure_reason",
      "message",
      "error",
      "summary",
      "status",
      "status_code",
    ]) ?? "—"
  );
};

const DetailItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);

export const MonitorDetailPage = () => {
  const { monitorId = "" } = useParams();
  const [historyOffset, setHistoryOffset] = useState(0);
  const monitorResponse = useGetMonitor(monitorId);
  const historyResponse = useGetMonitorHistory(monitorId, {
    limit: HISTORY_LIMIT,
    offset: historyOffset,
  });
  const incidentsResponse = useGetIncidents({ limit: 100 });

  const monitor = monitorResponse.data?.monitor;
  const parentAgentResponse = useGetAgent(monitor?.agent_id ?? "");
  const reports = historyResponse.data?.reports ?? [];
  const reportCount = historyResponse.data?.count ?? reports.length;
  const latestReport = monitorResponse.data?.recent_reports?.[0] ?? reports[0];
  const latestPayload = parsePayload(latestReport?.payload);
  const health =
    monitorResponse.data?.computed_health ??
    monitor?.computed_health ??
    monitor?.health ??
    "unknown";
  const relatedIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.monitor_id === monitorId,
  );
  const rawPayload = latestReport?.payload ?? "No payload recorded.";

  if (monitorResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading monitor...</div>;
  }

  if (monitorResponse.error || !monitor) {
    return <div className="py-3 text-sm">Unable to load monitor.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <Link
          to={`/servers/${monitor.agent_id}`}
          className="text-sm text-neutral-600 hover:text-neutral-950"
        >
          {parentAgentResponse.data?.agent?.name ?? "Server"}
        </Link>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-base font-medium">
              {monitor.name ?? monitor.id ?? "Unknown monitor"}
            </h1>
            <p className="text-sm text-neutral-600">
              {monitor.type ?? "unknown"} · {health} · last checked{" "}
              {formatDate(latestReport?.created_at ?? latestReport?.collected_at, DATE_TIME_FORMAT)}
            </p>
          </div>
          <div className="text-sm font-medium">{relatedIncidents.length} active incidents</div>
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Current Result</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="health" value={health} />
          <DetailItem label="latency" value={formatLatency(latestPayload)} />
          <DetailItem
            label="status code"
            value={readString(latestPayload, ["status_code", "code"])}
          />
          <DetailItem
            label="resolved ip"
            value={readString(latestPayload, ["resolved_ip", "ip"])}
          />
          <DetailItem
            label="tls expiry"
            value={readString(latestPayload, [
              "tls_days_remaining",
              "tls_expiry",
              "certificate_expiry",
            ])}
          />
          <DetailItem
            label="last success"
            value={formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
          />
        </div>
        <div className="space-y-1">
          <div className="text-sm font-medium">Failure reason</div>
          <p className="text-sm text-neutral-600">
            {readString(latestPayload, ["failure_reason", "message", "error", "detail"])}
          </p>
        </div>
        <pre className="overflow-auto whitespace-pre-wrap py-2 text-sm text-neutral-700">
          {rawPayload}
        </pre>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Check History</h2>
        {historyResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading check history...</div>
        )}
        {historyResponse.error && <div className="text-sm">Unable to load check history.</div>}
        {!historyResponse.isLoading && !historyResponse.error && reports.length === 0 && (
          <div className="text-sm text-neutral-600">No check history recorded.</div>
        )}
        <div>
          {reports.map((report, index) => {
            const payload = parsePayload(report.payload);
            return (
              <div key={report.id ?? index}>
                <div className="grid grid-cols-[minmax(0,1fr)_6rem] items-center gap-3 py-2 text-sm sm:grid-cols-[minmax(0,1.2fr)_6rem_7rem_minmax(0,1fr)]">
                  <span className="truncate font-medium">
                    {formatDate(report.created_at ?? report.collected_at, DATE_TIME_FORMAT)}
                  </span>
                  <span>{report.health ?? "unknown"}</span>
                  <span className="hidden sm:inline">{formatLatency(payload)}</span>
                  <span className="hidden truncate text-neutral-600 sm:inline">
                    {payloadSummary(report)}
                  </span>
                </div>
                {index < reports.length - 1 && <Separator />}
              </div>
            );
          })}
        </div>
        <ListPagination
          count={reportCount}
          limit={HISTORY_LIMIT}
          offset={historyOffset}
          onOffsetChange={setHistoryOffset}
        />
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Configuration Snapshot</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="type" value={monitor.type ?? "unknown"} />
          <DetailItem label="interval" value={`${monitor.reporting_interval_seconds ?? 0}s`} />
          <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
          <DetailItem label="expected status/body/regex" value="config-owned" />
          <DetailItem label="thresholds" value="config-owned" />
          <DetailItem label="alerts" value="config-owned" />
        </div>
        <p className="text-sm text-neutral-600">
          Monitor configuration remains read-only in Console and is sourced from the Agent/Core
          configuration flow.
        </p>
      </section>
    </div>
  );
};
