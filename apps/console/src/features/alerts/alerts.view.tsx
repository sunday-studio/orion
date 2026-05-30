import { PageHeader } from "@/components/page-header";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAlertChannelResponse,
  useCreateAlertChannel,
  useDeleteAlertChannel,
  useGetAlertChannels,
  useGetAlertDeliveries,
  useTestAlertChannel,
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
  const [testingChannelId, setTestingChannelId] = useState<string | null>(null);
  const [testFeedback, setTestFeedback] = useState<{
    channelId?: string;
    message: string;
    status: "pending" | "success" | "error";
  } | null>(null);
  const [
    { page, status, incident, tab, type: deliveryType, channel, event_type: eventType },
    setDeliveryQuery,
  ] = useQueryStates({
    page: parseAsInteger.withDefault(1),
    status: parseAsStringLiteral(deliveryStatuses).withDefault("all"),
    incident: parseAsString.withDefault(""),
    type: parseAsString.withDefault("all"),
    channel: parseAsString.withDefault("all"),
    event_type: parseAsString.withDefault("all"),
    tab: parseAsStringLiteral(alertTabs).withDefault("logs"),
  });

  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * DELIVERY_LIMIT;
  const channelsResponse = useGetAlertChannels();
  const deliveriesQuery = useGetAlertDeliveries({
    limit: DELIVERY_LIMIT,
    offset,
    status: status === "all" ? undefined : status,
    type: deliveryType === "all" ? undefined : deliveryType,
    channel: channel === "all" ? undefined : channel,
    event_type: eventType === "all" ? undefined : eventType,
    incident_id: incident.trim() || undefined,
  });
  const channels = useMemo(() => channelsResponse.data?.channels ?? [], [channelsResponse.data]);
  const webhookChannels = useMemo(
    () => channels.filter((item) => item.type === "webhook"),
    [channels],
  );

  const refreshAlertConfiguration = () => {
    void channelsResponse.refetch();
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
  const testChannel = useTestAlertChannel({
    mutation: {
      onSuccess: (data, variables) => {
        const testedChannel = channels.find((item) => item.id === variables.id);
        const channelName = testedChannel?.name ?? "webhook";
        const deliveryStatus = data.delivery?.status;

        setTestFeedback({
          channelId: variables.id,
          message: deliveryStatus
            ? `Test sent to ${channelName}. Delivery status: ${deliveryStatus}.`
            : `Test sent to ${channelName}.`,
          status: "success",
        });
        void channelsResponse.refetch();
        void deliveriesQuery.refetch();
      },
      onError: (error, variables) => {
        const testedChannel = channels.find((item) => item.id === variables.id);
        const channelName = testedChannel?.name ?? "webhook";

        setTestFeedback({
          channelId: variables.id,
          message: `Unable to test ${channelName}. ${getMutationErrorMessage(error, "Test delivery failed.")}`,
          status: "error",
        });
      },
      onSettled: () => {
        setTestingChannelId(null);
      },
    },
  });

  const deliveries = deliveriesQuery.data?.deliveries ?? [];
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
  const setDeliveryType = (nextType: string) => {
    void setDeliveryQuery({ type: nextType, page: 1 });
  };
  const setChannel = (nextChannel: string) => {
    void setDeliveryQuery({ channel: nextChannel, page: 1 });
  };
  const setEventType = (nextEventType: string) => {
    void setDeliveryQuery({ event_type: nextEventType, page: 1 });
  };
  const clearDeliveryFilters = () => {
    void setDeliveryQuery({
      status: "all",
      type: "all",
      channel: "all",
      event_type: "all",
      incident: "",
      page: 1,
    });
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
    setWebhookUrl(channel.webhook_url ?? "");
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
  const testAlertDestination = (channel: ApiAlertChannelResponse) => {
    if (!channel.id || testChannel.isPending) return;
    const channelName = channel.name ?? "webhook";

    setTestingChannelId(channel.id);
    setTestFeedback({
      channelId: channel.id,
      message: `Sending test to ${channelName}...`,
      status: "pending",
    });
    testChannel.mutate({ id: channel.id });
  };
  const handleWebhookSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = webhookName.trim();
    const url = webhookUrl.trim();
    if (!name || isWebhookPending) return;
    if (!url) return;
    if (webhookEvents.length === 0) return;
    if (editingChannel?.id) {
      updateWebhook.mutate({
        id: editingChannel.id,
        data: {
          name,
          type: "webhook",
          enabled: webhookEnabled,
          subscribed_events: webhookEvents,
          webhook_url: url,
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
        description="Review webhook channels, effective alert behavior, and delivery attempts."
      />

      <Tabs value={tab} onValueChange={setTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="logs">Notification Log</TabsTrigger>
          <TabsTrigger value="channels">Channels</TabsTrigger>
          <TabsTrigger value="rules">Rules</TabsTrigger>
        </TabsList>

        <TabsContent value="logs">
          <NotificationLogTab
            channel={channel}
            channels={channels}
            count={deliveryCount}
            deliveries={deliveries}
            error={deliveriesQuery.error}
            eventType={eventType}
            incident={incident}
            isLoading={deliveriesQuery.isLoading}
            offset={offset}
            onChannelChange={setChannel}
            onClearFilters={clearDeliveryFilters}
            onEventTypeChange={setEventType}
            onIncidentChange={setIncident}
            onOffsetChange={setOffset}
            onStatusChange={setStatus}
            onTypeChange={setDeliveryType}
            status={status}
            type={deliveryType}
          />
        </TabsContent>

        <TabsContent value="channels">
          <WebhookChannelsTab
            channels={webhookChannels}
            error={channelsResponse.error}
            isLoading={channelsResponse.isLoading}
            isTesting={testChannel.isPending}
            onCreate={openCreateWebhookDialog}
            onDelete={setDeletingChannel}
            onEdit={openEditWebhookDialog}
            onTest={testAlertDestination}
            testFeedback={testFeedback}
            testingChannelId={testingChannelId}
          />
        </TabsContent>

        <TabsContent value="rules">
          <AlertRulesTab channels={webhookChannels} />
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
