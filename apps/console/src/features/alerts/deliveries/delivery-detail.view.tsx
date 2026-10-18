import { useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import {
  Button,
  CodeBlock,
  EmptyState,
  FieldPanel,
  GridRow,
  GridTable,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  getGetAlertDeliveryQueryKey,
  useGetAlertDelivery,
  useRetryAlertDelivery,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { deliveryTitle, statusLabel } from "../alerts-format";

const COLS = "70px minmax(0,1.5fr) 80px 90px";

export const DeliveryDetailPage = () => {
  const { deliveryId = "" } = useParams();
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetAlertDelivery(deliveryId);
  const detail = query.data;
  const delivery = detail?.delivery;
  const retry = useRetryAlertDelivery({
    mutation: {
      onSuccess: () => {
        toast.add({ title: "Retry queued", description: "The same payload is sent again." });
        void queryClient.invalidateQueries({ queryKey: getGetAlertDeliveryQueryKey(deliveryId) });
      },
      onError: () => toast.add({ title: "Unable to retry the delivery" }),
    },
  });

  if (query.isPending) return <Skeleton className="mx-auto mt-10 h-40 w-full max-w-[712px]" />;
  if (query.isError || !delivery)
    return <EmptyState tone="error" title="Unable to load the delivery" />;

  const attempts = [...(delivery.attempts ?? [])].sort(
    (a, b) => (b.attempt_number ?? 0) - (a.attempt_number ?? 0),
  );
  const failed = delivery.status === "failed";
  const payload = detail?.payload ? JSON.stringify(detail.payload) : "";

  return (
    <div className="mx-auto w-full max-w-[712px] pb-8">
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
        <div className="min-w-0">
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
            Delivery to {detail?.channel?.name ?? delivery.channel ?? "channel"}
          </h1>
          <div className="mt-1.5 flex items-center gap-2 text-[13.5px] text-quiet">
            <StatusBadge status={delivery.status} shape={failed ? "square" : undefined}>
              {statusLabel(delivery.status)}
            </StatusBadge>
            <span>
              {[deliveryTitle(delivery), delivery.incident_display_id].filter(Boolean).join(", ")}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          {detail?.channel?.id && (
            <Link to={`/alerts/channels/${detail.channel.id}`}>
              <Button size="sm" variant="secondary">
                Edit channel
              </Button>
            </Link>
          )}
          {detail?.retryable !== false && (
            <Button
              size="sm"
              disabled={retry.isPending}
              onClick={() => retry.mutate({ id: deliveryId })}
            >
              Retry delivery
            </Button>
          )}
        </div>
      </div>
      <div className="mt-2 flex flex-col gap-3">
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLS} header>
              <span>Attempt</span>
              <span>Result</span>
              <span>Took</span>
              <span>Time</span>
            </GridRow>
            {attempts.map((attempt) => (
              <GridRow key={attempt.id ?? attempt.attempt_number} columns={COLS}>
                <span>{attempt.attempt_number}</span>
                <b className="font-medium">
                  {attempt.result_text ?? attempt.error ?? attempt.status}
                </b>
                <span className="text-faint">
                  {typeof attempt.duration_ms === "number"
                    ? `${(attempt.duration_ms / 1000).toFixed(1)}s`
                    : "-"}
                </span>
                <span className="text-faint">
                  {formatDate(attempt.started_at ?? attempt.created_at, "HH:mm")}
                </span>
              </GridRow>
            ))}
          </GridTable>
          {attempts.length === 0 && (
            <p className="px-3.5 py-4 text-[13px] text-quiet">No attempts recorded yet.</p>
          )}
        </div>
        <FieldPanel title="Payload" aside="JSON">
          {payload ? (
            <CodeBlock className="whitespace-pre-wrap break-all">{payload}</CodeBlock>
          ) : (
            <p className="text-[13px] text-quiet">No payload was stored for this delivery.</p>
          )}
        </FieldPanel>
        <p className="text-[13px] text-quiet">
          {detail?.retry_note ??
            "Retrying sends the same payload again. It does not create a new notification record."}
        </p>
      </div>
    </div>
  );
};
