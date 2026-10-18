import {
  Button,
  DataTable,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
} from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type { ApiAlertChannelResponse } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { Mail, MoreHorizontal, Pencil, Send, Trash2 } from "lucide-react";
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
  onCreateEmail: () => void;
  onRecipients: (channel: ApiAlertChannelResponse) => void;
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
  onCreateEmail,
  onRecipients,
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
        header: "Destination",
        cell: ({ row }) => (
          <div className="max-w-88 truncate text-quiet">
            {row.original.type === "email"
              ? `${row.original.confirmed_recipient_count ?? 0} of ${row.original.recipient_count ?? 0} recipients confirmed`
              : row.original.webhook_configured
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
            <Menu>
              <MenuTrigger
                aria-label={`Open actions for ${row.original.name ?? "webhook"}`}
                className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <MoreHorizontal className="size-4" />
              </MenuTrigger>
              <MenuContent>
                <MenuItem
                  disabled={!row.original.id || isTesting}
                  onClick={() => onTest(row.original)}
                >
                  <Send className="size-4" />
                  {isTestingThisChannel ? "Sending test..." : "Send test"}
                </MenuItem>
                <MenuSeparator />
                {row.original.type === "email" && (
                  <MenuItem onClick={() => onRecipients(row.original)}>
                    <Mail className="size-4" />
                    Recipients
                  </MenuItem>
                )}
                <MenuItem
                  disabled={row.original.type === "email"}
                  onClick={() => onEdit(row.original)}
                >
                  <Pencil className="size-4" />
                  Edit
                </MenuItem>
                <MenuItem onClick={() => onDelete(row.original)}>
                  <Trash2 className="size-4" />
                  Delete
                </MenuItem>
              </MenuContent>
            </Menu>
          );
        },
      },
    ],
    [isTesting, onDelete, onEdit, onRecipients, onTest, testingChannelId],
  );

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium">Channels</h2>
          <p className="text-sm text-neutral-600">
            Webhook and email destinations for incident and recovery notifications.
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" onClick={onCreateEmail}>
            New email channel
          </Button>
          <Button size="sm" onClick={onCreate}>
            New webhook
          </Button>
        </div>
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
          emptyMessage="No channels configured."
          getRowId={(channel, index) => channel.id ?? channel.name ?? `channel-${index}`}
          isLoading={isLoading}
        />
      )}
    </section>
  );
};
