import {
  NotificationBadge,
  StatusBadge,
  toNotificationStatus,
  toStatus,
} from "@/components/shared/status-badges";
import { DetailGroup, DetailItem, reportTimestamp } from "@/components/shared/detail-group";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type {
  ApiAlertDeliveryResponse,
  ApiIncidentResponse,
  ApiIncidentTimelineItemResponse,
  ApiMonitorReportResponse,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";

export { DetailGroup, DetailItem, reportTimestamp };

export const durationLabel = (incident: ApiIncidentResponse) => {
  const start = incident.opened_at ? new Date(incident.opened_at).getTime() : undefined;
  const end = incident.resolved_at ? new Date(incident.resolved_at).getTime() : Date.now();
  if (!start || Number.isNaN(start) || Number.isNaN(end)) return "—";
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
};

export const coverageUntilInputValue = () => {
  const value = new Date(Date.now() + 60 * 60 * 1000);
  const offset = value.getTimezoneOffset() * 60000;
  return new Date(value.getTime() - offset).toISOString().slice(0, 16);
};

export const coverageUntilPayload = (value: string) => {
  if (!value) return undefined;
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
};

export type LifecycleAction = "acknowledge" | "resolve" | "reopen";

export const lifecycleActionLabels: Record<LifecycleAction, string> = {
  acknowledge: "Acknowledge",
  resolve: "Resolve",
  reopen: "Reopen",
};

export const actorLabel = (actorType?: string, actorID?: string) => {
  const type = actorType?.trim();
  const id = actorID?.trim();
  if (type && id) return `${type} / ${id}`;
  return type || id || "—";
};

export const actionAllowed = (action?: { allowed?: boolean }) => action?.allowed === true;

export type IncidentTimelineItemWithMeta = ApiIncidentTimelineItemResponse & {
  actor_id?: string;
  actor_type?: string;
  note?: string;
};

export type IncidentAllowedActions = {
  acknowledge?: { allowed?: boolean };
  cover?: { allowed?: boolean };
  reopen?: { allowed?: boolean };
  resolve?: { allowed?: boolean };
};

export type IncidentWithAllowedActions = ApiIncidentResponse & {
  allowed_actions?: IncidentAllowedActions;
};

export type IncidentComponentImpact = NonNullable<
  ApiIncidentResponse["impacted_components"]
>[number];

export const componentLabel = (component: IncidentComponentImpact) =>
  component.component_name || component.component_id || "Unnamed component";

export const componentImpactLabel = (component: IncidentComponentImpact) =>
  component.impact || component.status || "";

export const ComponentImpactList = ({ components }: { components: IncidentComponentImpact[] }) => {
  if (components.length === 0) {
    return <span className="text-neutral-500">No components</span>;
  }

  return (
    <div className="space-y-1">
      {components.map((component, index) => {
        const impact = componentImpactLabel(component);
        return (
          <div
            key={`${component.component_id ?? component.component_name ?? "component"}-${index}`}
            className="min-w-0"
          >
            <div className="truncate">{componentLabel(component)}</div>
            {impact && <div className="truncate text-xs text-neutral-500">{impact}</div>}
          </div>
        );
      })}
    </div>
  );
};

export type MonitorPayload = Record<string, unknown>;

export const parsePayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

export const readPayloadValue = (payload: MonitorPayload, keys: string[]) => {
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
  const timestamp = reportTimestamp(report);
  const value = timestamp ? new Date(timestamp).getTime() : 0;
  return Number.isNaN(value) ? 0 : value;
};

export const detailTabs = ["timeline", "notifications", "monitor-reports"] as const;
export type DetailTab = (typeof detailTabs)[number];

export const isDetailTab = (value: string | null): value is DetailTab =>
  detailTabs.includes(value as DetailTab);

export const timelineColumns = (
  reportsByID: Map<string, ApiMonitorReportResponse>,
): ColumnDef<ApiIncidentTimelineItemResponse>[] => [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "source",
    header: "Source",
    cell: ({ row }) => row.original.source ?? "unknown",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
  {
    id: "actor",
    header: "Actor",
    cell: ({ row }) => {
      const item = row.original as IncidentTimelineItemWithMeta;
      return (
        <div className="max-w-[12rem] truncate text-neutral-600">
          {actorLabel(item.actor_type, item.actor_id)}
        </div>
      );
    },
  },
  {
    accessorKey: "note",
    header: "Note",
    cell: ({ row }) => {
      const item = row.original as IncidentTimelineItemWithMeta;
      return <div className="max-w-[18rem] truncate text-neutral-600">{item.note ?? "—"}</div>;
    },
  },
  {
    id: "evidence",
    header: "Evidence",
    cell: ({ row }) => {
      const report = row.original.monitor_report_id
        ? reportsByID.get(row.original.monitor_report_id)
        : undefined;
      return (
        <div className="max-w-[22rem] truncate text-neutral-600">
          {row.original.evidence ?? reportReason(report)}
        </div>
      );
    },
  },
];

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
    cell: ({ row }) => <NotificationBadge value={toNotificationStatus(row.original.status)} />,
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
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.health)} />,
  },
  {
    accessorKey: "id",
    header: "Report ID",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.id ?? "—"}</div>
    ),
  },
];
