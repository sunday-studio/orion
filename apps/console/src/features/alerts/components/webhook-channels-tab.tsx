import { DataTable } from "@/components/data-table";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiAlertChannelResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { MoreHorizontal, Pencil, Send, Trash2 } from "lucide-react";
import { useMemo } from "react";
import { alertChannelTypeLabel, boolLabel, eventLabel } from "./alert-constants";

type AlertActionFeedback = {
  channelId?: string;
  message: string;
  status: "pending" | "success" | "error";
} | null;

type WebhookChannelsTabProps = {
  channels: ApiAlertChannelResponse[];
  error: unknown;
  isLoading: boolean;
  isTesting: boolean;
  onCreate: () => void;
  onDelete: (channel: ApiAlertChannelResponse) => void;
  onEdit: (channel: ApiAlertChannelResponse) => void;
  onTest: (channel: ApiAlertChannelResponse) => void;
  testFeedback: AlertActionFeedback;
  testingChannelId: string | null;
};

export const WebhookChannelsTab = ({
  channels,
  error,
  isLoading,
  isTesting,
  onCreate,
  onDelete,
  onEdit,
  onTest,
  testFeedback,
  testingChannelId,
}: WebhookChannelsTabProps) => {
  const columns = useMemo<ColumnDef<ApiAlertChannelResponse>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
      },
      {
        accessorKey: "type",
        header: "Type",
        cell: ({ row }) => alertChannelTypeLabel(row.original.type),
      },
      {
        accessorKey: "enabled",
        header: "Enabled",
        cell: ({ row }) => boolLabel(row.original.enabled),
      },
      {
        id: "configured",
        header: "Webhook",
        cell: ({ row }) => (
          <div className="max-w-88 truncate text-neutral-600">
            {row.original.webhook_configured
              ? (row.original.webhook_url ?? "webhook url")
              : "no webhook configured"}
          </div>
        ),
      },
      {
        accessorKey: "subscribed_events",
        header: "Events",
        cell: ({ row }) => (
          <div className="max-w-88 truncate text-neutral-600">
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
        cell: ({ row }) => {
          const isTestingThisChannel = isTesting && testingChannelId === row.original.id;

          return (
            <DropdownMenu>
              <DropdownMenuTrigger
                aria-label={`Open actions for ${row.original.name ?? "webhook"}`}
                className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <MoreHorizontal className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  disabled={!row.original.id || isTesting}
                  onClick={() => onTest(row.original)}
                >
                  <Send className="size-4" />
                  {isTestingThisChannel ? "Sending test..." : "Send test"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => onEdit(row.original)}>
                  <Pencil className="size-4" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onDelete(row.original)}>
                  <Trash2 className="size-4" />
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          );
        },
      },
    ],
    [isTesting, onDelete, onEdit, onTest, testingChannelId],
  );

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">Webhook Channels</h2>
          <p className="text-sm text-neutral-600">
            Generic webhooks for incident and recovery notifications.
          </p>
        </div>
        <Button size="sm" onClick={onCreate}>
          New webhook
        </Button>
      </div>
      {testFeedback && (
        <div
          className={
            testFeedback.status === "error" ? "text-sm text-red-700" : "text-sm text-neutral-600"
          }
        >
          {testFeedback.message}
        </div>
      )}
      {Boolean(error) && <div className="text-sm">Unable to load webhook channels.</div>}
      {!error && (
        <DataTable
          columns={columns}
          data={channels}
          emptyMessage="No webhook channels configured."
          getRowId={(channel, index) => channel.id ?? channel.name ?? `channel-${index}`}
          isLoading={isLoading}
          loadingMessage="Loading webhook channels..."
        />
      )}
    </section>
  );
};
