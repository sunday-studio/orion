import { PageHeader } from "@/components/page-header";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAlertChannelResponse,
  useCreateAlertChannel,
  useDeleteAlertChannel,
  useGetAlertChannels,
  useGetAlertDeliveries,
  useGetAlertRules,
  useUpdateAlertChannel,
} from "@/orion-sdk";
import { parseAsInteger, parseAsString, parseAsStringLiteral, useQueryStates } from "nuqs";
import { type FormEvent, useMemo, useState } from "react";
import { AlertRulesTab } from "./components/alert-rules-tab";
import {
  DELIVERY_LIMIT,
  alertTabs,
  defaultAlertEvents,
  deliveryStatuses,
  getMutationErrorMessage,
} from "./components/alert-constants";
import { DeleteWebhookDialog } from "./components/delete-webhook-dialog";
import { NotificationLogTab } from "./components/notification-log-tab";
import { WebhookChannelsTab } from "./components/webhook-channels-tab";
import { WebhookDialog } from "./components/webhook-dialog";

export const AlertsPage = () => {
  const [isWebhookDialogOpen, setIsWebhookDialogOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<ApiAlertChannelResponse | null>(null);
  const [deletingChannel, setDeletingChannel] = useState<ApiAlertChannelResponse | null>(null);
  const [webhookName, setWebhookName] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookEnabled, setWebhookEnabled] = useState(true);
  const [webhookEvents, setWebhookEvents] = useState<string[]>(defaultAlertEvents);
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
  const deliveriesQuery = useGetAlertDeliveries({
    limit: DELIVERY_LIMIT,
    offset,
    status: status === "all" ? undefined : status,
    incident_id: incident.trim() || undefined,
  });

  const refreshAlertConfiguration = () => {
    void channelsResponse.refetch();
    void rulesResponse.refetch();
  };
  const closeWebhookDialog = () => {
    setWebhookName("");
    setWebhookUrl("");
    setWebhookEnabled(true);
    setWebhookEvents(defaultAlertEvents);
    setEditingChannel(null);
    setIsWebhookDialogOpen(false);
  };

  const createWebhook = useCreateAlertChannel({
    mutation: {
      onSuccess: () => {
        closeWebhookDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const updateWebhook = useUpdateAlertChannel({
    mutation: {
      onSuccess: () => {
        closeWebhookDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const deleteWebhook = useDeleteAlertChannel({
    mutation: {
      onSuccess: () => {
        setDeletingChannel(null);
        refreshAlertConfiguration();
      },
    },
  });

  const webhookChannels = useMemo(
    () => (channelsResponse.data?.channels ?? []).filter((channel) => channel.type === "webhook"),
    [channelsResponse.data?.channels],
  );
  const webhookChannelNames = useMemo(
    () => new Set(webhookChannels.flatMap((channel) => (channel.name ? [channel.name] : []))),
    [webhookChannels],
  );
  const displayedRules = useMemo(
    () =>
      (rulesResponse.data?.rules ?? []).map((rule) => ({
        ...rule,
        target_channels: (rule.target_channels ?? []).filter((channel) =>
          webhookChannelNames.has(channel),
        ),
      })),
    [rulesResponse.data?.rules, webhookChannelNames],
  );
  const deliveries = (deliveriesQuery.data?.deliveries ?? []).filter(
    (delivery) => delivery.type !== "email",
  );
  const deliveryCount = deliveriesQuery.data?.count ?? deliveries.length;
  const isEditingWebhook = Boolean(editingChannel);
  const isWebhookPending = createWebhook.isPending || updateWebhook.isPending;
  const webhookMutationError = getMutationErrorMessage(
    createWebhook.error ?? updateWebhook.error ?? deleteWebhook.error,
  );

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

  const openCreateWebhookDialog = () => {
    setEditingChannel(null);
    setWebhookName("");
    setWebhookUrl("");
    setWebhookEnabled(true);
    setWebhookEvents(defaultAlertEvents);
    setIsWebhookDialogOpen(true);
  };
  const openEditWebhookDialog = (channel: ApiAlertChannelResponse) => {
    setEditingChannel(channel);
    setWebhookName(channel.name ?? "");
    setWebhookUrl("");
    setWebhookEnabled(channel.enabled ?? true);
    setWebhookEvents(
      channel.subscribed_events?.length ? channel.subscribed_events : defaultAlertEvents,
    );
    setIsWebhookDialogOpen(true);
  };
  const toggleWebhookEvent = (event: string, enabled: boolean) => {
    setWebhookEvents((current) => {
      if (enabled) return Array.from(new Set([...current, event]));
      return current.filter((item) => item !== event);
    });
  };
  const handleWebhookSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = webhookName.trim();
    const url = webhookUrl.trim();
    if (!name || isWebhookPending) return;
    if (!isEditingWebhook && !url) return;
    if (webhookEvents.length === 0) return;
    if (editingChannel?.id) {
      updateWebhook.mutate({
        id: editingChannel.id,
        data: {
          name,
          type: "webhook",
          enabled: webhookEnabled,
          subscribed_events: webhookEvents,
          ...(url ? { webhook_url: url } : {}),
        },
      });
      return;
    }
    createWebhook.mutate({
      data: {
        name,
        type: "webhook",
        enabled: webhookEnabled,
        webhook_url: url,
        subscribed_events: webhookEvents,
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
          <NotificationLogTab
            count={deliveryCount}
            deliveries={deliveries}
            error={deliveriesQuery.error}
            incident={incident}
            isLoading={deliveriesQuery.isLoading}
            offset={offset}
            onIncidentChange={setIncident}
            onOffsetChange={setOffset}
            onStatusChange={setStatus}
            status={status}
          />
        </TabsContent>

        <TabsContent value="channels">
          <WebhookChannelsTab
            channels={webhookChannels}
            error={channelsResponse.error}
            isLoading={channelsResponse.isLoading}
            onCreate={openCreateWebhookDialog}
            onDelete={setDeletingChannel}
            onEdit={openEditWebhookDialog}
          />
        </TabsContent>

        <TabsContent value="rules">
          <AlertRulesTab
            error={rulesResponse.error}
            isLoading={rulesResponse.isLoading}
            rules={displayedRules}
            webhookChannelNames={webhookChannelNames}
          />
        </TabsContent>
      </Tabs>

      <WebhookDialog
        enabled={webhookEnabled}
        events={webhookEvents}
        isEditing={isEditingWebhook}
        isOpen={isWebhookDialogOpen}
        isPending={isWebhookPending}
        mutationError={webhookMutationError}
        name={webhookName}
        onClose={closeWebhookDialog}
        onEnabledChange={setWebhookEnabled}
        onEventToggle={toggleWebhookEvent}
        onNameChange={setWebhookName}
        onOpenChange={setIsWebhookDialogOpen}
        onSubmit={handleWebhookSubmit}
        onUrlChange={setWebhookUrl}
        url={webhookUrl}
      />
      <DeleteWebhookDialog
        channel={deletingChannel}
        isPending={deleteWebhook.isPending}
        mutationError={webhookMutationError}
        onClose={() => setDeletingChannel(null)}
        onDelete={() => {
          if (!deletingChannel?.id) return;
          deleteWebhook.mutate({ id: deletingChannel.id });
        }}
      />
    </div>
  );
};
