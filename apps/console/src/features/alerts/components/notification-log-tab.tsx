import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { NotificationBadge, toNotificationStatus } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiAlertChannelResponse, ApiAlertDeliveryResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import {
  DELIVERY_LIMIT,
  deliveryEventOptions,
  deliveryTypeOptions,
  eventLabel,
} from "./alert-constants";

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
    cell: ({ row }) => <NotificationBadge value={toNotificationStatus(row.original.status)} />,
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
  onStatusChange,
  onTypeChange,
  status,
  type,
}: NotificationLogTabProps) => {
  const statusLabel = statusOptions.find((option) => option.value === status)?.label ?? status;
  const typeLabel = deliveryTypeOptions.find((option) => option.value === type)?.label ?? type;
  const eventTypeLabel =
    deliveryEventOptions.find((option) => option.value === eventType)?.label ??
    eventLabel(eventType);
  const channelOptions = useMemo(() => {
    const names = new Set(
      channels
        .map((alertChannel) => alertChannel.name?.trim())
        .filter((name): name is string => Boolean(name)),
    );
    if (channel !== "all") names.add(channel);

    return Array.from(names)
      .sort((first, second) => first.localeCompare(second))
      .map((name) => ({ value: name, label: name }));
  }, [channel, channels]);
  const channelLabel =
    channel === "all"
      ? "All destinations"
      : (channelOptions.find((option) => option.value === channel)?.label ?? channel);
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
      <div className="flex flex-wrap items-center gap-2">
        <Select value={status} onValueChange={onStatusChange}>
          <SelectTrigger className="w-52">
            <span data-slot="select-value">Status: {statusLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {statusOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={type} onValueChange={onTypeChange}>
          <SelectTrigger className="w-48">
            <span data-slot="select-value">Type: {typeLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {deliveryTypeOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={eventType} onValueChange={onEventTypeChange}>
          <SelectTrigger className="w-56">
            <span data-slot="select-value">Event: {eventTypeLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {deliveryEventOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={channel} onValueChange={onChannelChange}>
          <SelectTrigger className="w-60">
            <span data-slot="select-value">Destination: {channelLabel}</span>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All destinations</SelectItem>
            {channelOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          value={incident}
          onChange={(event) => onIncidentChange(event.target.value)}
          placeholder="Filter by incident ID"
          className="w-full max-w-sm sm:w-72"
        />
        {hasFilters && (
          <Button type="button" variant="ghost" size="sm" onClick={onClearFilters}>
            Clear
          </Button>
        )}
      </div>
      {Boolean(error) && (
        <EmptyState
          className="min-h-40"
          title="Unable to load notification log"
          description="Retry from the browser after Core is reachable."
          tone="error"
        />
      )}
      {!error && (
        <DataTable
          columns={deliveryColumns}
          data={deliveries}
          emptyMessage="No notification deliveries recorded."
          getRowId={(delivery, index) => delivery.id ?? `delivery-${index}`}
          isLoading={isLoading}
          loadingMessage="Loading notification log..."
        />
      )}
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
