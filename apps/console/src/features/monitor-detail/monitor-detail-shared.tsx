import { DataTableLink } from "@/components/data-table-link";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiIncidentResponse, ApiMonitorReportResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import type { ReactNode } from "react";
import {
  type MonitorPayload,
  formatMonitorLatency,
  parseMonitorPayload,
  readMonitorPayloadString,
  summarizeMonitorResult,
} from "@/features/monitors/monitor-result-summary";

export const HISTORY_LIMIT = 20;
export const monitorDetailTabs = ["history", "incidents", "config"] as const;
export type MonitorDetailTab = (typeof monitorDetailTabs)[number];

export const isMonitorDetailTab = (value: string | null): value is MonitorDetailTab =>
  monitorDetailTabs.includes(value as MonitorDetailTab);

export const isHeartbeatPayload = (payload: MonitorPayload) =>
  payload.type === "heartbeat" || payload.runner === "heartbeat";

export const heartbeatPayloadContext = (report?: ApiMonitorReportResponse) => {
  if (!report) return "—";
  const payload = parseMonitorPayload(report.payload);
  if (!isHeartbeatPayload(payload)) return "—";
  return (
    readMonitorPayloadString(payload, ["payload", "failure_stage", "status", "message", "error"]) ??
    "—"
  );
};

export const DetailItem = ({ label, value }: { label: string; value: ReactNode }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="break-words text-sm font-medium">{value}</div>
  </div>
);

export const DetailGroup = ({ title, children }: { title: string; children: ReactNode }) => (
  <div className="space-y-3 bg-neutral-50 px-3 py-3">
    <h2 className="text-sm font-medium">{title}</h2>
    <div className="space-y-3">{children}</div>
  </div>
);

export const formatUptime = (value?: number) =>
  typeof value === "number" ? `${value.toFixed(1)}%` : "—";

export const isCoreOwnedMonitor = (monitor?: { owner_kind?: string; source?: string }) =>
  monitor?.owner_kind === "core" || monitor?.source === "core";

export const formatJSON = (value?: Record<string, unknown>) => JSON.stringify(value ?? {}, null, 2);

export const coreConfigValue = (
  config: { config?: Record<string, unknown> } | undefined,
  key: string,
) => {
  const value = config?.config?.[key];
  if (typeof value === "number") return String(value);
  if (typeof value === "string" && value.trim() !== "") return value;
  return "—";
};

export const coreConfigArrayCount = (
  config: { config?: Record<string, unknown> } | undefined,
  key: string,
) => {
  const value = config?.config?.[key];
  return Array.isArray(value) ? value.length : 0;
};

export const reportTimestamp = (report?: ApiMonitorReportResponse) =>
  report?.created_at ?? report?.collected_at;

export const bucketFillClassName = (bucket: { total?: number; uptime_percent?: number }) => {
  const percent = bucket.uptime_percent ?? 0;
  if (!bucket.total) return "bg-neutral-300";
  if (percent >= 99) return "bg-emerald-400";
  if (percent >= 95) return "bg-amber-300";
  return "bg-rose-400";
};

export const historyColumns: ColumnDef<ApiMonitorReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.collected_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "health",
    header: "Status",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.health)} />,
  },
  {
    id: "latency",
    header: "Latency",
    cell: ({ row }) => formatMonitorLatency(parseMonitorPayload(row.original.payload)),
  },
  {
    id: "result",
    header: "Result",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">
        {summarizeMonitorResult(row.original).headline}
      </div>
    ),
  },
];

export const incidentColumns: ColumnDef<ApiIncidentResponse>[] = [
  {
    accessorKey: "title",
    header: "Incident",
    cell: ({ row }) => (
      <DataTableLink to={`/incidents/${row.original.id}`}>
        {row.original.title ?? row.original.id ?? "Untitled incident"}
      </DataTableLink>
    ),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.status)} />,
  },
  {
    accessorKey: "opened_at",
    header: "Opened",
    cell: ({ row }) => formatDate(row.original.opened_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "latest_event",
    header: "Latest event",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">
        {row.original.latest_event ?? "—"}
      </div>
    ),
  },
];
