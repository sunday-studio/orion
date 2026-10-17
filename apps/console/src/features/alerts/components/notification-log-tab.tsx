import {
  DataTable,
  type DataTableFilterField,
  DataTableLink,
  FilterBar,
  ListPagination,
  StatusBadge,
} from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type { ApiAlertChannelResponse, ApiAlertDeliveryResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import { DELIVERY_LIMIT, deliveryEventOptions, deliveryTypeOptions } from "./alert-constants";

const deliveryColumns: ColumnDef<ApiAlertDeliveryResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "channel",
    header: "Destination",
    cell: ({ row }) => row.original.channel ?? row.original.type ?? "none",
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
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
    accessorKey: "attempt_count",
    header: "Attempts",
    cell: ({ row }) =>
      `${row.original.attempt_count ?? 0}/${row.original.max_attempts ?? row.original.attempt_count ?? 0}`,
  },
  {
    accessorKey: "incident_id",
    header: "Incident",
    cell: ({ row }) =>
      row.original.incident_id ? (
        <DataTableLink to={`/incidents/${row.original.incident_id}`}>incident</DataTableLink>
      ) : (
        "—"
      ),
  },
  {
    accessorKey: "error",
    header: "Error",
    cell: ({ row }) => (
      <div className="max-w-[28rem] text-neutral-600">
        <div className="truncate">{row.original.error ?? "—"}</div>
        {row.original.next_attempt_at && (
          <div className="text-xs">
            Next retry {formatDate(row.original.next_attempt_at, DATE_TIME_FORMAT)}
          </div>
        )}
        {row.original.last_attempt_at && (
          <div className="text-xs">
            Last attempt {formatDate(row.original.last_attempt_at, DATE_TIME_FORMAT)}
          </div>
        )}
      </div>
    ),
  },
];

const statusOptions = [
  { value: "all", label: "All statuses" },
  { value: "pending", label: "Pending" },
  { value: "sent", label: "Sent" },
  { value: "failed", label: "Failed" },
  { value: "suppressed", label: "Suppressed" },
  { value: "cooldown", label: "Cooldown" },
] as const;

type NotificationLogTabProps = {
  channel: string;
  channels: ApiAlertChannelResponse[];
  count: number;
  deliveries: ApiAlertDeliveryResponse[];
  error: unknown;
  eventType: string;
  incident: string;
  isLoading: boolean;
  offset: number;
  onChannelChange: (value: string) => void;
  onClearFilters: () => void;
  onEventTypeChange: (value: string) => void;
  onIncidentChange: (value: string) => void;
  onOffsetChange: (offset: number) => void;
  onRetry: () => void;
  onStatusChange: (status: string) => void;
  onTypeChange: (type: string) => void;
  status: string;
  type: string;
};

export const NotificationLogTab = ({
  channel,
  channels,
  count,
  deliveries,
  error,
  eventType,
  incident,
  isLoading,
  offset,
  onChannelChange,
  onClearFilters,
  onEventTypeChange,
  onIncidentChange,
  onOffsetChange,
  onRetry,
  onStatusChange,
  onTypeChange,
  status,
  type,
}: NotificationLogTabProps) => {
  const channelOptions = useMemo(() => {
    const names = new Set(
      channels
        .map((alertChannel) => alertChannel.name?.trim())
        .filter((name): name is string => Boolean(name)),
    );
    if (channel !== "all") names.add(channel);

    return [
      { value: "all", label: "All destinations" },
      ...Array.from(names)
        .sort((first, second) => first.localeCompare(second))
        .map((name) => ({ value: name, label: name })),
    ];
  }, [channel, channels]);
  const fields = useMemo<readonly DataTableFilterField[]>(
    () => [
      { type: "select", key: "status", label: "Status", options: statusOptions },
      { type: "select", key: "type", label: "Type", options: deliveryTypeOptions },
      { type: "select", key: "event", label: "Event", options: deliveryEventOptions },
      { type: "select", key: "channel", label: "Destination", options: channelOptions },
      { type: "input", key: "incident", placeholder: "Filter by incident ID" },
    ],
    [channelOptions],
  );
  const values = { status, type, event: eventType, channel, incident };
  const handlers: Record<string, (value: string) => void> = {
    status: onStatusChange,
    type: onTypeChange,
    event: onEventTypeChange,
    channel: onChannelChange,
    incident: onIncidentChange,
  };
  const hasFilters =
    status !== "all" ||
    type !== "all" ||
    eventType !== "all" ||
    channel !== "all" ||
    Boolean(incident.trim());

  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Notification Log</h2>
        <p className="text-sm text-neutral-600">
          Delivery attempts generated when incidents open or recover.
        </p>
      </div>
      <FilterBar
        fields={fields}
        values={values}
        onChange={(key, value) => handlers[key]?.(value)}
        onClear={onClearFilters}
      />
      <DataTable
        columns={deliveryColumns}
        data={deliveries}
        emptyMessage={
          hasFilters
            ? "No deliveries match the current filters."
            : "No notification deliveries recorded."
        }
        getRowId={(delivery, index) => delivery.id ?? `delivery-${index}`}
        isLoading={isLoading}
        error={
          error
            ? {
                message: "Unable to load notification log",
                onRetry,
              }
            : undefined
        }
        onResetFilters={hasFilters ? onClearFilters : undefined}
      />
      {count > 0 && (
        <ListPagination
          count={count}
          limit={DELIVERY_LIMIT}
          offset={offset}
          onOffsetChange={onOffsetChange}
        />
      )}
    </section>
  );
};
