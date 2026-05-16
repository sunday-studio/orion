import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { PageHeader } from "@/components/page-header";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useGetOrionEvents } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { parseAsInteger, useQueryState } from "nuqs";

const EVENT_LIMIT = 40;

const hasRelatedData = (event: { incident_id?: string; agent_id?: string; monitor_id?: string }) =>
  Boolean(event.incident_id || event.agent_id || event.monitor_id);

export const LogsPage = () => {
  const [page, setPage] = useQueryState("page", parseAsInteger.withDefault(1));
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * EVENT_LIMIT;
  const eventsQuery = useGetOrionEvents({ limit: EVENT_LIMIT, offset });
  const events = eventsQuery.data?.events ?? [];
  const count = eventsQuery.data?.count ?? events.length;
  const linkedEventCount = events.filter(hasRelatedData).length;
  const sourceCount = new Set(events.map((event) => event.source ?? "unknown")).size;
  const setOffset = (nextOffset: number) => {
    void setPage(Math.floor(nextOffset / EVENT_LIMIT) + 1);
  };

  return (
    <div className="space-y-7">
      <PageHeader
        title="Logs"
        description="Agent, monitor, report, incident, alert, and lifecycle activity."
      />

      <section className="space-y-3">
        <div className="grid gap-1 text-sm sm:grid-cols-4">
          <div className="bg-neutral-100 p-3">
            <div className="text-neutral-600">total events</div>
            <div className="font-medium">{count}</div>
          </div>
          <div className="bg-neutral-100 p-3">
            <div className="text-neutral-600">visible</div>
            <div className="font-medium">{events.length}</div>
          </div>
          <div className="bg-neutral-100 p-3">
            <div className="text-neutral-600">linked</div>
            <div className="font-medium">{linkedEventCount}</div>
          </div>
          <div className="bg-neutral-100 p-3">
            <div className="text-neutral-600">sources</div>
            <div className="font-medium">{sourceCount}</div>
          </div>
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
                        <DataTableLink
                          to={`/incidents/${event.incident_id}`}
                          className="font-medium"
                        >
                          incident
                        </DataTableLink>
                      )}
                      {event.agent_id && (
                        <DataTableLink to={`/agents/${event.agent_id}`} className="font-medium">
                          agent
                        </DataTableLink>
                      )}
                      {event.monitor_id && (
                        <DataTableLink to={`/monitors/${event.monitor_id}`} className="font-medium">
                          monitor
                        </DataTableLink>
                      )}
                      {!event.incident_id && !event.agent_id && !event.monitor_id && "—"}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        <ListPagination
          count={count}
          limit={EVENT_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
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
