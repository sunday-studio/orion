import { StatusBadge } from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type {
  ApiAlertDeliveryResponse,
  ApiIncidentResponse,
  ApiIncidentTimelineItemResponse,
  ApiMonitorReportResponse,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";

export type LifecycleAction = "acknowledge" | "resolve" | "reopen";

export const actionAllowed = (action?: { allowed?: boolean }) => action?.allowed === true;

export type IncidentTimelineItemWithMeta = ApiIncidentTimelineItemResponse & {
  actor_id?: string;
  actor_type?: string;
  note?: string;
};

type IncidentAllowedActions = {
  acknowledge?: { allowed?: boolean };
  cover?: { allowed?: boolean };
  reopen?: { allowed?: boolean };
  resolve?: { allowed?: boolean };
};

export type IncidentWithAllowedActions = ApiIncidentResponse & {
  allowed_actions?: IncidentAllowedActions;
};

export type MonitorPayload = Record<string, unknown>;

const parsePayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const readPayloadValue = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number") return String(value);
    if (typeof value === "boolean") return value ? "true" : "false";
  }
  return "—";
};

export const reportReason = (report?: ApiMonitorReportResponse) => {
  if (!report) return "No linked monitor report.";
  const payload = parsePayload(report.payload);
  return readPayloadValue(payload, [
    "failure_reason",
    "error",
    "message",
    "summary",
    "status",
    "status_code",
  ]);
};

export const reportSortTime = (report: ApiMonitorReportResponse) => {
  const timestamp = report.created_at ?? report.collected_at;
  const value = timestamp ? new Date(timestamp).getTime() : 0;
  return Number.isNaN(value) ? 0 : value;
};

const detailTabs = ["timeline", "notifications", "monitor-reports", "logs"] as const;
export type DetailTab = (typeof detailTabs)[number];

export const isDetailTab = (value: string | null): value is DetailTab =>
  detailTabs.includes(value as DetailTab);

export const notificationColumns: ColumnDef<ApiAlertDeliveryResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "channel",
    header: "Channel",
    cell: ({ row }) => row.original.channel ?? "none",
  },
  {
    accessorKey: "event_type",
    header: "Event",
    cell: ({ row }) => row.original.event_type ?? "unknown",
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
  {
    accessorKey: "error",
    header: "Error",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.error ?? "—"}</div>
    ),
  },
];

export const monitorReportColumns: ColumnDef<ApiMonitorReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.collected_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge status={row.original.health} />,
  },
  {
    accessorKey: "id",
    header: "Report ID",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.id ?? "—"}</div>
    ),
  },
];
