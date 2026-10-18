import { keepPreviousData, useQueryClient } from "@tanstack/react-query";
import { parseAsString, useQueryStates } from "nuqs";
import {
  Button,
  EmptyState,
  GridRow,
  GridTable,
  Input,
  Segmented,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  exportStatusPageSubscribers,
  getListStatusPageSubscribersQueryKey,
  getListStatusPagesQueryKey,
  useDisableStatusPageSubscriber,
  useListStatusPageSubscribers,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const COLUMNS = "minmax(0,1.6fr) minmax(0,1fr) 110px 120px 80px";
const STATES = ["all", "confirmed", "pending", "disabled"] as const;

export const SubscribersTab = ({ pageId }: { pageId: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [{ q, state }, setQuery] = useQueryStates({
    q: parseAsString.withDefault(""),
    state: parseAsString.withDefault("all"),
  });
  const list = useListStatusPageSubscribers(
    pageId,
    { q: q.trim() || undefined, state: state === "all" ? undefined : state },
    { query: { placeholderData: keepPreviousData } },
  );
  const disable = useDisableStatusPageSubscriber({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({
          queryKey: getListStatusPageSubscribersQueryKey(pageId),
        });
        void queryClient.invalidateQueries({ queryKey: getListStatusPagesQueryKey() });
        toast.add({ title: "Subscriber disabled" });
      },
      onError: () => toast.add({ title: "Unable to disable the subscriber" }),
    },
  });
  const exportCsv = async () => {
    const csv = await exportStatusPageSubscribers(pageId, {
      state: state === "all" ? undefined : state,
    });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
    link.download = "subscribers.csv";
    link.click();
    URL.revokeObjectURL(link.href);
  };
  const subscribers = list.data?.subscribers ?? [];
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <Input
          aria-label="Search subscribers"
          placeholder="Search email"
          className="w-[260px]"
          value={q}
          onChange={(event) => void setQuery({ q: event.target.value })}
        />
        <Segmented
          aria-label="Subscriber state"
          role="group"
          options={STATES.map((value) => ({
            value,
            label: `${value.charAt(0).toUpperCase()}${value.slice(1)}`,
          }))}
          value={state}
          onValueChange={(next) => void setQuery({ state: next })}
        />
        <span className="flex-1" />
        <Button
          size="sm"
          variant="secondary"
          onClick={() =>
            exportCsv().catch(() => toast.add({ title: "Unable to export subscribers" }))
          }
        >
          Export CSV
        </Button>
      </div>
      {list.isPending ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : list.isError ? (
        <EmptyState tone="error" title="Unable to load subscribers" />
      ) : subscribers.length === 0 ? (
        <EmptyState title="No subscribers" description="Visitors who subscribe show up here." />
      ) : (
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>Email</span>
              <span>Scope</span>
              <span>Status</span>
              <span>Since</span>
              <span />
            </GridRow>
            {subscribers.map((subscriber) => (
              <GridRow key={subscriber.id} columns={COLUMNS}>
                <span className="truncate font-medium">
                  {subscriber.email || subscriber.masked_destination}
                </span>
                <span className="truncate text-faint">{subscriber.scope}</span>
                <span className="capitalize text-faint">{subscriber.state}</span>
                <span className="text-faint">{formatDate(subscriber.since, "MMM d, yyyy")}</span>
                <span>
                  {subscriber.state !== "disabled" && (
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={disable.isPending}
                      onClick={() =>
                        disable.mutate({ id: pageId, subscriberId: subscriber.id ?? "" })
                      }
                    >
                      Disable
                    </Button>
                  )}
                </span>
              </GridRow>
            ))}
          </GridTable>
        </div>
      )}
    </div>
  );
};
