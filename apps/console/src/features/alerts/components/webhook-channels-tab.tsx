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
import type {
  ApiAlertChannelResponse,
  ApiAlertEmailDestinationResponse,
  ApiAlertSMTPServiceResponse,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { MoreHorizontal, Pencil, Send, Trash2 } from "lucide-react";
import { useMemo } from "react";
import {
  alertChannelTypeLabel,
  boolLabel,
  eventLabel,
  manageableAlertChannelTypes,
} from "./alert-constants";

const configuredParts = (
  channel: Pick<
    ApiAlertChannelResponse,
    | "email_from_configured"
    | "email_to_configured"
    | "smtp_host_configured"
    | "smtp_port_configured"
    | "smtp_username_configured"
    | "webhook_configured"
    | "webhook_url"
  >,
) => {
  const parts = [];
  if (channel.webhook_configured) parts.push(channel.webhook_url ?? "webhook url");
  if (channel.email_to_configured) parts.push("recipient");
  if (channel.email_from_configured) parts.push("sender");
  if (channel.smtp_host_configured) parts.push("SMTP host");
  if (channel.smtp_port_configured) parts.push("SMTP port");
  if (channel.smtp_username_configured) parts.push("SMTP username");
  return parts.length > 0 ? parts.join(", ") : "no destination configured";
};

type AlertActionFeedback = {
  itemId?: string;
  message: string;
  status: "pending" | "success" | "error";
} | null;

type WebhookChannelsTabProps = {
  channels: ApiAlertChannelResponse[];
  destinations: ApiAlertEmailDestinationResponse[];
  error: unknown;
  destinationError: unknown;
  destinationFeedback: AlertActionFeedback;
  isDestinationLoading: boolean;
  isDestinationTesting: boolean;
  isLoading: boolean;
  isServiceLoading: boolean;
  isServiceTesting: boolean;
  isTesting: boolean;
  onCreate: () => void;
  onCreateDestination: () => void;
  onCreateService: () => void;
  onDelete: (channel: ApiAlertChannelResponse) => void;
  onDeleteDestination: (destination: ApiAlertEmailDestinationResponse) => void;
  onDeleteService: (service: ApiAlertSMTPServiceResponse) => void;
  onEdit: (channel: ApiAlertChannelResponse) => void;
  onEditDestination: (destination: ApiAlertEmailDestinationResponse) => void;
  onEditService: (service: ApiAlertSMTPServiceResponse) => void;
  onTestDestination: (destination: ApiAlertEmailDestinationResponse) => void;
  onTestService: (service: ApiAlertSMTPServiceResponse) => void;
  onTest: (channel: ApiAlertChannelResponse) => void;
  testFeedback: {
    channelId?: string;
    message: string;
    status: "pending" | "success" | "error";
  } | null;
  serviceError: unknown;
  serviceFeedback: AlertActionFeedback;
  services: ApiAlertSMTPServiceResponse[];
  testingDestinationId: string | null;
  testingServiceId: string | null;
  testingChannelId: string | null;
};

export const WebhookChannelsTab = ({
  channels,
  destinations,
  error,
  destinationError,
  destinationFeedback,
  isDestinationLoading,
  isDestinationTesting,
  isLoading,
  isServiceLoading,
  isServiceTesting,
  isTesting,
  onCreate,
  onCreateDestination,
  onCreateService,
  onDelete,
  onDeleteDestination,
  onDeleteService,
  onEdit,
  onEditDestination,
  onEditService,
  onTest,
  onTestDestination,
  onTestService,
  testFeedback,
  serviceError,
  serviceFeedback,
  services,
  testingDestinationId,
  testingServiceId,
  testingChannelId,
}: WebhookChannelsTabProps) => {
  const serviceColumns = useMemo<ColumnDef<ApiAlertSMTPServiceResponse>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
      },
      {
        accessorKey: "host",
        header: "Host",
        cell: ({ row }) => (
          <div className="max-w-72 truncate text-neutral-600">
            {[row.original.host, row.original.port].filter(Boolean).join(":") || "—"}
          </div>
        ),
      },
      {
        accessorKey: "from_email",
        header: "From",
        cell: ({ row }) => (
          <div className="max-w-72 truncate text-neutral-600">{row.original.from_email ?? "—"}</div>
        ),
      },
      {
        accessorKey: "enabled",
        header: "Enabled",
        cell: ({ row }) => boolLabel(row.original.enabled),
      },
      {
        id: "auth",
        header: "Auth",
        cell: ({ row }) => {
          const parts = [];
          if (row.original.username_configured) parts.push("username");
          if (row.original.password_configured) parts.push("password stored");
          return parts.length ? parts.join(", ") : "none";
        },
      },
      {
        accessorKey: "updated_at",
        header: "Updated",
        cell: ({ row }) => formatDate(row.original.updated_at, DATE_TIME_FORMAT),
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => {
          const isTestingThisService = isServiceTesting && testingServiceId === row.original.id;

          return (
            <DropdownMenu>
              <DropdownMenuTrigger
                aria-label={`Open actions for ${row.original.name ?? "SMTP service"}`}
                className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <MoreHorizontal className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  disabled={!row.original.id || isServiceTesting}
                  onClick={() => onTestService(row.original)}
                >
                  <Send className="size-4" />
                  {isTestingThisService ? "Testing..." : "Test connection"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => onEditService(row.original)}>
                  <Pencil className="size-4" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onDeleteService(row.original)}>
                  <Trash2 className="size-4" />
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          );
        },
      },
    ],
    [isServiceTesting, onDeleteService, onEditService, onTestService, testingServiceId],
  );

  const destinationColumns = useMemo<ColumnDef<ApiAlertEmailDestinationResponse>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Name",
      },
      {
        accessorKey: "email_to",
        header: "Recipient",
        cell: ({ row }) => (
          <div className="max-w-72 truncate text-neutral-600">{row.original.email_to ?? "—"}</div>
        ),
      },
      {
        accessorKey: "smtp_service_name",
        header: "SMTP service",
        cell: ({ row }) =>
          row.original.smtp_service_name ??
          services.find((service) => service.id === row.original.smtp_service_id)?.name ??
          "—",
      },
      {
        accessorKey: "enabled",
        header: "Enabled",
        cell: ({ row }) => boolLabel(row.original.enabled),
      },
      {
        accessorKey: "subscribed_events",
        header: "Events",
        cell: ({ row }) => (
          <div className="max-w-80 truncate text-neutral-600">
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
          const isTestingThisDestination =
            isDestinationTesting && testingDestinationId === row.original.id;

          return (
            <DropdownMenu>
              <DropdownMenuTrigger
                aria-label={`Open actions for ${row.original.name ?? "email destination"}`}
                className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <MoreHorizontal className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  disabled={!row.original.id || isDestinationTesting}
                  onClick={() => onTestDestination(row.original)}
                >
                  <Send className="size-4" />
                  {isTestingThisDestination ? "Sending test..." : "Send test"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => onEditDestination(row.original)}>
                  <Pencil className="size-4" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onDeleteDestination(row.original)}>
                  <Trash2 className="size-4" />
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          );
        },
      },
    ],
    [
      isDestinationTesting,
      onDeleteDestination,
      onEditDestination,
      onTestDestination,
      services,
      testingDestinationId,
    ],
  );

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
        header: "Configured",
        cell: ({ row }) => (
          <div className="max-w-88 truncate text-neutral-600">{configuredParts(row.original)}</div>
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
          const canManageChannel = manageableAlertChannelTypes.some(
            (type) => type === row.original.type,
          );
          const canTest = canManageChannel || row.original.type === "email";
          const isTestingThisChannel = isTesting && testingChannelId === row.original.id;

          return canManageChannel || canTest ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                aria-label={`Open actions for ${row.original.name ?? "channel"}`}
                className="ml-auto flex size-6 items-center justify-center hover:bg-accent focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <MoreHorizontal className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                {canTest && (
                  <DropdownMenuItem
                    disabled={!row.original.id || isTesting}
                    onClick={() => onTest(row.original)}
                  >
                    <Send className="size-4" />
                    {isTestingThisChannel ? "Sending test..." : "Send test"}
                  </DropdownMenuItem>
                )}
                {canTest && canManageChannel && <DropdownMenuSeparator />}
                {canManageChannel && (
                  <>
                    <DropdownMenuItem onClick={() => onEdit(row.original)}>
                      <Pencil className="size-4" />
                      Edit
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onDelete(row.original)}>
                      <Trash2 className="size-4" />
                      Delete
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null;
        },
      },
    ],
    [isTesting, onDelete, onEdit, onTest, testingChannelId],
  );

  return (
    <section className="space-y-7">
      <div className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium">Email Destinations</h2>
            <p className="text-sm text-neutral-600">
              Reusable email recipients that send through an SMTP service.
            </p>
          </div>
          <Button size="sm" onClick={onCreateDestination}>
            New email destination
          </Button>
        </div>
        {destinationFeedback && (
          <div
            className={
              destinationFeedback.status === "error"
                ? "text-sm text-red-700"
                : "text-sm text-neutral-600"
            }
          >
            {destinationFeedback.message}
          </div>
        )}
        {Boolean(destinationError) && (
          <div className="text-sm">Unable to load email destinations.</div>
        )}
        {!destinationError && (
          <DataTable
            columns={destinationColumns}
            data={destinations}
            emptyMessage="No email destinations configured."
            getRowId={(destination, index) => destination.id ?? `email-destination-${index}`}
            isLoading={isDestinationLoading}
            loadingMessage="Loading email destinations..."
          />
        )}
      </div>

      <div className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium">SMTP Services</h2>
            <p className="text-sm text-neutral-600">
              Reusable SMTP connection settings. Secret values are stored by Core and hidden here.
            </p>
          </div>
          <Button size="sm" onClick={onCreateService}>
            New SMTP service
          </Button>
        </div>
        {serviceFeedback && (
          <div
            className={
              serviceFeedback.status === "error"
                ? "text-sm text-red-700"
                : "text-sm text-neutral-600"
            }
          >
            {serviceFeedback.message}
          </div>
        )}
        {Boolean(serviceError) && <div className="text-sm">Unable to load SMTP services.</div>}
        {!serviceError && (
          <DataTable
            columns={serviceColumns}
            data={services}
            emptyMessage="No SMTP services configured."
            getRowId={(service, index) => service.id ?? `smtp-service-${index}`}
            isLoading={isServiceLoading}
            loadingMessage="Loading SMTP services..."
          />
        )}
      </div>

      <div className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium">Channels</h2>
            <p className="text-sm text-neutral-600">
              Review alert destinations Core can deliver to. Webhook-backed destinations can be
              managed here.
            </p>
          </div>
          <Button size="sm" onClick={onCreate}>
            New destination
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
        {Boolean(error) && <div className="text-sm">Unable to load alert channels.</div>}
        {!error && (
          <DataTable
            columns={columns}
            data={channels}
            emptyMessage="No alert destinations configured."
            getRowId={(channel, index) => channel.name ?? channel.type ?? `channel-${index}`}
            isLoading={isLoading}
            loadingMessage="Loading alert destinations..."
          />
        )}
      </div>
    </section>
  );
};
