import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { NotificationBadge, toNotificationStatus } from "@/components/status-badges";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiAlertDeliveryResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { DELIVERY_LIMIT } from "./alert-constants";

const deliveryColumns: ColumnDef<ApiAlertDeliveryResponse>[] = [
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
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.error ?? "—"}</div>
    ),
  },
];

type NotificationLogTabProps = {
  count: number;
  deliveries: ApiAlertDeliveryResponse[];
  error: unknown;
  incident: string;
  isLoading: boolean;
  offset: number;
  onIncidentChange: (value: string) => void;
  onOffsetChange: (offset: number) => void;
  onStatusChange: (status: string) => void;
  status: string;
};

export const NotificationLogTab = ({
  count,
  deliveries,
  error,
  incident,
  isLoading,
  offset,
  onIncidentChange,
  onOffsetChange,
  onStatusChange,
  status,
}: NotificationLogTabProps) => (
  <section className="space-y-3">
    <div>
      <h2 className="text-sm font-medium">Notification Log</h2>
      <p className="text-sm text-neutral-600">
        Delivery attempts generated when incidents open or recover.
      </p>
    </div>
    <div className="flex flex-wrap items-center gap-2">
      <Select value={status} onValueChange={onStatusChange}>
        <SelectTrigger className="w-44">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All statuses</SelectItem>
          <SelectItem value="pending">Pending</SelectItem>
          <SelectItem value="sent">Sent</SelectItem>
          <SelectItem value="failed">Failed</SelectItem>
          <SelectItem value="suppressed">Suppressed</SelectItem>
          <SelectItem value="cooldown">Cooldown</SelectItem>
        </SelectContent>
      </Select>
      <Input
        value={incident}
        onChange={(event) => onIncidentChange(event.target.value)}
        placeholder="Filter by incident ID"
        className="w-full max-w-sm sm:w-72"
      />
    </div>
    {Boolean(error) && <div className="text-sm">Unable to load notification log.</div>}
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
