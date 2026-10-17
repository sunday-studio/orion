import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  EmptyState,
  Input,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiAlertChannelResponse,
  type ApiAlertRecipientResponse,
  getGetAlertChannelRecipientsQueryKey,
  getGetAlertChannelsQueryKey,
  useAddAlertChannelRecipient,
  useGetAlertChannelRecipients,
  useRemoveAlertChannelRecipient,
  useResendAlertChannelRecipient,
} from "@/orion-sdk";

type ChannelRecipientsDrawerProps = {
  channel?: ApiAlertChannelResponse;
  onOpenChange: (open: boolean) => void;
};

const statusTone = (status?: string) =>
  status === "confirmed" ? "up" : status === "unsubscribed" ? "disabled" : "pending";

export const ChannelRecipientsDrawer = ({
  channel,
  onOpenChange,
}: ChannelRecipientsDrawerProps) => {
  const channelId = channel?.id ?? "";
  const queryClient = useQueryClient();
  const toast = useToast();
  const recipientsQuery = useGetAlertChannelRecipients(channelId, {
    query: { enabled: Boolean(channelId) },
  });
  const [email, setEmail] = useState("");
  const [removing, setRemoving] = useState<ApiAlertRecipientResponse | null>(null);

  const refresh = () => {
    void queryClient.invalidateQueries({
      queryKey: getGetAlertChannelRecipientsQueryKey(channelId),
    });
    void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
  };
  const addRecipient = useAddAlertChannelRecipient({
    mutation: {
      onSuccess: () => {
        setEmail("");
        refresh();
        toast.add({
          title: "Confirmation email sent",
          description: "They receive alerts once confirmed.",
        });
      },
      onError: () => toast.add({ title: "Unable to add recipient" }),
    },
  });
  const removeRecipient = useRemoveAlertChannelRecipient({ mutation: { onSuccess: refresh } });
  const resendRecipient = useResendAlertChannelRecipient({
    mutation: {
      onSuccess: () => toast.add({ title: "Confirmation email resent" }),
      onError: () => toast.add({ title: "Unable to resend confirmation" }),
    },
  });
  const recipients = recipientsQuery.data?.recipients ?? [];

  return (
    <Drawer swipeDirection="right" open={Boolean(channel)} onOpenChange={onOpenChange}>
      <DrawerContent>
        <DrawerTitle>{channel?.name ?? "Email channel"}</DrawerTitle>
        <DrawerDescription>
          {channel?.confirmed_recipient_count ?? 0} of {channel?.recipient_count ?? 0} recipients
          confirmed
        </DrawerDescription>

        <form
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault();
            if (!email.trim() || !channelId) return;
            addRecipient.mutate({ id: channelId, data: { email: email.trim() } });
          }}
        >
          <Input
            aria-label="Recipient email"
            type="email"
            placeholder="name@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
          <Button type="submit" disabled={!email.trim() || addRecipient.isPending}>
            Add recipient
          </Button>
        </form>

        {recipientsQuery.isLoading ? (
          <Skeleton className="h-8 w-full" />
        ) : recipientsQuery.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load recipients"
            action={
              <Button size="sm" variant="secondary" onClick={() => void recipientsQuery.refetch()}>
                Retry
              </Button>
            }
          />
        ) : recipients.length === 0 ? (
          <p className="text-[13px] text-quiet">
            No recipients yet. Add an email address to send a confirmation.
          </p>
        ) : (
          <ul className="divide-y divide-hairline text-[13px]">
            {recipients.map((recipient) => (
              <li className="flex items-center justify-between gap-3 py-2" key={recipient.id}>
                <div className="min-w-0 space-y-1">
                  <div className="truncate">{recipient.email}</div>
                  <StatusBadge status={statusTone(recipient.status)}>
                    {recipient.status ?? "pending"}
                  </StatusBadge>
                </div>
                <div className="flex gap-2">
                  {recipient.status === "pending" && (
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() =>
                        recipient.id &&
                        resendRecipient.mutate({ id: channelId, recipientId: recipient.id })
                      }
                    >
                      Resend
                    </Button>
                  )}
                  <Button size="sm" variant="secondary" onClick={() => setRemoving(recipient)}>
                    Remove
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}

        <ConfirmDialog
          open={removing !== null}
          onOpenChange={(open) => {
            if (!open) setRemoving(null);
          }}
          title="Remove recipient?"
          description={`${removing?.email ?? "This address"} will stop receiving alerts from ${channel?.name ?? "this channel"}.`}
          confirmLabel="Remove"
          onConfirm={() => {
            if (removing?.id) removeRecipient.mutate({ id: channelId, recipientId: removing.id });
          }}
        />
      </DrawerContent>
    </Drawer>
  );
};
