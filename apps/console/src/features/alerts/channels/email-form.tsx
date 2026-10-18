import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { X } from "lucide-react";
import { Input, Segmented, StatusBadge, useToast } from "@/design-system";
import {
  type ApiAlertChannelResponse,
  getGetAlertChannelRecipientsQueryKey,
  useAddAlertChannelRecipient,
  useGetAlertChannelRecipients,
  useRemoveAlertChannelRecipient,
  useResendAlertChannelRecipient,
} from "@/orion-sdk";

export type EmailDraft = {
  name: string;
  recipients: string[];
  mode: "instant" | "digest_15m";
  prefix: string;
};

export const emptyEmailDraft: EmailDraft = {
  name: "",
  recipients: [],
  mode: "instant",
  prefix: "[Orion]",
};

export const emailDraftFrom = (channel: ApiAlertChannelResponse): EmailDraft => ({
  name: channel.name ?? "",
  recipients: [],
  mode: channel.email_delivery_mode === "digest_15m" ? "digest_15m" : "instant",
  prefix: channel.email_subject_prefix ?? "[Orion]",
});

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";
const hint = "text-[13px] font-normal text-quiet";
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export const EmailForm = ({
  draft,
  channel,
  onChange,
}: {
  draft: EmailDraft;
  channel?: ApiAlertChannelResponse;
  onChange: (next: EmailDraft) => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const channelId = channel?.id ?? "";
  const saved = useGetAlertChannelRecipients(channelId, { query: { enabled: Boolean(channelId) } });
  const [typed, setTyped] = useState("");
  const refresh = () =>
    void queryClient.invalidateQueries({
      queryKey: getGetAlertChannelRecipientsQueryKey(channelId),
    });
  const add = useAddAlertChannelRecipient({
    mutation: {
      onSuccess: refresh,
      onError: () => toast.add({ title: "Unable to add the address" }),
    },
  });
  const remove = useRemoveAlertChannelRecipient({ mutation: { onSuccess: refresh } });
  const resend = useResendAlertChannelRecipient({
    mutation: { onSuccess: () => toast.add({ title: "Confirmation email sent" }) },
  });

  const commit = () => {
    const value = typed.trim();
    if (!EMAIL.test(value)) return;
    if (channelId) add.mutate({ id: channelId, data: { email: value } });
    else if (!draft.recipients.includes(value))
      onChange({ ...draft, recipients: [...draft.recipients, value] });
    setTyped("");
  };
  const rows = channelId
    ? (saved.data?.recipients ?? []).map((r) => ({
        id: r.id,
        email: r.email ?? "",
        status: r.status,
      }))
    : draft.recipients.map((email) => ({ id: undefined, email, status: "pending" }));
  const pending = rows.find((row) => row.status === "pending" && row.id);

  return (
    <>
      <label className={label}>
        Name
        <Input
          className="font-normal"
          value={draft.name}
          placeholder="ops-team"
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
        />
      </label>
      <div className={label}>
        <label htmlFor="email-recipients">Recipients</label>
        <div className="flex min-h-[64px] flex-wrap content-start items-center gap-1.5 rounded-md border border-input-border bg-card px-2 py-2">
          {rows.map((row) => (
            <span
              key={row.id ?? row.email}
              className="inline-flex items-center gap-1.5 rounded-md bg-tray px-2 py-0.5 text-xs font-medium"
            >
              {row.email}
              <StatusBadge status={row.status === "confirmed" ? "up" : "degraded"}>
                {row.status === "confirmed" ? "Confirmed" : "Pending"}
              </StatusBadge>
              <button
                type="button"
                aria-label={`Remove ${row.email}`}
                className="text-faint hover:text-ink"
                onClick={() =>
                  row.id
                    ? remove.mutate({ id: channelId, recipientId: row.id })
                    : onChange({
                        ...draft,
                        recipients: draft.recipients.filter((e) => e !== row.email),
                      })
                }
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
          <input
            id="email-recipients"
            className="min-w-[120px] flex-1 bg-transparent text-[13px] font-normal outline-none placeholder:text-faint"
            placeholder="Add address"
            value={typed}
            onChange={(event) => setTyped(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === ",") {
                event.preventDefault();
                commit();
              }
            }}
            onBlur={commit}
          />
        </div>
        <span className={hint}>
          New addresses get a confirmation email. Alerts only go to confirmed addresses.
          {pending && (
            <>
              {" "}
              <button
                type="button"
                className="underline"
                onClick={() =>
                  pending.id && resend.mutate({ id: channelId, recipientId: pending.id })
                }
              >
                Resend to {pending.email}
              </button>
            </>
          )}
        </span>
      </div>
      <div className={label}>
        Delivery
        <Segmented
          aria-label="Delivery"
          surface="card"
          options={[
            { value: "instant", label: "Instant" },
            { value: "digest_15m", label: "Digest every 15 min" },
          ]}
          value={draft.mode}
          onValueChange={(mode) => onChange({ ...draft, mode: mode as EmailDraft["mode"] })}
        />
        <span className={hint}>Critical incidents always send instantly.</span>
      </div>
      <label className={label}>
        Subject prefix
        <Input
          className="font-normal"
          value={draft.prefix}
          onChange={(event) => onChange({ ...draft, prefix: event.target.value })}
        />
      </label>
    </>
  );
};
