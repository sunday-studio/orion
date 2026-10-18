import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  Callout,
  ConfirmDialog,
  FieldPanel,
  GridRow,
  GridTable,
  Input,
  RaisedCard,
  useToast,
} from "@/design-system";
import {
  getGetStatusPageDomainQueryKey,
  useGetStatusPageDomain,
  useRemoveStatusPageDomain,
  useUpdateStatusPage,
  useVerifyStatusPageDomain,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const COLUMNS = "70px minmax(0,1fr) minmax(0,1.3fr) 70px";

const ago = (iso?: string) => {
  if (!iso) return "not yet";
  const minutes = Math.max(Math.round((Date.now() - new Date(iso).getTime()) / 60000), 0);
  return minutes < 1
    ? "just now"
    : minutes < 60
      ? `${minutes} min ago`
      : formatDate(iso, "MMM d, HH:mm");
};

/** Custom domain: add one, see the DNS record to create, verify it, or remove it. */
export const DomainPanel = ({
  pageId,
  domain,
  onChanged,
}: {
  pageId: string;
  domain?: string;
  onChanged: () => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string>();
  const [removing, setRemoving] = useState(false);
  const info = useGetStatusPageDomain(pageId, { query: { enabled: Boolean(domain) } });
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageDomainQueryKey(pageId) });
    onChanged();
  };
  const add = useUpdateStatusPage({
    mutation: {
      onSuccess: () => {
        setDraft("");
        setError(undefined);
        refresh();
      },
      onError: (failure) => setError((failure as Error).message || "Unable to add the domain."),
    },
  });
  const verify = useVerifyStatusPageDomain({
    mutation: {
      onSuccess: (result) => {
        refresh();
        toast.add({ title: result.status_text ?? "Checked the domain" });
      },
      onError: () => toast.add({ title: "Unable to check the domain" }),
    },
  });
  const remove = useRemoveStatusPageDomain({
    mutation: {
      onSuccess: () => {
        setRemoving(false);
        refresh();
        toast.add({ title: "Domain removed" });
      },
      onError: () => toast.add({ title: "Unable to remove the domain" }),
    },
  });

  if (!domain) {
    return (
      <FieldPanel title="Custom domain" aside="Optional">
        <p className="mb-3 text-[13px] text-quiet">
          Serve this page on your own address, like status.example.com. You add one DNS record, then
          Orion verifies it.
        </p>
        <div className="flex gap-2">
          <Input
            aria-label="Custom domain"
            placeholder="status.example.com"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
          <Button
            disabled={add.isPending || !draft.trim()}
            onClick={() => {
              setError(undefined);
              add.mutate({ id: pageId, data: { custom_domain: draft.trim() } });
            }}
          >
            Add domain
          </Button>
        </div>
        {error && (
          <Callout tone="down" className="mt-3">
            {error}
          </Callout>
        )}
      </FieldPanel>
    );
  }

  const data = info.data;
  const status = data?.status;
  return (
    <FieldPanel title="Custom domain" aside={domain}>
      <p className="mb-3 text-[13px] text-quiet">
        Add this record at your DNS provider, then verify. Changes can take up to an hour.
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
      {status && (
        <Callout
          tone={status === "verified" ? "up" : status === "failed" ? "down" : "warn"}
          className="mt-3"
          lead={
            status === "verified"
              ? "Verified."
              : status === "failed"
                ? "Failed."
                : "Waiting for DNS."
          }
        >
          {data?.status_text ? `${data.status_text}. ` : ""}We last checked{" "}
          {ago(data?.last_checked_at)}.
          {data?.certificate && data.certificate !== "none"
            ? ` Certificate ${data.certificate}.`
            : ""}
        </Callout>
      )}
      {data?.target_known === false && (
        <p className="mt-2 text-xs text-faint">
          Core has no CNAME target configured, so the record value is empty.
        </p>
      )}
      <div className="mt-3 flex gap-2">
        <Button size="sm" disabled={verify.isPending} onClick={() => verify.mutate({ id: pageId })}>
          Verify now
        </Button>
        <Button size="sm" variant="secondary" onClick={() => setRemoving(true)}>
          Remove domain
        </Button>
      </div>
      <ConfirmDialog
        open={removing}
        onOpenChange={setRemoving}
        title="Remove the custom domain"
        description={`${domain} stops serving this page. The page stays available on its Orion address.`}
        confirmLabel="Remove domain"
        onConfirm={() => remove.mutate({ id: pageId })}
      />
    </FieldPanel>
  );
};
