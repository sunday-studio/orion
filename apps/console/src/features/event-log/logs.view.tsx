import { InfiniteScrollSentinel } from "@/components/infinite-scroll-sentinel";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { getOrionEvents, type GetOrionEvents200 } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { Link } from "react-router-dom";

const EVENT_LIMIT = 40;

const getNextOffset = (lastPage: GetOrionEvents200) => {
  const offset = lastPage.offset ?? 0;
  const limit = lastPage.limit ?? EVENT_LIMIT;
  const count = lastPage.count ?? 0;
  const nextOffset = offset + limit;
  return nextOffset < count ? nextOffset : undefined;
};

export const LogsPage = () => {
  const eventsQuery = useInfiniteQuery({
    queryKey: ["orion-events", EVENT_LIMIT],
    queryFn: ({ pageParam, signal }) =>
      getOrionEvents({ limit: EVENT_LIMIT, offset: pageParam }, { signal }),
    initialPageParam: 0,
    getNextPageParam: getNextOffset,
  });
  const { fetchNextPage } = eventsQuery;
  const events = eventsQuery.data?.pages.flatMap((page) => page.events ?? []) ?? [];
  const loadMoreEvents = useCallback(() => {
    void fetchNextPage();
  }, [fetchNextPage]);

  return (
    <div className="space-y-7">
      <div>
        <h1 className="text-base font-medium">Logs</h1>
        <p className="text-sm text-neutral-600">
          Orion operational events derived from Core records.
        </p>
      </div>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Orion Event Log</h2>
          <p className="text-sm text-neutral-600">
            Agent, monitor, report, incident, alert, and lifecycle activity.
          </p>
        </div>
        {eventsQuery.isLoading && (
          <div className="text-sm text-neutral-600">Loading event log...</div>
        )}
        {eventsQuery.error && <div className="text-sm">Unable to load event log.</div>}
        {!eventsQuery.isLoading && !eventsQuery.error && events.length === 0 && (
          <div className="text-sm text-neutral-600">No Orion events recorded.</div>
        )}
        {events.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Source</TableHead>
                <TableHead>Message</TableHead>
                <TableHead>Related</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {events.map((event) => (
                <TableRow key={event.id}>
                  <TableCell className="font-medium">
                    {formatDate(event.created_at, DATE_TIME_FORMAT)}
                  </TableCell>
                  <TableCell>{event.type ?? "unknown"}</TableCell>
                  <TableCell>{event.source ?? "unknown"}</TableCell>
                  <TableCell className="max-w-[28rem] truncate text-neutral-600">
                    {event.message ?? "—"}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-3">
                      {event.incident_id && (
                        <Link
                          to={`/incidents/${event.incident_id}`}
                          className="font-medium hover:text-neutral-600"
                        >
                          incident
                        </Link>
                      )}
                      {event.agent_id && (
                        <Link
                          to={`/agents/${event.agent_id}`}
                          className="font-medium hover:text-neutral-600"
                        >
                          agent
                        </Link>
                      )}
                      {event.monitor_id && (
                        <Link
                          to={`/monitors/${event.monitor_id}`}
                          className="font-medium hover:text-neutral-600"
                        >
                          monitor
                        </Link>
                      )}
                      {!event.incident_id && !event.agent_id && !event.monitor_id && "—"}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <InfiniteScrollSentinel
          hasNextPage={Boolean(eventsQuery.hasNextPage)}
          isFetchingNextPage={eventsQuery.isFetchingNextPage}
          onLoadMore={loadMoreEvents}
        />
      </section>

      <section className="space-y-2">
        <h2 className="text-sm font-medium">Service Logs</h2>
        <p className="text-sm text-neutral-600">
          Service log collection is not implemented yet. This page only shows Orion events.
        </p>
      </section>
    </div>
  );
};
