import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, Callout, ConfirmDialog, Input, JsonBlock, useToast } from "@/design-system";
import {
  type ApiAlertChannelResponse,
  getGetAlertChannelsQueryKey,
  useGetAlertPayloadPreview,
  useRegenerateAlertChannelSecret,
} from "@/orion-sdk";

export type HeaderRow = { name: string; value: string };

export type WebhookDraft = {
  name: string;
  url: string;
  headers: HeaderRow[];
  headersTouched: boolean;
};

export const emptyWebhookDraft: WebhookDraft = {
  name: "",
  url: "",
  headers: [],
  headersTouched: false,
};

export const webhookDraftFrom = (channel: ApiAlertChannelResponse): WebhookDraft => ({
  name: channel.name ?? "",
  url: channel.webhook_url ?? "",
  headers: (channel.header_names ?? []).map((name) => ({ name, value: "" })),
  headersTouched: false,
});

/** Headers object for the API, or undefined when an edit left them alone. */
export const headersPayload = (draft: WebhookDraft, editing: boolean) => {
  if (editing && !draft.headersTouched) return undefined;
  return Object.fromEntries(
    draft.headers
      .filter((row) => row.name.trim() && row.value !== "")
      .map((row) => [row.name.trim(), row.value]),
  );
};

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";
const hint = "text-[13px] font-normal text-quiet";

export const WebhookForm = ({
  draft,
  channel,
  onChange,
}: {
  draft: WebhookDraft;
  /** Present when editing a saved channel. */
  channel?: ApiAlertChannelResponse;
  onChange: (next: WebhookDraft) => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const preview = useGetAlertPayloadPreview();
  const [newSecret, setNewSecret] = useState<string>();
  const [confirmRegen, setConfirmRegen] = useState(false);
  const regenerate = useRegenerateAlertChannelSecret({
    mutation: {
      onSuccess: (result) => {
        setNewSecret(result.secret);
        void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
        toast.add({ title: "New signing secret", description: "Copy it now; it is shown once." });
      },
      onError: () => toast.add({ title: "Unable to regenerate the secret" }),
    },
  });
  const secretText = newSecret ?? channel?.signing_secret_mask ?? "Generated when you save";
  const setRows = (headers: HeaderRow[]) => onChange({ ...draft, headers, headersTouched: true });
  const payload = preview.data?.payload ?? null;

  return (
    <>
      <label className={label}>
        Name
        <Input
          className="font-normal"
          value={draft.name}
          placeholder="oncall webhook"
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
        />
      </label>
      <label className={label}>
        URL
        <Input
          className="font-normal"
          value={draft.url}
          placeholder="https://pager.example.com/orion"
          onChange={(event) => onChange({ ...draft, url: event.target.value })}
        />
        <span className={hint}>Must be https. Orion sends a POST with a JSON body.</span>
      </label>
      <div className={label}>
        Signing secret
        <div className="flex items-center gap-2">
          <Input readOnly aria-label="Signing secret" className="font-normal" value={secretText} />
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={!newSecret}
            onClick={async () => {
              await navigator.clipboard.writeText(newSecret ?? "");
              toast.add({ title: "Secret copied" });
            }}
          >
            Copy
          </Button>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={!channel?.id || regenerate.isPending}
            onClick={() => setConfirmRegen(true)}
          >
            Regenerate
          </Button>
        </div>
        <span className={hint}>
          Verify requests with the {preview.data?.signature_header ?? "X-Orion-Signature"} header
          (HMAC SHA-256).
        </span>
      </div>
      <div className={label}>
        Custom headers (optional)
        {draft.headers.map((row, index) => (
          <div key={index} className="grid grid-cols-[1fr_1fr_auto] items-center gap-2">
            <Input
              aria-label="Header name"
              className="font-normal"
              value={row.name}
              placeholder="X-Team"
              onChange={(event) =>
                setRows(
                  draft.headers.map((r, i) =>
                    i === index ? { ...r, name: event.target.value } : r,
                  ),
                )
              }
            />
            <Input
              aria-label="Header value"
              className="font-normal"
              value={row.value}
              placeholder={channel && !draft.headersTouched ? "unchanged" : "value"}
              onChange={(event) =>
                setRows(
                  draft.headers.map((r, i) =>
                    i === index ? { ...r, value: event.target.value } : r,
                  ),
                )
              }
            />
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={() => setRows(draft.headers.filter((_, i) => i !== index))}
            >
              Remove
            </Button>
          </div>
        ))}
        <button
          type="button"
          className="w-fit text-[13px] font-normal text-mid hover:text-ink"
          onClick={() => setRows([...draft.headers, { name: "", value: "" }])}
        >
          + Add header
        </button>
        {channel && draft.headers.length > 0 && (
          <span className={hint}>
            Values are never shown. Changing headers replaces all of them, so re-enter every value.
          </span>
        )}
      </div>
      <div className={label}>
        Payload
        {payload ? (
          <JsonBlock value={payload} />
        ) : (
          <Callout tone="warn">Payload preview is unavailable.</Callout>
        )}
      </div>
      <ConfirmDialog
        open={confirmRegen}
        onOpenChange={setConfirmRegen}
        title="Regenerate the signing secret?"
        description="Receivers verifying the old secret will reject new deliveries until you update them."
        confirmLabel="Regenerate"
        onConfirm={() => channel?.id && regenerate.mutate({ id: channel.id })}
      />
    </>
  );
};
