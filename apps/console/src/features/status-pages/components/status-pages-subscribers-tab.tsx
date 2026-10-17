import { DataTable } from "@/components/shared/data-table";
import { type DataTableFilterField, filterParam } from "@/components/shared/data-table-schema";
import { DataTableToolbar } from "@/components/shared/data-table-toolbar";
import { useDataTableUrlState } from "@/components/shared/use-data-table-url-state";
import { StatusBadge } from "@/components/shared/status-badges";
import { Button } from "@/components/ui/button";
import {
  type ApiStatusPageSubscriberAdminResponse,
  useAnonymizeStatusPageSubscriber,
  useDeleteStatusPageSubscriber,
  useDisableStatusPageSubscriber,
  useListStatusPageSubscribers,
} from "@/orion-sdk";
import { ShieldX, Trash2, UserX } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import {
  formatDateTime,
  subscriberBadgeStatus,
  subscriberStateOptions,
} from "./status-pages-shared";

type PendingAction = {
  kind: "anonymize" | "delete";
  subscriber: ApiStatusPageSubscriberAdminResponse;
};

const subscriberFields: readonly DataTableFilterField[] = [
  {
    type: "select",
    key: "state",
    label: "State",
    options: subscriberStateOptions.map((option) => ({
      value: option.value || "all",
      label: option.label,
    })),
  },
];

export const StatusPageSubscribersTab = ({ pageId }: { pageId: string }) => {
  const table = useDataTableUrlState({
    fields: subscriberFields,
    limit: 1000,
    prefix: "subscribers",
  });
  const stateFilter = filterParam(subscriberFields[0], table.values.state);
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
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const isMutating =
    disableSubscriber.isPending || anonymizeSubscriber.isPending || deleteSubscriber.isPending;

  const disable = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    disableSubscriber.mutate({ id: pageId, subscriberId: subscriber.id });
  };

  const anonymize = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    setPendingAction({ kind: "anonymize", subscriber });
  };

  const hardDelete = (subscriber: ApiStatusPageSubscriberAdminResponse) => {
    if (!pageId || !subscriber.id) return;
    setPendingAction({ kind: "delete", subscriber });
  };

  const confirmPending = () => {
    if (!pendingAction?.subscriber.id) return;
    const request = { id: pageId, subscriberId: pendingAction.subscriber.id };
    if (pendingAction.kind === "anonymize") anonymizeSubscriber.mutate(request);
    else deleteSubscriber.mutate(request);
    setPendingAction(null);
  };

  const columns: ColumnDef<ApiStatusPageSubscriberAdminResponse>[] = [
    {
      id: "destination",
      header: "Destination",
      cell: ({ row }) => {
        const subscriber = row.original;
        return (
          <div className="min-w-0 space-y-1">
            <div className="font-medium">{subscriber.masked_destination}</div>
            <div className="text-xs text-neutral-600">
              {subscriber.destination_type} · Source {subscriber.source || "unknown"}
            </div>
          </div>
        );
      },
    },
    {
      accessorKey: "state",
      header: "State",
      cell: ({ row }) => (
        <StatusBadge
          fallback={row.original.state}
          value={subscriberBadgeStatus(row.original.state)}
        />
      ),
    },
    {
      id: "components",
      header: "Components",
      cell: ({ row }) => {
        const names = (row.original.components ?? []).map((component) => component.name);
        return names.length === 0 ? (
          <span className="text-neutral-600">All visible components</span>
        ) : (
          names.join(", ")
        );
      },
    },
    {
      id: "delivery",
      header: "Last delivery",
      cell: ({ row }) => {
        const subscriber = row.original;
        if (!subscriber.last_delivery_status) return "—";
        return (
          <div className="text-xs text-neutral-600">
            {subscriber.last_delivery_status} - {formatDateTime(subscriber.last_delivery_at)}
            {subscriber.bounce_count ? <div>Bounces {subscriber.bounce_count}</div> : null}
          </div>
        );
      },
    },
    {
      accessorKey: "created_at",
      header: "Created",
      cell: ({ row }) => formatDateTime(row.original.created_at),
    },
    {
      id: "lifecycle",
      header: "Lifecycle",
      cell: ({ row }) => (
        <div className="space-y-0.5 text-xs text-neutral-600">
          <div>Confirmed {formatDateTime(row.original.confirmed_at)}</div>
          <div>Unsubscribed {formatDateTime(row.original.unsubscribed_at)}</div>
          <div>Disabled {formatDateTime(row.original.disabled_at)}</div>
        </div>
      ),
    },
    {
      id: "actions",
      header: "Actions",
      cell: ({ row }) => {
        const subscriber = row.original;
        return (
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
        );
      },
    },
  ];

  return (
    <section className="space-y-4">
      <div>
        <h3 className="text-sm font-medium">Subscribers</h3>
        <p className="mt-1 text-sm text-neutral-600">
          {subscribersResponse.data?.count ?? 0} matching records
        </p>
      </div>
      <DataTableToolbar
        fields={subscriberFields}
        values={table.values}
        onChange={table.setFilter}
        onClear={table.clearFilters}
      />

      {subscribersResponse.isError && <div className="text-sm">Unable to load subscribers.</div>}
      {!subscribersResponse.isError && (
        <DataTable
          columns={columns}
          data={subscribers}
          emptyMessage="No subscribers. Confirmed public subscribers will appear here with masked destinations."
          getRowId={(subscriber, index) => subscriber.id ?? `subscriber-${index}`}
          isLoading={subscribersResponse.isLoading}
          loadingMessage="Loading subscribers..."
        />
      )}

      <ConfirmDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingAction(null);
        }}
        title={pendingAction?.kind === "delete" ? "Delete subscriber?" : "Anonymize subscriber?"}
        description={
          pendingAction?.kind === "delete"
            ? `${pendingAction.subscriber.masked_destination} and their delivery history will be permanently deleted.`
            : `Contact data for ${pendingAction?.subscriber.masked_destination ?? "this subscriber"} will be removed.`
        }
        confirmLabel={pendingAction?.kind === "delete" ? "Delete subscriber" : "Anonymize"}
        onConfirm={confirmPending}
      />

      {(disableSubscriber.isError || anonymizeSubscriber.isError || deleteSubscriber.isError) && (
        <p className="text-sm">Unable to update subscriber.</p>
      )}
    </section>
  );
};
