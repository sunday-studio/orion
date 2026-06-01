import { EmptyState } from "@/components/empty-state";
import { StatusBadge } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import {
  type ApiStatusPageSubscriberAdminResponse,
  useAnonymizeStatusPageSubscriber,
  useDeleteStatusPageSubscriber,
  useDisableStatusPageSubscriber,
  useListStatusPageSubscribers,
} from "@/orion-sdk";
import { ShieldX, Trash2, UserX } from "lucide-react";
import {
  Field,
  formatDateTime,
  subscriberBadgeStatus,
  subscriberStateOptions,
} from "./status-pages-shared";

export const StatusPageSubscribersTab = ({ pageId }: { pageId: string }) => {
  const [stateFilter, setStateFilter] = useState("");
  const subscribersResponse = useListStatusPageSubscribers(
    pageId,
    stateFilter ? { state: stateFilter } : undefined,
    { query: { enabled: Boolean(pageId) } },
  );
  const subscribers = subscribersResponse.data?.subscribers ?? [];
  const refreshSubscribers = () => {
    void subscribersResponse.refetch();
  };
  const disableSubscriber = useDisableStatusPageSubscriber({
    mutation: { onSuccess: refreshSubscribers },
  });
  const anonymizeSubscriber = useAnonymizeStatusPageSubscriber({
    mutation: { onSuccess: refreshSubscribers },
  });
  const deleteSubscriber = useDeleteStatusPageSubscriber({
    mutation: { onSuccess: refreshSubscribers },
  });
  const isMutating =
    disableSubscriber.isPending || anonymizeSubscriber.isPending || deleteSubscriber.isPending;

  const disable = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    disableSubscriber.mutate({ id: pageId, subscriberId: subscriber.id });
  };

  const anonymize = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    if (!window.confirm("Anonymize this subscriber and remove contact data?")) return;
    anonymizeSubscriber.mutate({ id: pageId, subscriberId: subscriber.id });
  };

  const hardDelete = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    if (!window.confirm("Hard-delete this subscriber and delivery history?")) return;
    deleteSubscriber.mutate({ id: pageId, subscriberId: subscriber.id });
  };

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h3 className="text-sm font-medium">Subscribers</h3>
          <p className="mt-1 text-sm text-neutral-600">
            {subscribersResponse.data?.count ?? 0} matching records
          </p>
        </div>
        <Field label="State">
          <select
            className="h-9 w-full min-w-48 border border-neutral-200 bg-white px-3 text-sm"
            value={stateFilter}
            onChange={(event) => setStateFilter(event.target.value)}
          >
            {subscriberStateOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>
      </div>

      {subscribersResponse.isLoading && <div className="text-sm text-neutral-600">Loading...</div>}
      {subscribersResponse.isError && <div className="text-sm">Unable to load subscribers.</div>}
      {!subscribersResponse.isLoading &&
        !subscribersResponse.isError &&
        subscribers.length === 0 && (
          <EmptyState
            title="No subscribers"
            description="Confirmed public subscribers will appear here with masked destinations."
          />
        )}

      <div className="space-y-3">
        {subscribers.map((subscriber) => (
          <div className="border border-neutral-200 p-3" key={subscriber.id}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{subscriber.masked_destination}</span>
                  <StatusBadge
                    fallback={subscriber.state}
                    value={subscriberBadgeStatus(subscriber.state)}
                  />
                </div>
                <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-sm text-neutral-600">
                  <span>{subscriber.destination_type}</span>
                  <span>Source {subscriber.source || "unknown"}</span>
                  <span>Created {formatDateTime(subscriber.created_at)}</span>
                  {subscriber.last_delivery_status && (
                    <span>
                      Last delivery {subscriber.last_delivery_status} -{" "}
                      {formatDateTime(subscriber.last_delivery_at)}
                    </span>
                  )}
                  {subscriber.bounce_count ? <span>Bounces {subscriber.bounce_count}</span> : null}
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  disabled={isMutating || subscriber.state === "disabled"}
                  onClick={() => disable(subscriber)}
                  type="button"
                  variant="outline"
                >
                  <UserX className="size-4" />
                  Disable
                </Button>
                <Button
                  disabled={isMutating || subscriber.masked_destination === "anonymized"}
                  onClick={() => anonymize(subscriber)}
                  type="button"
                  variant="outline"
                >
                  <ShieldX className="size-4" />
                  Anonymize
                </Button>
                <Button
                  disabled={isMutating}
                  onClick={() => hardDelete(subscriber)}
                  type="button"
                  variant="outline"
                >
                  <Trash2 className="size-4" />
                  Delete
                </Button>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap gap-2 text-sm">
              {(subscriber.components ?? []).map((component) => (
                <span className="border border-neutral-200 px-2 py-1" key={component.id}>
                  {component.name}
                </span>
              ))}
              {(subscriber.components ?? []).length === 0 && (
                <span className="text-neutral-600">All visible components</span>
              )}
            </div>
            <div className="mt-3 grid gap-2 text-xs text-neutral-600 sm:grid-cols-3">
              <span>Confirmed {formatDateTime(subscriber.confirmed_at)}</span>
              <span>Unsubscribed {formatDateTime(subscriber.unsubscribed_at)}</span>
              <span>Disabled {formatDateTime(subscriber.disabled_at)}</span>
            </div>
          </div>
        ))}
      </div>

      {(disableSubscriber.isError || anonymizeSubscriber.isError || deleteSubscriber.isError) && (
        <p className="text-sm">Unable to update subscriber.</p>
      )}
    </section>
  );
};
