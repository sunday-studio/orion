import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { keepPreviousData } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import {
  AddFilterButton,
  Button,
  CheckFilterPanel,
  Dot,
  EmptyState,
  FilterToken,
  FilterTokenRow,
  GridRow,
  GridTable,
  ListPagination,
  PopoverClose,
  PopoverContent,
  Skeleton,
  StatusBadge,
  usePageSize,
  useToast,
} from "@/design-system";
import { formatWhen } from "@/features/overview/overview-time";
import {
  type GetAlertDeliveriesParams,
  getGetAlertDeliveriesQueryKey,
  useGetAlertDeliveries,
  useRetryAlertDelivery,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  type DeliveryRow,
  deliveryStatusDot,
  deliveryStatusOptions,
  deliveryTitle,
  sinceOptions,
  statusLabel,
} from "../alerts-format";

const COLUMNS = "12px minmax(0,1.5fr) minmax(0,1fr) 110px 90px 70px";
const MENU = "block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray";

export const NotificationLogTab = () => {
  const navigate = useNavigate();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [{ status, since, page }, setQuery] = useQueryStates({
    status: parseAsString.withDefault(""),
    since: parseAsString.withDefault(""),
    page: parseAsInteger.withDefault(1),
  });
  const { limit, setLimit } = usePageSize(20);
  const offset = (Math.max(page, 1) - 1) * limit;
  const statuses = status.split(",").filter(Boolean);
  const [now] = useState(() => Date.now());
  const params: GetAlertDeliveriesParams = {
    limit,
    offset,
    status: status || undefined,
    since: since || undefined,
  };
  const query = useGetAlertDeliveries(params, { query: { placeholderData: keepPreviousData } });
  const deliveries = (query.data?.deliveries ?? []) as DeliveryRow[];
  const total = query.data?.count ?? deliveries.length;
  const retry = useRetryAlertDelivery({
    mutation: {
      onSuccess: () => {
        toast.add({ title: "Retry queued" });
        void queryClient.invalidateQueries({ queryKey: getGetAlertDeliveriesQueryKey() });
      },
      onError: () => toast.add({ title: "Unable to retry the delivery" }),
    },
  });
  const filtersActive = Boolean(status || since);
  const clear = () => void setQuery({ status: "", since: "", page: 1 });

  return (
    <div className="mt-3 flex flex-col gap-3">
      <FilterTokenRow onClearAll={clear}>
        {statuses.length > 0 && (
          <FilterToken
            field="Status"
            operator="is one of"
            value={statuses.map(statusLabel).join(", ")}
            onRemove={() => void setQuery({ status: "", page: 1 })}
          >
            <CheckFilterPanel
              title="Status"
              options={deliveryStatusOptions}
              selected={statuses}
              onChange={(next) => void setQuery({ status: next.join(","), page: 1 })}
              onDone={() => undefined}
            />
          </FilterToken>
        )}
        {since && (
          <FilterToken
            field="Time"
            operator="in"
            value={sinceOptions.find((option) => option.value === since)?.label ?? since}
            onRemove={() => void setQuery({ since: "", page: 1 })}
          >
            <PopoverContent className="w-[200px] p-1.5">
              {sinceOptions.map((option) => (
                <PopoverClose
                  key={option.value}
                  className={MENU}
                  onClick={() => void setQuery({ since: option.value, page: 1 })}
                >
                  {option.label}
                </PopoverClose>
              ))}
            </PopoverContent>
          </FilterToken>
        )}
        <AddFilterButton>
          <PopoverContent className="w-[180px] p-1.5">
            {!status && (
              <PopoverClose
                className={MENU}
                onClick={() =>
                  void setQuery({
                    status: deliveryStatusOptions.map((o) => o.value).join(","),
                    page: 1,
                  })
                }
              >
                Status
              </PopoverClose>
            )}
            {!since && (
              <PopoverClose
                className={MENU}
                onClick={() => void setQuery({ since: "24h", page: 1 })}
              >
                Time
              </PopoverClose>
            )}
          </PopoverContent>
        </AddFilterButton>
      </FilterTokenRow>
      {query.isPending ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : query.isError ? (
        <EmptyState
          tone="error"
          title="Unable to load the notification log"
          action={
            <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
              Retry
            </Button>
          }
        />
      ) : deliveries.length === 0 && total > 0 && offset > 0 ? (
        <EmptyState
          title="That page is empty"
          description="The list is shorter than this page."
          action={
            <Button size="sm" variant="secondary" onClick={() => void setQuery({ page: 1 })}>
              Back to the first page
            </Button>
          }
        />
      ) : deliveries.length === 0 ? (
        <EmptyState
          title={filtersActive ? "No notifications match" : "No notifications yet"}
          description={
            filtersActive
              ? "Clear the filters to see every notification."
              : "Alerts appear here when a rule sends one."
          }
        />
      ) : (
        <div className="ds-settled overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span />
              <span>Alert</span>
              <span>Channel</span>
              <span>Status</span>
              <span>Time</span>
              <span />
            </GridRow>
            {deliveries.map((delivery) => (
              <GridRow
                key={delivery.id}
                flipId={delivery.id}
                columns={COLUMNS}
                className="cursor-pointer"
                onClick={() => navigate(`/alerts/deliveries/${delivery.id}`)}
              >
                <Dot color={deliveryStatusDot(delivery.status)} />
                <span className="truncate">
                  <b className="font-medium">{deliveryTitle(delivery)}</b>{" "}
                  {delivery.incident_display_id && (
                    <span className="text-faint">{delivery.incident_display_id}</span>
                  )}
                </span>
                <span className="text-faint">{delivery.channel ?? delivery.type ?? "-"}</span>
                <span>
                  <StatusBadge
                    status={delivery.status}
                    shape={delivery.status === "failed" ? "square" : undefined}
                  >
                    {statusLabel(delivery.status)}
                  </StatusBadge>
                </span>
                <span className="text-faint">{formatWhen(delivery.created_at, now)}</span>
                <span className="!overflow-visible">
                  {delivery.status === "failed" && delivery.id && (
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={retry.isPending}
                      onClick={(event) => {
                        event.stopPropagation();
                        retry.mutate({ id: delivery.id ?? "" });
                      }}
                    >
                      Retry
                    </Button>
                  )}
                </span>
              </GridRow>
            ))}
          </GridTable>
          <ListPagination
            count={total}
            limit={limit}
            offset={offset}
            onOffsetChange={(next) => void setQuery({ page: Math.floor(next / limit) + 1 })}
            onLimitChange={(next) => {
              setLimit(next);
              void setQuery({ page: 1 });
            }}
          />
        </div>
      )}
    </div>
  );
};
