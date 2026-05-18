import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { PageHeader } from "@/components/page-header";
import {
  NotificationBadge,
  SeverityBadge,
  toNotificationStatus,
  toSeverity,
} from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAlertChannelResponse,
  type ApiAlertDeliveryResponse,
  type ApiAlertRuleResponse,
  useCreateAlertChannel,
  useGetAlertChannels,
  useGetAlertDeliveries,
  useGetAlertRules,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ColumnDef } from "@tanstack/react-table";
import { Plus } from "lucide-react";
import { parseAsInteger, parseAsString, parseAsStringLiteral, useQueryStates } from "nuqs";
import { type FormEvent, useState } from "react";

const DELIVERY_LIMIT = 30;
const alertTabs = ["logs", "channels", "rules"] as const;
const deliveryStatuses = ["all", "pending", "sent", "failed", "suppressed", "cooldown"] as const;

const boolLabel = (value?: boolean) => (value ? "yes" : "no");

const getMutationErrorMessage = (error: unknown) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? "Unable to create webhook.");
  }
  return "Unable to create webhook.";
};

const configuredParts = (channel: {
  webhook_configured?: boolean;
  email_to_configured?: boolean;
  email_from_configured?: boolean;
  smtp_host_configured?: boolean;
  smtp_port_configured?: boolean;
  smtp_username_configured?: boolean;
}) => {
  const parts = [];
  if (channel.webhook_configured) parts.push("webhook url");
  if (channel.email_to_configured) parts.push("email to");
  if (channel.email_from_configured) parts.push("email from");
  if (channel.smtp_host_configured) parts.push("smtp host");
  if (channel.smtp_port_configured) parts.push("smtp port");
  if (channel.smtp_username_configured) parts.push("smtp username");
  return parts.length > 0 ? parts.join(", ") : "no endpoint configured";
};

const channelColumns: ColumnDef<ApiAlertChannelResponse>[] = [
  {
    accessorKey: "name",
    header: "Name",
    cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
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
      <div className="max-w-[22rem] truncate text-neutral-600">{configuredParts(row.original)}</div>
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
];

const ruleColumns: ColumnDef<ApiAlertRuleResponse>[] = [
  {
    accessorKey: "name",
    header: "Name",
    cell: ({ row }) => <span className="font-medium">{row.original.name ?? "unnamed"}</span>,
  },
  {
    accessorKey: "trigger_condition",
    header: "Trigger",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">
        {row.original.trigger_condition ?? "—"}
      </div>
    ),
  },
  {
    accessorKey: "severity",
    header: "Severity",
    cell: ({ row }) => <SeverityBadge value={toSeverity(row.original.severity)} />,
  },
  {
    accessorKey: "cooldown_seconds",
    header: "Cooldown",
    cell: ({ row }) => `${row.original.cooldown_seconds ?? 0}s`,
  },
  {
    accessorKey: "recovery_notification_enabled",
    header: "Recovery",
    cell: ({ row }) => boolLabel(row.original.recovery_notification_enabled),
  },
  {
    accessorKey: "target_channels",
    header: "Channels",
    cell: ({ row }) => (row.original.target_channels ?? []).join(", ") || "none",
  },
];

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

export const AlertsPage = () => {
  const [isCreateWebhookOpen, setIsCreateWebhookOpen] = useState(false);
  const [webhookName, setWebhookName] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookEnabled, setWebhookEnabled] = useState(true);
  const [{ page, status, incident, tab }, setDeliveryQuery] = useQueryStates({
    page: parseAsInteger.withDefault(1),
    status: parseAsStringLiteral(deliveryStatuses).withDefault("all"),
    incident: parseAsString.withDefault(""),
    tab: parseAsStringLiteral(alertTabs).withDefault("logs"),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * DELIVERY_LIMIT;
  const channelsResponse = useGetAlertChannels();
  const rulesResponse = useGetAlertRules();
  const createWebhook = useCreateAlertChannel({
    mutation: {
      onSuccess: () => {
        setWebhookName("");
        setWebhookUrl("");
        setWebhookEnabled(true);
        setIsCreateWebhookOpen(false);
        void channelsResponse.refetch();
        void rulesResponse.refetch();
      },
    },
  });
  const deliveriesQuery = useGetAlertDeliveries({
    limit: DELIVERY_LIMIT,
    offset,
    status: status === "all" ? undefined : status,
    incident_id: incident.trim() || undefined,
  });
  const deliveries = deliveriesQuery.data?.deliveries ?? [];
  const deliveryCount = deliveriesQuery.data?.count ?? deliveries.length;
  const setOffset = (nextOffset: number) => {
    void setDeliveryQuery({ page: Math.floor(nextOffset / DELIVERY_LIMIT) + 1 });
  };
  const setStatus = (nextStatus: string) => {
    if (!deliveryStatuses.includes(nextStatus as (typeof deliveryStatuses)[number])) return;
    void setDeliveryQuery({ status: nextStatus as (typeof deliveryStatuses)[number], page: 1 });
  };
  const setIncident = (nextIncident: string) => {
    void setDeliveryQuery({ incident: nextIncident, page: 1 });
  };
  const setTab = (nextTab: string) => {
    if (!alertTabs.includes(nextTab as (typeof alertTabs)[number])) return;
    void setDeliveryQuery({ tab: nextTab as (typeof alertTabs)[number] });
  };
  const createWebhookError = getMutationErrorMessage(createWebhook.error);
  const handleCreateWebhook = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = webhookName.trim();
    const url = webhookUrl.trim();
    if (!name || !url || createWebhook.isPending) return;
    createWebhook.mutate({
      data: {
        name,
        type: "webhook",
        enabled: webhookEnabled,
        webhook_url: url,
      },
    });
  };

  return (
    <div className="space-y-7">
      <PageHeader
        title="Alerts"
        description="Review notification channels, effective alert behavior, and delivery attempts."
      />

      <Tabs value={tab} onValueChange={setTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="logs">Notification Log</TabsTrigger>
          <TabsTrigger value="channels">Channels</TabsTrigger>
          <TabsTrigger value="rules">Rules</TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-medium">Notification Log</h2>
              <p className="text-sm text-neutral-600">
                Delivery attempts generated when incidents open or recover.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Select value={status} onValueChange={setStatus}>
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
                onChange={(event) => setIncident(event.target.value)}
                placeholder="Filter by incident ID"
                className="w-full max-w-sm sm:w-72"
              />
            </div>
            {deliveriesQuery.error && (
              <div className="text-sm">Unable to load notification log.</div>
            )}
            {!deliveriesQuery.error && (
              <DataTable
                columns={deliveryColumns}
                data={deliveries}
                emptyMessage="No notification deliveries recorded."
                getRowId={(delivery, index) => delivery.id ?? `delivery-${index}`}
                isLoading={deliveriesQuery.isLoading}
                loadingMessage="Loading notification log..."
              />
            )}
            {deliveryCount > 0 && (
              <ListPagination
                count={deliveryCount}
                limit={DELIVERY_LIMIT}
                offset={offset}
                onOffsetChange={setOffset}
              />
            )}
          </section>
        </TabsContent>

        <TabsContent value="channels">
          <section className="space-y-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-sm font-medium">Channels</h2>
                <p className="text-sm text-neutral-600">
                  Secrets are hidden. Add webhooks here and Core stores them for delivery.
                </p>
              </div>
              <Button size="sm" onClick={() => setIsCreateWebhookOpen(true)}>
                <Plus />
                New webhook
              </Button>
            </div>
            {channelsResponse.error && (
              <div className="text-sm">Unable to load alert channels.</div>
            )}
            {!channelsResponse.error && (
              <DataTable
                columns={channelColumns}
                data={channelsResponse.data?.channels ?? []}
                emptyMessage="No alert channels configured."
                getRowId={(channel, index) => channel.name ?? channel.type ?? `channel-${index}`}
                isLoading={channelsResponse.isLoading}
                loadingMessage="Loading alert channels..."
              />
            )}
          </section>
        </TabsContent>

        <TabsContent value="rules">
          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-medium">Rules</h2>
              <p className="text-sm text-neutral-600">
                These are the effective alert rules Core applies during incident reconciliation.
              </p>
            </div>
            {rulesResponse.error && <div className="text-sm">Unable to load alert rules.</div>}
            {!rulesResponse.error && (
              <DataTable
                columns={ruleColumns}
                data={rulesResponse.data?.rules ?? []}
                emptyMessage="No alert rules configured."
                getRowId={(rule, index) => rule.name ?? `rule-${index}`}
                isLoading={rulesResponse.isLoading}
                loadingMessage="Loading alert rules..."
              />
            )}
          </section>
        </TabsContent>
      </Tabs>

      <Dialog open={isCreateWebhookOpen} onOpenChange={setIsCreateWebhookOpen}>
        <DialogContent className="sm:max-w-md">
          <form className="space-y-5" onSubmit={handleCreateWebhook}>
            <DialogHeader>
              <DialogTitle>New webhook</DialogTitle>
              <DialogDescription>
                Add a webhook channel for incident and recovery notifications.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3">
              <label className="block space-y-1.5 text-sm">
                <span className="font-medium">Name</span>
                <Input
                  value={webhookName}
                  onChange={(event) => setWebhookName(event.target.value)}
                  placeholder="ops-webhook"
                  required
                />
              </label>
              <label className="block space-y-1.5 text-sm">
                <span className="font-medium">Webhook URL</span>
                <Input
                  value={webhookUrl}
                  onChange={(event) => setWebhookUrl(event.target.value)}
                  placeholder="https://example.com/webhook"
                  required
                  type="url"
                />
              </label>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={webhookEnabled}
                  onCheckedChange={(value) => setWebhookEnabled(value === true)}
                />
                <span>Enabled</span>
              </label>
              {createWebhookError && (
                <div className="text-sm text-red-700">{createWebhookError}</div>
              )}
            </div>

            <DialogFooter>
              <Button
                variant="ghost"
                onClick={() => setIsCreateWebhookOpen(false)}
                disabled={createWebhook.isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={createWebhook.isPending || !webhookName.trim() || !webhookUrl.trim()}
              >
                {createWebhook.isPending ? "Creating..." : "Create webhook"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
};
