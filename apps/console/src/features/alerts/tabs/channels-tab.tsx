import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  Button,
  Dot,
  EmptyState,
  GridRow,
  GridTable,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiAlertChannelResponse,
  type ApiAlertTestResult,
  getAlertDeliveries,
  getGetAlertChannelsQueryKey,
  useGetAlertChannels,
  useRetryAlertDelivery,
  useTestAlertChannel,
} from "@/orion-sdk";
import { channelDot, channelTypeLabel, healthLabel } from "../alerts-format";
import { ChannelTestResultDialog } from "../channels/channel-test-result-dialog";
import { useState } from "react";

const COLUMNS = "12px minmax(0,1.2fr) 90px minmax(0,1fr) 100px 150px";

export const ChannelsTab = ({
  onEdit,
  onAdd,
}: {
  onEdit: (channel: ApiAlertChannelResponse) => void;
  onAdd: () => void;
}) => {
  const navigate = useNavigate();
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetAlertChannels();
  const channels = query.data?.channels ?? [];
  const [tested, setTested] = useState<{
    channel: ApiAlertChannelResponse;
    result: ApiAlertTestResult;
  }>();
  const test = useTestAlertChannel({
    mutation: {
      onSuccess: (data, variables) => {
        const result = (data as { result?: ApiAlertTestResult }).result;
        const channel = channels.find((item) => item.id === variables.id);
        if (result && channel) setTested({ channel, result });
        void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
      },
      onError: () => toast.add({ title: "Unable to send the test" }),
    },
  });
  const retry = useRetryAlertDelivery({
    mutation: {
      onSuccess: () => {
        toast.add({ title: "Retry queued" });
        void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
      },
      onError: () => toast.add({ title: "Unable to retry the delivery" }),
    },
  });
  /** Retries the newest failed delivery of a channel. */
  const retryLatest = async (channelId: string) => {
    try {
      const failed = await getAlertDeliveries({
        channel_id: channelId,
        status: "failed",
        limit: 1,
      });
      const id = failed.deliveries?.[0]?.id;
      if (id) retry.mutate({ id });
      else toast.add({ title: "No failed delivery to retry" });
    } catch {
      toast.add({ title: "Unable to find the failed delivery" });
    }
  };

  if (query.isPending) return <Skeleton className="mt-3 h-40 w-full rounded-xl" />;
  if (query.isError) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load channels"
        action={
          <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  }
  if (channels.length === 0) {
    return (
      <div className="mt-3 flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <b className="text-[15px] font-medium">No channels yet</b>
        <p className="text-[13px] text-mid">Add a webhook or an email address to get alerts.</p>
        <Button size="sm" className="mt-3" onClick={onAdd}>
          Add channel
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-3 overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
      <GridTable>
        <GridRow columns={COLUMNS} header>
          <span />
          <span>Channel</span>
          <span>Type</span>
          <span>Last delivery</span>
          <span>Status</span>
          <span />
        </GridRow>
        {channels.map((channel) => {
          const failing = channel.health === "failing";
          return (
            <GridRow
              key={channel.id}
              flipId={channel.id}
              columns={COLUMNS}
              className="cursor-pointer"
              onClick={() => navigate(`/alerts/channels/${channel.id}`)}
            >
              <Dot color={channelDot(channel.health)} />
              <b className="font-medium">{channel.name}</b>
              <span className="text-faint">{channelTypeLabel(channel.type)}</span>
              <span className="text-faint">{channel.last_delivery_text ?? "-"}</span>
              <span>
                <StatusBadge
                  status={
                    channel.health === "healthy"
                      ? "up"
                      : channel.health === "failing"
                        ? "down"
                        : "unknown"
                  }
                  shape={failing ? "square" : channel.health === "untested" ? "hollow" : undefined}
                >
                  {healthLabel(channel.health)}
                </StatusBadge>
              </span>
              <span
                className="flex justify-end gap-1.5 !overflow-visible"
                onClick={(event) => event.stopPropagation()}
              >
                {failing ? (
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={retry.isPending}
                    onClick={() => void retryLatest(channel.id ?? "")}
                  >
                    Retry
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={test.isPending}
                    onClick={() => test.mutate({ id: channel.id ?? "" })}
                  >
                    Test
                  </Button>
                )}
                <Button size="sm" variant="secondary" onClick={() => onEdit(channel)}>
                  Edit
                </Button>
              </span>
            </GridRow>
          );
        })}
      </GridTable>
      <ChannelTestResultDialog
        result={tested?.result}
        email={tested?.channel.type === "email"}
        onRunAgain={() => tested?.channel.id && test.mutate({ id: tested.channel.id })}
        onClose={() => setTested(undefined)}
      />
    </div>
  );
};
