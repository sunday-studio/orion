import { DataTable } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiAlertChannelResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo } from "react";
import { boolLabel, eventLabel } from "./alert-constants";

const configuredParts = (channel: { webhook_configured?: boolean }) => {
  const parts = [];
  if (channel.webhook_configured) parts.push("webhook url");
  return parts.length > 0 ? parts.join(", ") : "no endpoint configured";
};

type WebhookChannelsTabProps = {
  channels: ApiAlertChannelResponse[];
  error: unknown;
  isLoading: boolean;
  onCreate: () => void;
  onDelete: (channel: ApiAlertChannelResponse) => void;
  onEdit: (channel: ApiAlertChannelResponse) => void;
};

export const WebhookChannelsTab = ({
  channels,
  error,
  isLoading,
  onCreate,
  onDelete,
  onEdit,
}: WebhookChannelsTabProps) => {
  const columns = useMemo<ColumnDef<ApiAlertChannelResponse>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
        cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
      },
      {
        accessorKey: "enabled",
        header: "Enabled",
        cell: ({ row }) => boolLabel(row.original.enabled),
      },
      {
        id: "configured",
        header: "Configured",
        cell: ({ row }) => (
          <div className="max-w-[22rem] truncate text-neutral-600">
            {configuredParts(row.original)}
          </div>
        ),
      },
      {
        accessorKey: "subscribed_events",
        header: "Events",
        cell: ({ row }) => (
          <div className="max-w-[22rem] truncate text-neutral-600">
            {(row.original.subscribed_events ?? []).map(eventLabel).join(", ") || "none"}
          </div>
        ),
      },
      {
        accessorKey: "last_delivery_at",
        header: "Last delivery",
        cell: ({ row }) =>
          row.original.last_delivery_status
            ? `${row.original.last_delivery_status} · ${formatDate(row.original.last_delivery_at, DATE_TIME_FORMAT)}`
            : "—",
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => (
          <div className="flex justify-end gap-1">
            <Button
              aria-label={`Edit ${row.original.name ?? "channel"}`}
              size="icon"
              variant="ghost"
              onClick={() => onEdit(row.original)}
            >
              <Pencil />
            </Button>
            <Button
              aria-label={`Delete ${row.original.name ?? "channel"}`}
              size="icon"
              variant="ghost"
              onClick={() => onDelete(row.original)}
            >
              <Trash2 />
            </Button>
          </div>
        ),
      },
    ],
    [onDelete, onEdit],
  );

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">Channels</h2>
          <p className="text-sm text-neutral-600">
            Secrets are hidden. Add webhooks here and Core stores them for delivery.
          </p>
        </div>
        <Button size="sm" onClick={onCreate}>
          <Plus />
          New webhook
        </Button>
      </div>
      {Boolean(error) && <div className="text-sm">Unable to load alert channels.</div>}
      {!error && (
        <DataTable
          columns={columns}
          data={channels}
          emptyMessage="No webhooks configured."
          getRowId={(channel, index) => channel.name ?? channel.type ?? `channel-${index}`}
          isLoading={isLoading}
          loadingMessage="Loading webhooks..."
        />
      )}
    </section>
  );
};
