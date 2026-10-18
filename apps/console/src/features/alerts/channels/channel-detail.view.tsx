import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
  Callout,
  ConfirmDialog,
  Dot,
  EmptyState,
  FieldPanel,
  GridFooter,
  GridRow,
  GridTable,
  RaisedCard,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import { formatWhen, useNow } from "@/features/overview/overview-time";
import {
  type ApiAlertTestResult,
  getGetAlertChannelQueryKey,
  getGetAlertChannelsQueryKey,
  useDeleteAlertChannel,
  useGetAlertChannel,
  useRegenerateAlertChannelSecret,
  useRetryAlertDelivery,
  useTestAlertChannel,
  useUpdateAlertChannel,
} from "@/orion-sdk";
import { channelTypeLabel, deliveryStatusDot, deliveryTitle, healthLabel } from "../alerts-format";
import { ChannelDialog } from "./channel-dialog";
import { ChannelTestResultDialog } from "./channel-test-result-dialog";

const PAGE = 10;

const FactRow = ({
  label,
  action,
  children,
}: {
  label: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) => (
  <div className="grid min-h-[43px] grid-cols-[150px_1fr_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0">
    <span className="text-quiet">{label}</span>
    <span className="min-w-0 truncate">{children}</span>
    {action}
  </div>
);

export const ChannelDetailPage = () => {
  const { channelId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const queryClient = useQueryClient();
  const now = useNow(60_000);
  const [limit, setLimit] = useState(PAGE);
  const query = useGetAlertChannel(channelId, { limit, offset: 0 });
  const detail = query.data;
  const channel = detail?.channel;
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [result, setResult] = useState<ApiAlertTestResult>();
  const [secret, setSecret] = useState<string>();
  const [confirmRegen, setConfirmRegen] = useState(false);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetAlertChannelQueryKey(channelId) });
    void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
  };
  const test = useTestAlertChannel({
    mutation: {
      onSuccess: (data) => {
        setResult(data.result);
        refresh();
      },
      onError: () => toast.add({ title: "Unable to send the test" }),
    },
  });
  const toggle = useUpdateAlertChannel({
    mutation: {
      onSuccess: refresh,
      onError: () => toast.add({ title: "Unable to update the channel" }),
    },
  });
  const regenerate = useRegenerateAlertChannelSecret({
    mutation: {
      onSuccess: (data) => {
        setSecret(data.secret);
        refresh();
        toast.add({ title: "New signing secret", description: "It is shown once on this page." });
      },
      onError: () => toast.add({ title: "Unable to regenerate the secret" }),
    },
  });
  const retry = useRetryAlertDelivery({
    mutation: {
      onSuccess: () => {
        toast.add({ title: "Retry queued" });
        refresh();
      },
      onError: () => toast.add({ title: "Unable to retry the delivery" }),
    },
  });
  const remove = useDeleteAlertChannel({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
        toast.add({ title: `${channel?.name} deleted` });
        navigate("/alerts?tab=channels", { replace: true });
      },
      onError: () => toast.add({ title: "Unable to delete the channel" }),
    },
  });

  if (query.isPending) return <Skeleton className="mx-auto mt-10 h-40 w-full max-w-[712px]" />;
  if (query.isError || !channel) {
    return <EmptyState tone="error" title="Unable to load the channel" />;
  }
  const email = channel.type === "email";
  const failing = channel.health === "failing";
  const deliveries = detail?.recent_deliveries ?? [];
  const total = detail?.recent_total ?? deliveries.length;
  const rules = detail?.rules_using ?? [];
  const COLS = "12px 90px minmax(0,1fr) 90px 64px";

  return (
    <div className="mx-auto w-full max-w-[712px] pb-8">
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
        <div className="min-w-0">
          <div className="text-[13.5px] text-quiet">
            <Link to="/alerts?tab=channels" className="hover:text-ink">
              Alerts
            </Link>{" "}
            / Channels
          </div>
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{channel.name}</h1>
          <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
            <StatusBadge
              status={failing ? "down" : channel.health === "healthy" ? "up" : "unknown"}
              shape={failing ? "square" : undefined}
            >
              {channel.enabled === false ? "Disabled" : healthLabel(channel.health)}
            </StatusBadge>
            <span>
              {channelTypeLabel(channel.type)}
              {failing && channel.consecutive_failures
                ? `, ${channel.consecutive_failures} failed deliveries in a row`
                : ""}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <Button
            size="sm"
            variant="secondary"
            disabled={test.isPending}
            onClick={() => test.mutate({ id: channelId })}
          >
            {email ? "Send test email" : "Send test"}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={toggle.isPending}
            onClick={() =>
              toggle.mutate({ id: channelId, data: { enabled: channel.enabled === false } })
            }
          >
            {channel.enabled === false ? "Enable" : "Disable"}
          </Button>
          <Button size="sm" onClick={() => setEditing(true)}>
            Edit
          </Button>
        </div>
      </div>
      <div className="mt-2 flex flex-col gap-3">
        {detail?.alert && (
          <Callout tone="down" className="rounded-[10px] px-4 py-3">
            {detail.alert}
          </Callout>
        )}
        <FieldPanel title="Configuration" aside="Secrets are never shown in full">
          <RaisedCard>
            <FactRow label="Type">{channelTypeLabel(channel.type)}</FactRow>
            {email ? (
              <>
                <FactRow label="Recipients">
                  {channel.confirmed_recipient_count ?? 0} of {channel.recipient_count ?? 0}{" "}
                  confirmed
                </FactRow>
                <FactRow label="Delivery">
                  {channel.email_delivery_mode === "digest_15m" ? "Digest every 15 min" : "Instant"}
                </FactRow>
              </>
            ) : (
              <>
                <FactRow
                  label="URL"
                  action={
                    <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
                      Edit
                    </Button>
                  }
                >
                  {channel.webhook_url}
                </FactRow>
                <FactRow
                  label="Signing secret"
                  action={
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={regenerate.isPending}
                      onClick={() => setConfirmRegen(true)}
                    >
                      Regenerate
                    </Button>
                  }
                >
                  {secret ?? channel.signing_secret_mask ?? "-"}
                </FactRow>
                <FactRow label="Headers">
                  {channel.header_names?.length
                    ? `${channel.header_names.length} custom ${channel.header_names.length === 1 ? "header" : "headers"}`
                    : "None"}
                </FactRow>
              </>
            )}
          </RaisedCard>
        </FieldPanel>
        <FieldPanel title="Recent deliveries" aside="This channel only">
          <RaisedCard>
            {deliveries.length === 0 ? (
              <p className="px-3.5 py-4 text-[13px] text-quiet">Nothing has been delivered yet.</p>
            ) : (
              <GridTable>
                <GridRow columns={COLS} header>
                  <span />
                  <span>Time</span>
                  <span>Alert</span>
                  <span>Result</span>
                  <span />
                </GridRow>
                {deliveries.map((delivery) => (
                  <GridRow
                    key={delivery.id}
                    flipId={delivery.id}
                    columns={COLS}
                    className="cursor-pointer"
                    onClick={() => navigate(`/alerts/deliveries/${delivery.id}`)}
                  >
                    <Dot color={deliveryStatusDot(delivery.status)} />
                    <span className="text-faint">{formatWhen(delivery.created_at, now)}</span>
                    <span className="truncate">
                      <b className="font-medium">{deliveryTitle(delivery)}</b>{" "}
                      <span className="text-faint">{delivery.incident_display_id}</span>
                    </span>
                    <span>
                      <StatusBadge
                        status={delivery.status}
                        shape={delivery.status === "failed" ? "square" : undefined}
                      >
                        {delivery.result_text ?? delivery.status}
                      </StatusBadge>
                    </span>
                    <span className="!overflow-visible">
                      {delivery.status === "failed" && delivery.id && (
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={retry.isPending}
                          onClick={(event) => {
                            event.stopPropagation();
                            retry.mutate({ id: delivery.id ?? "" });
                          }}
                        >
                          Retry
                        </Button>
                      )}
                    </span>
                  </GridRow>
                ))}
                {total > deliveries.length && (
                  <GridFooter>
                    <button
                      type="button"
                      className="underline"
                      onClick={() => setLimit((current) => current + PAGE)}
                    >
                      Show {Math.min(PAGE, total - deliveries.length)} more
                    </button>
                  </GridFooter>
                )}
              </GridTable>
            )}
          </RaisedCard>
        </FieldPanel>
        <FieldPanel title="Rules using this channel" aside={String(rules.length)}>
          <RaisedCard>
            {rules.length === 0 ? (
              <p className="px-3.5 py-4 text-[13px] text-quiet">
                No rule sends to this channel yet.
              </p>
            ) : (
              rules.map((rule) => (
                <div
                  key={rule.id}
                  className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
                >
                  <b className="truncate font-medium">{rule.name}</b>
                  <span className="truncate text-faint">{rule.summary}</span>
                  <Link to="/alerts?tab=rules">
                    <Button size="sm" variant="secondary">
                      Open
                    </Button>
                  </Link>
                </div>
              ))
            )}
          </RaisedCard>
        </FieldPanel>
        <div className="flex justify-end">
          <Button size="sm" variant="destructive" onClick={() => setDeleting(true)}>
            Delete channel
          </Button>
        </div>
      </div>
      <ChannelDialog open={editing} channel={channel} onOpenChange={setEditing} />
      <ChannelTestResultDialog
        result={result}
        email={email}
        onRunAgain={() => test.mutate({ id: channelId })}
        onClose={() => setResult(undefined)}
      />
      <ConfirmDialog
        open={confirmRegen}
        onOpenChange={setConfirmRegen}
        title="Regenerate the signing secret?"
        description="Receivers verifying the old secret will reject new deliveries until you update them."
        confirmLabel="Regenerate"
        onConfirm={() => regenerate.mutate({ id: channelId })}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${channel.name}?`}
        description={`${rules.length} ${rules.length === 1 ? "rule uses" : "rules use"} this channel. Alerts stop reaching it.`}
        confirmLabel="Delete channel"
        onConfirm={() => remove.mutate({ id: channelId })}
      />
    </div>
  );
};
