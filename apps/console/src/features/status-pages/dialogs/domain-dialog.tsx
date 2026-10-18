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
  GridRow,
  GridTable,
  Input,
  RaisedCard,
  StepIndicator,
  useToast,
} from "@/design-system";
import {
  getGetStatusPageDomainQueryKey,
  useGetStatusPageDomain,
  useUpdateStatusPage,
  useVerifyStatusPageDomain,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const STEPS = ["Domain", "DNS record", "Verify"] as const;
const COLUMNS = "64px minmax(0,1fr) minmax(0,1.3fr) 64px";
const AUTO_CHECK_MS = 15_000;

const HOST = /^(?=.{4,253}$)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$/i;

const ago = (iso?: string) => {
  if (!iso) return "not yet";
  const seconds = Math.max(Math.round((Date.now() - new Date(iso).getTime()) / 1000), 0);
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  return minutes < 60 ? `${minutes} min ago` : formatDate(iso, "MMM d, HH:mm");
};

/**
 * Guides adding a custom domain: type it, create the DNS record, then wait while Orion checks
 * until the domain is verified. Closing the dialog keeps the domain pending; reopening resumes.
 */
export const DomainDialog = ({
  open,
  pageId,
  domain,
  onOpenChange,
  onChanged,
}: {
  open: boolean;
  pageId: string;
  domain?: string;
  onOpenChange: (open: boolean) => void;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string>();
  const [editing, setEditing] = useState(false);
  const hasDomain = Boolean(domain) && !editing;
  const info = useGetStatusPageDomain(pageId, {
    query: { enabled: open && Boolean(domain), refetchInterval: open ? 5000 : false },
  });
  const data = info.data;
  const status = data?.status;
  const verified = status === "verified";
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageDomainQueryKey(pageId) });
    onChanged();
  };
  const save = useUpdateStatusPage({
    mutation: {
      onSuccess: () => {
        setEditing(false);
        setError(undefined);
        refresh();
      },
      onError: (failure) => setError((failure as Error).message || "Unable to add the domain."),
    },
  });
  const verify = useVerifyStatusPageDomain({
    mutation: { onSuccess: refresh, onError: () => undefined },
  });

  // While waiting for DNS, check again on a timer so the user can just watch.
  useEffect(() => {
    if (!open || !hasDomain || verified) return;
    const timer = window.setInterval(() => verify.mutate({ id: pageId }), AUTO_CHECK_MS);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, hasDomain, verified, pageId]);

  const step = !hasDomain ? 1 : verified ? 3 : 2;
  const trimmed = draft
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/$/, "");
  const valid = HOST.test(trimmed);
  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setDraft("");
      setError(undefined);
      setEditing(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-w-[560px] rounded-2xl p-5">
        <AnimatedHeight>
          <div className="grid gap-3.5 pb-1.5">
            <DialogHeader>
              <DialogTitle className="text-[16px]">Custom domain</DialogTitle>
            </DialogHeader>
            <StepIndicator steps={STEPS} current={step} />
            {step === 1 && (
              <>
                <p className="text-[13px] leading-[1.5] text-quiet">
                  Serve this page on your own address. Enter the domain you control, for example
                  status.example.com. Use a subdomain: apex domains cannot use a CNAME record.
                </p>
                <label className="grid gap-1.5">
                  <span className="text-[12.5px] font-medium">Domain</span>
                  <Input
                    autoFocus
                    placeholder="status.example.com"
                    value={draft}
                    aria-invalid={draft !== "" && !valid}
                    onChange={(event) => setDraft(event.target.value)}
                  />
                  {draft !== "" && !valid && (
                    <small className="text-xs text-[var(--tone-down-fg)]">
                      Enter a full domain name like status.example.com.
                    </small>
                  )}
                </label>
                {error && <Callout tone="down">{error}</Callout>}
                <DialogFooter>
                  <Button variant="secondary" onClick={() => close(false)}>
                    Cancel
                  </Button>
                  <Button
                    disabled={!valid || save.isPending}
                    onClick={() => {
                      setError(undefined);
                      save.mutate({ id: pageId, data: { custom_domain: trimmed } });
                    }}
                  >
                    Continue
                  </Button>
                </DialogFooter>
              </>
            )}
            {step === 2 && (
              <>
                <p className="text-[13px] leading-[1.5] text-quiet">
                  Add this record at your DNS provider for{" "}
                  <b className="font-medium text-ink">{domain}</b>. Changes can take up to an hour.
                  You can close this and come back, Orion keeps checking while this is open.
                </p>
                <RaisedCard>
                  <GridTable>
                    <GridRow columns={COLUMNS} header>
                      <span>Type</span>
                      <span>Host</span>
                      <span>Points to</span>
                      <span />
                    </GridRow>
                    {(data?.records ?? []).map((record) => (
                      <GridRow key={`${record.type}-${record.host}`} columns={COLUMNS}>
                        <span className="text-faint">{record.type}</span>
                        <span className="truncate text-faint">{record.host}</span>
                        <span className="truncate text-faint">
                          {record.value || "Set by the Core operator"}
                        </span>
                        <span className="text-right">
                          <Button
                            size="sm"
                            variant="secondary"
                            disabled={!record.value}
                            onClick={() => {
                              void navigator.clipboard.writeText(record.value ?? "");
                              toast.add({ title: "Record copied" });
                            }}
                          >
                            Copy
                          </Button>
                        </span>
                      </GridRow>
                    ))}
                  </GridTable>
                </RaisedCard>
                {data?.target_known === false && (
                  <Callout tone="warn">
                    Core has no CNAME target configured, so the record value is empty. Ask whoever
                    runs Core to set it.
                  </Callout>
                )}
                <Callout
                  tone={status === "failed" ? "down" : "warn"}
                  lead={status === "failed" ? "Not matching yet." : "Waiting for DNS."}
                >
                  {data?.status_text ? `${data.status_text}. ` : ""}We last checked{" "}
                  {ago(data?.last_checked_at)} and check again every 15 seconds.
                </Callout>
                <DialogFooter className="sm:justify-between">
                  <Button
                    variant="secondary"
                    onClick={() => {
                      setDraft(domain ?? "");
                      setEditing(true);
                    }}
                  >
                    Change domain
                  </Button>
                  <span className="flex gap-2">
                    <Button variant="secondary" onClick={() => close(false)}>
                      Close
                    </Button>
                    <Button
                      disabled={verify.isPending}
                      onClick={() =>
                        verify.mutate(
                          { id: pageId },
                          { onError: () => toast.add({ title: "Unable to check the domain" }) },
                        )
                      }
                    >
                      {verify.isPending ? "Checking..." : "Verify now"}
                    </Button>
                  </span>
                </DialogFooter>
              </>
            )}
            {step === 3 && (
              <>
                <Callout tone="up" lead="Verified.">
                  {domain} points to Orion
                  {data?.certificate === "issued"
                    ? ", the certificate is issued and HTTPS is on."
                    : data?.certificate === "pending"
                      ? ". The HTTPS certificate is being issued, it can take a few minutes."
                      : "."}
                </Callout>
                <p className="text-[13px] leading-[1.5] text-quiet">
                  Your status page is now served on this domain once it is published.
                </p>
                <DialogFooter>
                  <Button onClick={() => close(false)}>Done</Button>
                </DialogFooter>
              </>
            )}
          </div>
        </AnimatedHeight>
      </DialogContent>
    </Dialog>
  );
};
