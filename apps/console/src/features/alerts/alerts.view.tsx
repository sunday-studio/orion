import { PageHeader } from "@/components/page-header";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiAlertChannelResponse,
  type ApiAlertEmailDestinationResponse,
  type ApiAlertSMTPServiceResponse,
  useCreateAlertChannel,
  useCreateAlertEmailDestination,
  useCreateAlertSMTPService,
  useDeleteAlertChannel,
  useDeleteAlertEmailDestination,
  useDeleteAlertSMTPService,
  useGetAlertChannels,
  useGetAlertDeliveries,
  useGetAlertEmailDestinations,
  useGetAlertRules,
  useGetAlertSMTPServices,
  useTestAlertChannel,
  useTestAlertEmailDestination,
  useTestAlertSMTPService,
  useUpdateAlertChannel,
  useUpdateAlertEmailDestination,
  useUpdateAlertSMTPService,
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
  manageableAlertChannelTypes,
} from "./components/alert-constants";
import { DeleteEmailDestinationDialog } from "./components/delete-email-destination-dialog";
import { DeleteSMTPServiceDialog } from "./components/delete-smtp-service-dialog";
import { DeleteWebhookDialog } from "./components/delete-webhook-dialog";
import { EmailDestinationDialog } from "./components/email-destination-dialog";
import { NotificationLogTab } from "./components/notification-log-tab";
import { SMTPServiceDialog } from "./components/smtp-service-dialog";
import { WebhookChannelsTab } from "./components/webhook-channels-tab";
import { WebhookDialog } from "./components/webhook-dialog";

export const AlertsPage = () => {
  const [isWebhookDialogOpen, setIsWebhookDialogOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<ApiAlertChannelResponse | null>(null);
  const [deletingChannel, setDeletingChannel] = useState<ApiAlertChannelResponse | null>(null);
  const [webhookType, setWebhookType] = useState("webhook");
  const [webhookName, setWebhookName] = useState("");
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookEnabled, setWebhookEnabled] = useState(true);
  const [webhookEvents, setWebhookEvents] = useState<string[]>(defaultAlertEvents);
  const [isSMTPDialogOpen, setIsSMTPDialogOpen] = useState(false);
  const [editingSMTPService, setEditingSMTPService] = useState<ApiAlertSMTPServiceResponse | null>(
    null,
  );
  const [deletingSMTPService, setDeletingSMTPService] =
    useState<ApiAlertSMTPServiceResponse | null>(null);
  const [smtpName, setSMTPName] = useState("");
  const [smtpHost, setSMTPHost] = useState("");
  const [smtpPort, setSMTPPort] = useState("587");
  const [smtpFromEmail, setSMTPFromEmail] = useState("");
  const [smtpUsername, setSMTPUsername] = useState("");
  const [smtpPassword, setSMTPPassword] = useState("");
  const [smtpEnabled, setSMTPEnabled] = useState(true);
  const [isEmailDestinationDialogOpen, setIsEmailDestinationDialogOpen] = useState(false);
  const [editingEmailDestination, setEditingEmailDestination] =
    useState<ApiAlertEmailDestinationResponse | null>(null);
  const [deletingEmailDestination, setDeletingEmailDestination] =
    useState<ApiAlertEmailDestinationResponse | null>(null);
  const [emailDestinationName, setEmailDestinationName] = useState("");
  const [emailDestinationEmailTo, setEmailDestinationEmailTo] = useState("");
  const [emailDestinationSMTPServiceId, setEmailDestinationSMTPServiceId] = useState("");
  const [emailDestinationEnabled, setEmailDestinationEnabled] = useState(true);
  const [emailDestinationEvents, setEmailDestinationEvents] =
    useState<string[]>(defaultAlertEvents);
  const [testingChannelId, setTestingChannelId] = useState<string | null>(null);
  const [testingSMTPServiceId, setTestingSMTPServiceId] = useState<string | null>(null);
  const [testingEmailDestinationId, setTestingEmailDestinationId] = useState<string | null>(null);
  const [testFeedback, setTestFeedback] = useState<{
    channelId?: string;
    message: string;
    status: "pending" | "success" | "error";
  } | null>(null);
  const [smtpTestFeedback, setSMTPTestFeedback] = useState<{
    itemId?: string;
    message: string;
    status: "pending" | "success" | "error";
  } | null>(null);
  const [emailDestinationTestFeedback, setEmailDestinationTestFeedback] = useState<{
    itemId?: string;
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
  const rulesResponse = useGetAlertRules();
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
  const smtpServicesResponse = useGetAlertSMTPServices();
  const emailDestinationsResponse = useGetAlertEmailDestinations();
  const smtpServices = useMemo(
    () => smtpServicesResponse.data?.smtp_services ?? [],
    [smtpServicesResponse.data],
  );
  const emailDestinations = useMemo(
    () => emailDestinationsResponse.data?.email_destinations ?? [],
    [emailDestinationsResponse.data],
  );

  const refreshAlertConfiguration = () => {
    void channelsResponse.refetch();
    void smtpServicesResponse.refetch();
    void emailDestinationsResponse.refetch();
    void rulesResponse.refetch();
  };
  const closeWebhookDialog = () => {
    setWebhookType("webhook");
    setWebhookName("");
    setWebhookUrl("");
    setWebhookEnabled(true);
    setWebhookEvents(defaultAlertEvents);
    setEditingChannel(null);
    setIsWebhookDialogOpen(false);
  };
  const closeSMTPDialog = () => {
    setSMTPName("");
    setSMTPHost("");
    setSMTPPort("587");
    setSMTPFromEmail("");
    setSMTPUsername("");
    setSMTPPassword("");
    setSMTPEnabled(true);
    setEditingSMTPService(null);
    setIsSMTPDialogOpen(false);
  };
  const closeEmailDestinationDialog = () => {
    setEmailDestinationName("");
    setEmailDestinationEmailTo("");
    setEmailDestinationSMTPServiceId("");
    setEmailDestinationEnabled(true);
    setEmailDestinationEvents(defaultAlertEvents);
    setEditingEmailDestination(null);
    setIsEmailDestinationDialogOpen(false);
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
  const createSMTPService = useCreateAlertSMTPService({
    mutation: {
      onSuccess: () => {
        closeSMTPDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const updateSMTPService = useUpdateAlertSMTPService({
    mutation: {
      onSuccess: () => {
        closeSMTPDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const deleteSMTPService = useDeleteAlertSMTPService({
    mutation: {
      onSuccess: () => {
        setDeletingSMTPService(null);
        refreshAlertConfiguration();
      },
    },
  });
  const createEmailDestination = useCreateAlertEmailDestination({
    mutation: {
      onSuccess: () => {
        closeEmailDestinationDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const updateEmailDestination = useUpdateAlertEmailDestination({
    mutation: {
      onSuccess: () => {
        closeEmailDestinationDialog();
        refreshAlertConfiguration();
      },
    },
  });
  const deleteEmailDestination = useDeleteAlertEmailDestination({
    mutation: {
      onSuccess: () => {
        setDeletingEmailDestination(null);
        refreshAlertConfiguration();
      },
    },
  });
  const testChannel = useTestAlertChannel({
    mutation: {
      onSuccess: (data, variables) => {
        const testedChannel = channels.find((item) => item.id === variables.id);
        const channelName = testedChannel?.name ?? "alert destination";
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
        const channelName = testedChannel?.name ?? "alert destination";

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
  const testSMTPService = useTestAlertSMTPService({
    mutation: {
      onSuccess: (data, variables) => {
        const testedService = smtpServices.find((item) => item.id === variables.id);
        const serviceName = testedService?.name ?? "SMTP service";
        const result = data.test;
        const status = result?.status ?? "complete";
        const detail = result?.stage ? ` Stage: ${result.stage}.` : "";

        setSMTPTestFeedback({
          itemId: variables.id,
          message: `Tested ${serviceName}. Status: ${status}.${detail}`,
          status: result?.status === "failed" ? "error" : "success",
        });
      },
      onError: (error, variables) => {
        const testedService = smtpServices.find((item) => item.id === variables.id);
        const serviceName = testedService?.name ?? "SMTP service";

        setSMTPTestFeedback({
          itemId: variables.id,
          message: `Unable to test ${serviceName}. ${getMutationErrorMessage(error, "SMTP test failed.")}`,
          status: "error",
        });
      },
      onSettled: () => {
        setTestingSMTPServiceId(null);
      },
    },
  });
  const testEmailDestination = useTestAlertEmailDestination({
    mutation: {
      onSuccess: (data, variables) => {
        const testedDestination = emailDestinations.find((item) => item.id === variables.id);
        const destinationName = testedDestination?.name ?? "email destination";
        const deliveryStatus = data.delivery?.status;

        setEmailDestinationTestFeedback({
          itemId: variables.id,
          message: deliveryStatus
            ? `Test sent to ${destinationName}. Delivery status: ${deliveryStatus}.`
            : `Test sent to ${destinationName}.`,
          status: "success",
        });
        void emailDestinationsResponse.refetch();
        void deliveriesQuery.refetch();
      },
      onError: (error, variables) => {
        const testedDestination = emailDestinations.find((item) => item.id === variables.id);
        const destinationName = testedDestination?.name ?? "email destination";

        setEmailDestinationTestFeedback({
          itemId: variables.id,
          message: `Unable to test ${destinationName}. ${getMutationErrorMessage(error, "Test delivery failed.")}`,
          status: "error",
        });
      },
      onSettled: () => {
        setTestingEmailDestinationId(null);
      },
    },
  });

  const displayedRules = rulesResponse.data?.rules ?? [];
  const deliveries = deliveriesQuery.data?.deliveries ?? [];
  const deliveryCount = deliveriesQuery.data?.count ?? deliveries.length;
  const isEditingWebhook = Boolean(editingChannel);
  const isWebhookPending = createWebhook.isPending || updateWebhook.isPending;
  const isEditingSMTPService = Boolean(editingSMTPService);
  const isSMTPServicePending = createSMTPService.isPending || updateSMTPService.isPending;
  const isEditingEmailDestination = Boolean(editingEmailDestination);
  const isEmailDestinationPending =
    createEmailDestination.isPending || updateEmailDestination.isPending;
  const webhookMutationError = getMutationErrorMessage(
    createWebhook.error ?? updateWebhook.error ?? deleteWebhook.error,
  );
  const smtpServiceMutationError = getMutationErrorMessage(
    createSMTPService.error ?? updateSMTPService.error ?? deleteSMTPService.error,
    "Unable to save SMTP service.",
  );
  const emailDestinationMutationError = getMutationErrorMessage(
    createEmailDestination.error ?? updateEmailDestination.error ?? deleteEmailDestination.error,
    "Unable to save email destination.",
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
    setWebhookType("webhook");
    setWebhookName("");
    setWebhookUrl("");
    setWebhookEnabled(true);
    setWebhookEvents(defaultAlertEvents);
    setIsWebhookDialogOpen(true);
  };
  const openEditWebhookDialog = (channel: ApiAlertChannelResponse) => {
    setEditingChannel(channel);
    setWebhookType(
      manageableAlertChannelTypes.some((type) => type === channel.type)
        ? (channel.type ?? "webhook")
        : "webhook",
    );
    setWebhookName(channel.name ?? "");
    setWebhookUrl(channel.webhook_url ?? "");
    setWebhookEnabled(channel.enabled ?? true);
    setWebhookEvents(
      channel.subscribed_events?.length ? channel.subscribed_events : defaultAlertEvents,
    );
    setIsWebhookDialogOpen(true);
  };
  const openCreateSMTPDialog = () => {
    setEditingSMTPService(null);
    setSMTPName("");
    setSMTPHost("");
    setSMTPPort("587");
    setSMTPFromEmail("");
    setSMTPUsername("");
    setSMTPPassword("");
    setSMTPEnabled(true);
    setIsSMTPDialogOpen(true);
  };
  const openEditSMTPDialog = (service: ApiAlertSMTPServiceResponse) => {
    setEditingSMTPService(service);
    setSMTPName(service.name ?? "");
    setSMTPHost(service.host ?? "");
    setSMTPPort(service.port ? String(service.port) : "");
    setSMTPFromEmail(service.from_email ?? "");
    setSMTPUsername("");
    setSMTPPassword("");
    setSMTPEnabled(service.enabled ?? true);
    setIsSMTPDialogOpen(true);
  };
  const openCreateEmailDestinationDialog = () => {
    setEditingEmailDestination(null);
    setEmailDestinationName("");
    setEmailDestinationEmailTo("");
    setEmailDestinationSMTPServiceId(smtpServices.find((service) => service.id)?.id ?? "");
    setEmailDestinationEnabled(true);
    setEmailDestinationEvents(defaultAlertEvents);
    setIsEmailDestinationDialogOpen(true);
  };
  const openEditEmailDestinationDialog = (destination: ApiAlertEmailDestinationResponse) => {
    setEditingEmailDestination(destination);
    setEmailDestinationName(destination.name ?? "");
    setEmailDestinationEmailTo(destination.email_to ?? "");
    setEmailDestinationSMTPServiceId(destination.smtp_service_id ?? "");
    setEmailDestinationEnabled(destination.enabled ?? true);
    setEmailDestinationEvents(
      destination.subscribed_events?.length ? destination.subscribed_events : defaultAlertEvents,
    );
    setIsEmailDestinationDialogOpen(true);
  };
  const toggleWebhookEvent = (event: string, enabled: boolean) => {
    setWebhookEvents((current) => {
      if (enabled) return Array.from(new Set([...current, event]));
      return current.filter((item) => item !== event);
    });
  };
  const toggleEmailDestinationEvent = (event: string, enabled: boolean) => {
    setEmailDestinationEvents((current) => {
      if (enabled) return Array.from(new Set([...current, event]));
      return current.filter((item) => item !== event);
    });
  };
  const testAlertDestination = (channel: ApiAlertChannelResponse) => {
    if (!channel.id || testChannel.isPending) return;
    const channelName = channel.name ?? "alert destination";

    setTestingChannelId(channel.id);
    setTestFeedback({
      channelId: channel.id,
      message: `Sending test to ${channelName}...`,
      status: "pending",
    });
    testChannel.mutate({ id: channel.id });
  };
  const testReusableSMTPService = (service: ApiAlertSMTPServiceResponse) => {
    if (!service.id || testSMTPService.isPending) return;
    const serviceName = service.name ?? "SMTP service";

    setTestingSMTPServiceId(service.id);
    setSMTPTestFeedback({
      itemId: service.id,
      message: `Testing ${serviceName}...`,
      status: "pending",
    });
    testSMTPService.mutate({ id: service.id });
  };
  const testReusableEmailDestination = (destination: ApiAlertEmailDestinationResponse) => {
    if (!destination.id || testEmailDestination.isPending) return;
    const destinationName = destination.name ?? "email destination";

    setTestingEmailDestinationId(destination.id);
    setEmailDestinationTestFeedback({
      itemId: destination.id,
      message: `Sending test to ${destinationName}...`,
      status: "pending",
    });
    testEmailDestination.mutate({ id: destination.id });
  };
  const handleWebhookSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = webhookName.trim();
    const url = webhookUrl.trim();
    const type = manageableAlertChannelTypes.some((item) => item === webhookType)
      ? webhookType
      : "webhook";
    if (!name || isWebhookPending) return;
    if (!url) return;
    if (webhookEvents.length === 0) return;
    if (editingChannel?.id) {
      updateWebhook.mutate({
        id: editingChannel.id,
        data: {
          name,
          type,
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
        type,
        enabled: webhookEnabled,
        webhook_url: url,
        subscribed_events: webhookEvents,
      },
    });
  };
  const handleSMTPSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = smtpName.trim();
    const host = smtpHost.trim();
    const fromEmail = smtpFromEmail.trim();
    const port = Number(smtpPort);
    const username = smtpUsername.trim();
    const password = smtpPassword.trim();
    if (!name || !host || !fromEmail || !Number.isInteger(port) || port < 1 || port > 65535) {
      return;
    }
    if (isSMTPServicePending) return;

    const data = {
      name,
      host,
      port,
      from_email: fromEmail,
      enabled: smtpEnabled,
      ...(username ? { username } : {}),
      ...(password ? { password } : {}),
    };

    if (editingSMTPService?.id) {
      updateSMTPService.mutate({
        id: editingSMTPService.id,
        data,
      });
      return;
    }
    createSMTPService.mutate({ data });
  };
  const handleEmailDestinationSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = emailDestinationName.trim();
    const emailTo = emailDestinationEmailTo.trim();
    if (
      !name ||
      !emailTo ||
      !emailDestinationSMTPServiceId ||
      emailDestinationEvents.length === 0 ||
      isEmailDestinationPending
    ) {
      return;
    }

    const data = {
      name,
      email_to: emailTo,
      enabled: emailDestinationEnabled,
      smtp_service_id: emailDestinationSMTPServiceId,
      subscribed_events: emailDestinationEvents,
    };

    if (editingEmailDestination?.id) {
      updateEmailDestination.mutate({
        id: editingEmailDestination.id,
        data,
      });
      return;
    }
    createEmailDestination.mutate({ data });
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
            channels={channels}
            destinations={emailDestinations}
            destinationError={emailDestinationsResponse.error}
            destinationFeedback={emailDestinationTestFeedback}
            error={channelsResponse.error}
            isDestinationLoading={emailDestinationsResponse.isLoading}
            isDestinationTesting={testEmailDestination.isPending}
            isServiceLoading={smtpServicesResponse.isLoading}
            isServiceTesting={testSMTPService.isPending}
            isTesting={testChannel.isPending}
            isLoading={channelsResponse.isLoading}
            onCreate={openCreateWebhookDialog}
            onCreateDestination={openCreateEmailDestinationDialog}
            onCreateService={openCreateSMTPDialog}
            onDelete={setDeletingChannel}
            onDeleteDestination={setDeletingEmailDestination}
            onDeleteService={setDeletingSMTPService}
            onEdit={openEditWebhookDialog}
            onEditDestination={openEditEmailDestinationDialog}
            onEditService={openEditSMTPDialog}
            onTest={testAlertDestination}
            onTestDestination={testReusableEmailDestination}
            onTestService={testReusableSMTPService}
            serviceError={smtpServicesResponse.error}
            serviceFeedback={smtpTestFeedback}
            services={smtpServices}
            testFeedback={testFeedback}
            testingDestinationId={testingEmailDestinationId}
            testingServiceId={testingSMTPServiceId}
            testingChannelId={testingChannelId}
          />
        </TabsContent>

        <TabsContent value="rules">
          <AlertRulesTab
            error={rulesResponse.error}
            isLoading={rulesResponse.isLoading}
            rules={displayedRules}
          />
        </TabsContent>
      </Tabs>

      <WebhookDialog
        channelType={webhookType}
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
        onTypeChange={setWebhookType}
        onUrlChange={setWebhookUrl}
        url={webhookUrl}
      />
      <SMTPServiceDialog
        enabled={smtpEnabled}
        fromEmail={smtpFromEmail}
        host={smtpHost}
        isEditing={isEditingSMTPService}
        isOpen={isSMTPDialogOpen}
        isPending={isSMTPServicePending}
        mutationError={smtpServiceMutationError}
        name={smtpName}
        onClose={closeSMTPDialog}
        onEnabledChange={setSMTPEnabled}
        onFromEmailChange={setSMTPFromEmail}
        onHostChange={setSMTPHost}
        onNameChange={setSMTPName}
        onOpenChange={setIsSMTPDialogOpen}
        onPasswordChange={setSMTPPassword}
        onPortChange={setSMTPPort}
        onSubmit={handleSMTPSubmit}
        onUsernameChange={setSMTPUsername}
        password={smtpPassword}
        port={smtpPort}
        username={smtpUsername}
      />
      <EmailDestinationDialog
        emailTo={emailDestinationEmailTo}
        enabled={emailDestinationEnabled}
        events={emailDestinationEvents}
        isEditing={isEditingEmailDestination}
        isOpen={isEmailDestinationDialogOpen}
        isPending={isEmailDestinationPending}
        mutationError={emailDestinationMutationError}
        name={emailDestinationName}
        onClose={closeEmailDestinationDialog}
        onEmailToChange={setEmailDestinationEmailTo}
        onEnabledChange={setEmailDestinationEnabled}
        onEventToggle={toggleEmailDestinationEvent}
        onNameChange={setEmailDestinationName}
        onOpenChange={setIsEmailDestinationDialogOpen}
        onSMTPServiceChange={setEmailDestinationSMTPServiceId}
        onSubmit={handleEmailDestinationSubmit}
        services={smtpServices.filter((service) => service.id)}
        smtpServiceId={emailDestinationSMTPServiceId}
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
      <DeleteSMTPServiceDialog
        isPending={deleteSMTPService.isPending}
        mutationError={smtpServiceMutationError}
        onClose={() => setDeletingSMTPService(null)}
        onDelete={() => {
          if (!deletingSMTPService?.id) return;
          deleteSMTPService.mutate({ id: deletingSMTPService.id });
        }}
        service={deletingSMTPService}
      />
      <DeleteEmailDestinationDialog
        destination={deletingEmailDestination}
        isPending={deleteEmailDestination.isPending}
        mutationError={emailDestinationMutationError}
        onClose={() => setDeletingEmailDestination(null)}
        onDelete={() => {
          if (!deletingEmailDestination?.id) return;
          deleteEmailDestination.mutate({ id: deletingEmailDestination.id });
        }}
      />
    </div>
  );
};
