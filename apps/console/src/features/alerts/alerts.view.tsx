import { InfiniteScrollSentinel } from "@/components/infinite-scroll-sentinel";
import {
  NotificationBadge,
  SeverityBadge,
  toNotificationStatus,
  toSeverity,
} from "@/components/status-badges";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type GetAlertDeliveries200,
  getAlertDeliveries,
  useGetAlertChannels,
  useGetAlertRules,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCallback } from "react";

const DELIVERY_LIMIT = 30;

const getNextOffset = (lastPage: GetAlertDeliveries200) => {
  const offset = lastPage.offset ?? 0;
  const limit = lastPage.limit ?? DELIVERY_LIMIT;
  const count = lastPage.count ?? 0;
  const nextOffset = offset + limit;
  return nextOffset < count ? nextOffset : undefined;
};

const boolLabel = (value?: boolean) => (value ? "yes" : "no");

const configuredParts = (channel: {
  webhook_configured?: boolean;
  email_to_configured?: boolean;
  email_from_configured?: boolean;
  smtp_host_configured?: boolean;
  smtp_port_configured?: boolean;
  smtp_username_configured?: boolean;
}) => {
  const parts = [];
  if (channel.webhook_configured) parts.push("webhook url");
  if (channel.email_to_configured) parts.push("email to");
  if (channel.email_from_configured) parts.push("email from");
  if (channel.smtp_host_configured) parts.push("smtp host");
  if (channel.smtp_port_configured) parts.push("smtp port");
  if (channel.smtp_username_configured) parts.push("smtp username");
  return parts.length > 0 ? parts.join(", ") : "no endpoint configured";
};

export const AlertsPage = () => {
  const channelsResponse = useGetAlertChannels();
  const rulesResponse = useGetAlertRules();
  const deliveriesQuery = useInfiniteQuery({
    queryKey: ["alert-deliveries", DELIVERY_LIMIT],
    queryFn: ({ pageParam, signal }) =>
      getAlertDeliveries({ limit: DELIVERY_LIMIT, offset: pageParam }, { signal }),
    initialPageParam: 0,
    getNextPageParam: getNextOffset,
  });
  const { fetchNextPage } = deliveriesQuery;
  const deliveries = deliveriesQuery.data?.pages.flatMap((page) => page.deliveries ?? []) ?? [];
  const loadMoreDeliveries = useCallback(() => {
    void fetchNextPage();
  }, [fetchNextPage]);

  return (
    <div className="space-y-7">
      <div>
        <h1 className="text-base font-medium">Alerts</h1>
        <p className="text-sm text-neutral-600">
          Review notification channels, effective alert behavior, and delivery attempts.
        </p>
      </div>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Channels</h2>
          <p className="text-sm text-neutral-600">
            Secrets are hidden. Configure channel values through Core environment variables.
          </p>
        </div>
        {channelsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading alert channels...</div>
        )}
        {channelsResponse.error && <div className="text-sm">Unable to load alert channels.</div>}
        {!channelsResponse.isLoading &&
          !channelsResponse.error &&
          (channelsResponse.data?.channels ?? []).length === 0 && (
            <div className="text-sm text-neutral-600">No alert channels configured.</div>
          )}
        {(channelsResponse.data?.channels ?? []).length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead>Configured</TableHead>
                <TableHead>Last delivery</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(channelsResponse.data?.channels ?? []).map((channel) => (
                <TableRow key={channel.name ?? channel.type}>
                  <TableCell className="font-medium">{channel.name ?? "unnamed"}</TableCell>
                  <TableCell>{channel.type ?? "unknown"}</TableCell>
                  <TableCell>{boolLabel(channel.enabled)}</TableCell>
                  <TableCell className="max-w-[22rem] truncate text-neutral-600">
                    {configuredParts(channel)}
                  </TableCell>
                  <TableCell>
                    {channel.last_delivery_status
                      ? `${channel.last_delivery_status} · ${formatDate(channel.last_delivery_at, DATE_TIME_FORMAT)}`
                      : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </section>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Rules</h2>
          <p className="text-sm text-neutral-600">
            These are the effective alert rules Core applies during incident reconciliation.
          </p>
        </div>
        {rulesResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading alert rules...</div>
        )}
        {rulesResponse.error && <div className="text-sm">Unable to load alert rules.</div>}
        {(rulesResponse.data?.rules ?? []).length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Trigger</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Cooldown</TableHead>
                <TableHead>Recovery</TableHead>
                <TableHead>Channels</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(rulesResponse.data?.rules ?? []).map((rule) => (
                <TableRow key={rule.name}>
                  <TableCell className="font-medium">{rule.name ?? "unnamed"}</TableCell>
                  <TableCell className="max-w-[22rem] truncate text-neutral-600">
                    {rule.trigger_condition ?? "—"}
                  </TableCell>
                  <TableCell>
                    <SeverityBadge value={toSeverity(rule.severity)} />
                  </TableCell>
                  <TableCell>{rule.cooldown_seconds ?? 0}s</TableCell>
                  <TableCell>{boolLabel(rule.recovery_notification_enabled)}</TableCell>
                  <TableCell>{(rule.target_channels ?? []).join(", ") || "none"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </section>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Notification Log</h2>
          <p className="text-sm text-neutral-600">
            Delivery attempts generated when incidents open or recover.
          </p>
        </div>
        {deliveriesQuery.isLoading && (
          <div className="text-sm text-neutral-600">Loading notification log...</div>
        )}
        {deliveriesQuery.error && <div className="text-sm">Unable to load notification log.</div>}
        {!deliveriesQuery.isLoading && !deliveriesQuery.error && deliveries.length === 0 && (
          <div className="text-sm text-neutral-600">No notification deliveries recorded.</div>
        )}
        {deliveries.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Channel</TableHead>
                <TableHead>Event</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell className="font-medium">
                    {formatDate(delivery.created_at, DATE_TIME_FORMAT)}
                  </TableCell>
                  <TableCell>{delivery.channel ?? "none"}</TableCell>
                  <TableCell>{delivery.event_type ?? "unknown"}</TableCell>
                  <TableCell>
                    <NotificationBadge value={toNotificationStatus(delivery.status)} />
                  </TableCell>
                  <TableCell className="max-w-[24rem] truncate text-neutral-600">
                    {delivery.error ?? "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <InfiniteScrollSentinel
          hasNextPage={Boolean(deliveriesQuery.hasNextPage)}
          isFetchingNextPage={deliveriesQuery.isFetchingNextPage}
          onLoadMore={loadMoreDeliveries}
        />
      </section>
    </div>
  );
};
