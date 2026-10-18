import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  AnimatedHeight,
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  useToast,
} from "@/design-system";
import {
  type ApiAlertChannelResponse,
  type ApiAlertTestResult,
  addAlertChannelRecipient,
  getGetAlertChannelQueryKey,
  getGetAlertChannelsQueryKey,
  useCreateAlertChannel,
  useTestAlertChannel,
  useTestRunAlertChannel,
  useUpdateAlertChannel,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { ChannelTestResultDialog } from "./channel-test-result-dialog";
import { type EmailDraft, EmailForm, emailDraftFrom, emptyEmailDraft } from "./email-form";
import {
  type WebhookDraft,
  WebhookForm,
  emptyWebhookDraft,
  headersPayload,
  webhookDraftFrom,
} from "./webhook-form";

type Step = "pick" | "webhook" | "email";

const TYPES = [
  { value: "webhook", title: "Webhook", caption: "POST JSON to a URL you control" },
  { value: "email", title: "Email", caption: "Send to confirmed addresses" },
] as const;

const DEFAULT_EVENTS = ["incident_opened", "incident_resolved"];

type Props = {
  open: boolean;
  /** Set to edit a saved channel; omit to add one. */
  channel?: ApiAlertChannelResponse | null;
  onOpenChange: (open: boolean) => void;
};

export const ChannelDialog = ({ open, channel, onOpenChange }: Props) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const editing = Boolean(channel?.id);
  const [step, setStep] = useState<Step>("pick");
  const [pick, setPick] = useState<"webhook" | "email">("webhook");
  const [webhook, setWebhook] = useState<WebhookDraft>(emptyWebhookDraft);
  const [email, setEmail] = useState<EmailDraft>(emptyEmailDraft);
  const [error, setError] = useState<string>();
  const [result, setResult] = useState<ApiAlertTestResult>();

  useEffect(() => {
    if (!open) return;
    setError(undefined);
    setResult(undefined);
    if (channel) {
      setStep(channel.type === "email" ? "email" : "webhook");
      setWebhook(webhookDraftFrom(channel));
      setEmail(emailDraftFrom(channel));
    } else {
      setStep("pick");
      setPick("webhook");
      setWebhook(emptyWebhookDraft);
      setEmail(emptyEmailDraft);
    }
  }, [open, channel]);

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
    if (channel?.id) {
      void queryClient.invalidateQueries({ queryKey: getGetAlertChannelQueryKey(channel.id) });
    }
  };
  const failure = (fallback: string) => (cause: unknown) =>
    setError((cause as Error).message || fallback);
  const create = useCreateAlertChannel();
  const update = useUpdateAlertChannel();
  const testRun = useTestRunAlertChannel({
    mutation: {
      onSuccess: (data) => setResult(data),
      onError: failure("Unable to run the test."),
    },
  });
  const testSaved = useTestAlertChannel({
    mutation: {
      onSuccess: (data) => setResult((data as { result?: ApiAlertTestResult }).result),
      onError: failure("Unable to send the test email."),
    },
  });

  const webhookValid = webhook.name.trim() !== "" && webhook.url.trim().startsWith("https://");
  const emailValid = email.name.trim() !== "";
  const pending = create.isPending || update.isPending;

  const save = async () => {
    setError(undefined);
    try {
      if (step === "webhook") {
        const headers = headersPayload(webhook, editing);
        const data = {
          type: "webhook",
          name: webhook.name.trim(),
          enabled: channel?.enabled ?? true,
          webhook_url: webhook.url.trim(),
          subscribed_events: channel?.subscribed_events ?? DEFAULT_EVENTS,
          ...(headers ? { headers } : {}),
        };
        if (channel?.id) await update.mutateAsync({ id: channel.id, data });
        else await create.mutateAsync({ data });
      } else {
        const data = {
          type: "email",
          name: email.name.trim(),
          enabled: channel?.enabled ?? true,
          subscribed_events: channel?.subscribed_events ?? DEFAULT_EVENTS,
          email_delivery_mode: email.mode,
          email_subject_prefix: email.prefix,
        };
        if (channel?.id) await update.mutateAsync({ id: channel.id, data });
        else {
          const created = await create.mutateAsync({ data });
          const id = created.channel?.id;
          if (id) {
            for (const address of email.recipients) {
              await addAlertChannelRecipient(id, { email: address });
            }
          }
        }
      }
      refresh();
      toast.add({ title: editing ? "Channel saved" : "Channel added" });
      onOpenChange(false);
    } catch (cause) {
      failure("Unable to save the channel.")(cause);
    }
  };

  const runTest = () => {
    setError(undefined);
    if (step === "webhook") {
      testRun.mutate({
        data: {
          type: "webhook",
          webhook_url: webhook.url.trim(),
          headers: headersPayload(webhook, false),
        },
      });
    } else if (channel?.id) {
      testSaved.mutate({ id: channel.id });
    }
  };

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent
          className={cn(
            "gap-3.5 rounded-2xl p-5",
            step === "pick" ? "max-w-[460px]" : "max-w-[520px]",
          )}
        >
          <form
            className="flex min-w-0 flex-col gap-3.5"
            onSubmit={(event) => {
              event.preventDefault();
              if (step === "pick") setStep(pick);
              else if (step === "webhook" ? webhookValid : emailValid) void save();
            }}
          >
            <DialogHeader>
              <DialogTitle className="text-[15px] font-medium">
                {step === "pick"
                  ? "Add channel"
                  : editing
                    ? `Edit ${channel?.name}`
                    : step === "webhook"
                      ? "New webhook"
                      : "New email channel"}
              </DialogTitle>
            </DialogHeader>
            <AnimatedHeight>
              {step === "pick" ? (
                <div
                  role="radiogroup"
                  aria-label="Channel type"
                  className="grid grid-cols-2 gap-2.5"
                >
                  {TYPES.map((type) => (
                    <button
                      key={type.value}
                      type="button"
                      role="radio"
                      aria-checked={pick === type.value}
                      onClick={() => setPick(type.value)}
                      className={cn(
                        "rounded-[10px] border bg-card p-3.5 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-focus/60",
                        pick === type.value ? "border-brand" : "border-hairline",
                      )}
                    >
                      <b className="block text-[14px] font-medium">{type.title}</b>
                      <span className="mt-1 block text-[13px] text-quiet">{type.caption}</span>
                    </button>
                  ))}
                </div>
              ) : step === "webhook" ? (
                <WebhookForm draft={webhook} channel={channel ?? undefined} onChange={setWebhook} />
              ) : (
                <EmailForm draft={email} channel={channel ?? undefined} onChange={setEmail} />
              )}
              {error && <Callout tone="warn">{error}</Callout>}
            </AnimatedHeight>
            <DialogFooter className="items-center sm:justify-between">
              {step === "pick" ? (
                <span />
              ) : (
                <Button
                  type="button"
                  variant="ghost"
                  disabled={
                    testRun.isPending ||
                    testSaved.isPending ||
                    (step === "webhook" ? !webhookValid : !channel?.id)
                  }
                  onClick={runTest}
                >
                  {step === "webhook" ? "Send test" : "Send test email"}
                </Button>
              )}
              <div className="flex gap-2">
                <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
                <Button
                  type="submit"
                  disabled={
                    pending ||
                    (step === "webhook" ? !webhookValid : step === "email" ? !emailValid : false)
                  }
                >
                  {step === "pick" ? "Continue" : "Save channel"}
                </Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <ChannelTestResultDialog
        result={result}
        email={step === "email"}
        onSave={
          step === "webhook" && !editing
            ? () => {
                setResult(undefined);
                void save();
              }
            : undefined
        }
        onRunAgain={runTest}
        onResend={() => setResult(undefined)}
        onClose={() => setResult(undefined)}
      />
    </>
  );
};
