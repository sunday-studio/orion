import { InfiniteScrollSentinel } from "@/components/infinite-scroll-sentinel";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type ApiMonitorReportResponse,
  type GetMonitorHistory200,
  getMonitorHistory,
  useGetAgent,
  useGetIncident,
  useGetIncidents,
  useGetMonitor,
  useGetMonitorUptime,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";

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

const getNextOffset = (lastPage: GetMonitorHistory200) => {
  const offset = lastPage.offset ?? 0;
  const limit = lastPage.limit ?? HISTORY_LIMIT;
  const count = lastPage.count ?? 0;
  const nextOffset = offset + limit;
  return nextOffset < count ? nextOffset : undefined;
};

const formatUptime = (value?: number) => (typeof value === "number" ? `${value.toFixed(1)}%` : "—");

export const MonitorDetailPage = () => {
  const { monitorId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const highlightedIncidentId = searchParams.get("incident") ?? "";
  const monitorResponse = useGetMonitor(monitorId);
  const uptimeResponse = useGetMonitorUptime(monitorId, { period: "90d" });
  const historyQuery = useInfiniteQuery({
    queryKey: ["monitor-history", monitorId, HISTORY_LIMIT],
    queryFn: ({ pageParam, signal }) =>
      getMonitorHistory(monitorId, { limit: HISTORY_LIMIT, offset: pageParam }, { signal }),
    initialPageParam: 0,
    getNextPageParam: getNextOffset,
    enabled: monitorId !== "",
  });
  const incidentsResponse = useGetIncidents({ limit: 100 });
  const highlightedIncidentResponse = useGetIncident(highlightedIncidentId);

  const monitor = monitorResponse.data?.monitor;
  const parentAgentResponse = useGetAgent(monitor?.agent_id ?? "");
  const reports = historyQuery.data?.pages.flatMap((page) => page.reports ?? []) ?? [];
  const latestReport = monitorResponse.data?.recent_reports?.[0] ?? reports[0];
  const latestPayload = parsePayload(latestReport?.payload);
  const uptimeBuckets = uptimeResponse.data?.daily_buckets ?? [];
  const recentUptimeBuckets = uptimeBuckets.slice(-7);
  const health =
    monitorResponse.data?.computed_health ??
    monitor?.computed_health ??
    monitor?.health ??
    "unknown";
  const relatedIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.monitor_id === monitorId,
  );
  const highlightedIncidentFromList = relatedIncidents.find(
    (incident) => incident.id === highlightedIncidentId,
  );
  const highlightedIncident =
    highlightedIncidentResponse.data?.incident?.monitor_id === monitorId
      ? highlightedIncidentResponse.data.incident
      : highlightedIncidentFromList;
  const rawPayload = latestReport?.payload ?? "No payload recorded.";
  const { fetchNextPage } = historyQuery;
  const loadMoreHistory = useCallback(() => {
    void fetchNextPage();
  }, [fetchNextPage]);

  if (monitorResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading monitor...</div>;
  }

  if (monitorResponse.error || !monitor) {
    return <div className="py-3 text-sm">Unable to load monitor.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <PageBreadcrumbs
          items={[
            { label: "Agents", to: "/agents" },
            {
              label: parentAgentResponse.data?.agent?.name ?? "Agent",
              to: `/agents/${monitor.agent_id}`,
            },
            { label: monitor.name ?? "Monitor" },
          ]}
        />
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

      {highlightedIncident && (
        <section className="space-y-1 bg-amber-50 px-3 py-2 text-sm">
          <div className="font-medium">
            Highlighted incident: {highlightedIncident.title ?? highlightedIncident.id}
          </div>
          <div className="text-neutral-700">
            {highlightedIncident.latest_event ?? "No latest event recorded."}
          </div>
        </section>
      )}

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
        <h2 className="text-sm font-medium">Uptime</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem
            label="90d uptime"
            value={formatUptime(uptimeResponse.data?.uptime_percent)}
          />
          <DetailItem label="days sampled" value={uptimeBuckets.length} />
        </div>
        {recentUptimeBuckets.length > 0 && (
          <div className="grid gap-2 sm:grid-cols-7">
            {recentUptimeBuckets.map((bucket) => (
              <div key={bucket.date} className="text-sm">
                <div className="text-neutral-600">{bucket.date ?? "—"}</div>
                <div className="font-medium">{formatUptime(bucket.uptime_percent)}</div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Check History</h2>
        {historyQuery.isLoading && (
          <div className="text-sm text-neutral-600">Loading check history...</div>
        )}
        {historyQuery.error && <div className="text-sm">Unable to load check history.</div>}
        {!historyQuery.isLoading && !historyQuery.error && reports.length === 0 && (
          <div className="text-sm text-neutral-600">No check history recorded.</div>
        )}
        {reports.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Latency</TableHead>
                <TableHead>Result</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {reports.map((report, index) => {
                const payload = parsePayload(report.payload);
                return (
                  <TableRow key={report.id ?? index}>
                    <TableCell className="font-medium">
                      {formatDate(report.created_at ?? report.collected_at, DATE_TIME_FORMAT)}
                    </TableCell>
                    <TableCell>{report.health ?? "unknown"}</TableCell>
                    <TableCell>{formatLatency(payload)}</TableCell>
                    <TableCell className="max-w-[22rem] truncate text-neutral-600">
                      {payloadSummary(report)}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
        <InfiniteScrollSentinel
          hasNextPage={Boolean(historyQuery.hasNextPage)}
          isFetchingNextPage={historyQuery.isFetchingNextPage}
          onLoadMore={loadMoreHistory}
        />
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Related Incidents</h2>
        {incidentsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading related incidents...</div>
        )}
        {incidentsResponse.error && (
          <div className="text-sm">Unable to load related incidents.</div>
        )}
        {!incidentsResponse.isLoading &&
          !incidentsResponse.error &&
          relatedIncidents.length === 0 && (
            <div className="text-sm text-neutral-600">No related incidents recorded.</div>
          )}
        {relatedIncidents.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Incident</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Opened</TableHead>
                <TableHead>Latest event</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {relatedIncidents.map((incident) => (
                <TableRow
                  key={incident.id}
                  className={cn(incident.id === highlightedIncidentId && "bg-amber-50")}
                >
                  <TableCell className="font-medium">
                    <Link to={`/incidents/${incident.id}`} className="hover:text-neutral-600">
                      {incident.title ?? incident.id ?? "Untitled incident"}
                    </Link>
                  </TableCell>
                  <TableCell>{incident.status ?? "unknown"}</TableCell>
                  <TableCell>{formatDate(incident.opened_at, DATE_TIME_FORMAT)}</TableCell>
                  <TableCell className="max-w-[22rem] truncate text-neutral-600">
                    {incident.latest_event ?? "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
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
