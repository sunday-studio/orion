import { useQueryClient } from "@tanstack/react-query";
import { Button, Dot, FieldPanel, RaisedCard, StatusBadge, useToast } from "@/design-system";
import {
  getGetStatusPageDomainQueryKey,
  useGetStatusPageDomain,
  useRemoveStatusPageDomain,
  useVerifyStatusPageDomain,
} from "@/orion-sdk";

const tone = (status?: string) =>
  status === "verified" ? "up" : status === "failed" ? "down" : "degraded";

/** DNS records and verification for a custom domain. Only rendered when one is set. */
export const DomainPanel = ({ pageId, onChanged }: { pageId: string; onChanged: () => void }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const domain = useGetStatusPageDomain(pageId);
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: getGetStatusPageDomainQueryKey(pageId) });
    onChanged();
  };
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
      onSuccess: refresh,
      onError: () => toast.add({ title: "Unable to remove the domain" }),
    },
  });
  const data = domain.data;
  if (!data?.domain) return null;
  return (
    <FieldPanel
      title="Custom domain"
      aside={
        data.certificate && data.certificate !== "none"
          ? `Certificate ${data.certificate}`
          : undefined
      }
    >
      <RaisedCard>
        <div className="flex items-center justify-between gap-3 px-3.5 py-3 text-[13px]">
          <span className="flex min-w-0 items-center gap-2.5">
            <Dot
              color={
                data.status === "verified"
                  ? "#2a9f69"
                  : data.status === "failed"
                    ? "#e5484d"
                    : "#f59e0b"
              }
            />
            <b className="truncate font-medium">{data.domain}</b>
            <StatusBadge status={tone(data.status)}>{data.status_text ?? data.status}</StatusBadge>
          </span>
          <span className="flex gap-1.5">
            <Button
              size="sm"
              variant="secondary"
              disabled={verify.isPending}
              onClick={() => verify.mutate({ id: pageId })}
            >
              Verify now
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={remove.isPending}
              onClick={() => remove.mutate({ id: pageId })}
            >
              Remove
            </Button>
          </span>
        </div>
        {(data.records ?? []).map((record) => (
          <div
            key={`${record.type}-${record.host}`}
            className="grid grid-cols-[60px_minmax(0,1fr)_minmax(0,1.4fr)] gap-3 border-t border-line-soft px-3.5 py-2.5 font-mono text-xs"
          >
            <span className="text-faint">{record.type}</span>
            <span className="truncate">{record.host}</span>
            <span className="truncate">{record.value || "Set by the Core operator"}</span>
          </div>
        ))}
      </RaisedCard>
      {data.target_known === false && (
        <p className="mt-2 text-xs text-faint">
          Core has no CNAME target configured, so the record value is empty.
        </p>
      )}
      {data.fallback_url && data.status !== "verified" && (
        <p className="mt-2 text-xs text-faint">
          Until verified, the page stays at {data.fallback_url}.
        </p>
      )}
    </FieldPanel>
  );
};
