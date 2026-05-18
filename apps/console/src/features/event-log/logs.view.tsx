import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiOrionEventResponse,
  type GetOrionEventsParams,
  useGetOrionEvents,
} from "@/orion-sdk";
import { type ColumnDef } from "@tanstack/react-table";
import { Search } from "lucide-react";
import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";

const EVENT_LIMIT = 40;

const sourceOptions = [
  { value: "all", label: "All sources" },
  { value: "agent", label: "Agents" },
  { value: "monitor", label: "Monitors" },
  { value: "agent_report", label: "Agent reports" },
  { value: "monitor_report", label: "Monitor reports" },
  { value: "incident_event", label: "Incidents" },
  { value: "alert_delivery", label: "Alerts" },
  { value: "data_lifecycle", label: "Data lifecycle" },
] as const;

const typeOptions = [
  { value: "all", label: "All types" },
  { value: "agent_registered", label: "Agent registered" },
  { value: "monitor_registered", label: "Monitor registered" },
  { value: "agent_report_received", label: "Agent report received" },
  { value: "monitor_report_received", label: "Monitor report received" },
  { value: "incident_opened", label: "Incident opened" },
  { value: "incident_resolved", label: "Incident resolved" },
  { value: "monitor_failed", label: "Monitor failed" },
  { value: "alert_pending", label: "Alert pending" },
  { value: "alert_sent", label: "Alert sent" },
  { value: "alert_failed", label: "Alert failed" },
  { value: "alert_suppressed", label: "Alert suppressed" },
  { value: "alert_cooldown", label: "Alert cooldown" },
  { value: "retention_rollup_ran", label: "Retention rollup ran" },
  { value: "retention_archive_ran", label: "Retention archive ran" },
] as const;

const hasRelatedData = (event: ApiOrionEventResponse) =>
  Boolean(event.incident_id || event.agent_id || event.monitor_id);

const columns: ColumnDef<ApiOrionEventResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => <span>{formatDate(row.original.created_at, DATE_TIME_FORMAT)}</span>,
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "source",
    header: "Source",
    cell: ({ row }) => row.original.source ?? "unknown",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[28rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
  {
    id: "related",
    header: "Related",
    cell: ({ row }) => {
      const event = row.original;

      return (
        <div className="flex flex-wrap gap-3">
          {event.incident_id && (
            <DataTableLink to={`/incidents/${event.incident_id}`} className="font-medium">
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
      );
    },
  },
];

export const LogsPage = () => {
  const [{ page, q, source, type }, setLogQuery] = useQueryStates({
    page: parseAsInteger.withDefault(1),
    q: parseAsString.withDefault(""),
    source: parseAsString.withDefault("all"),
    type: parseAsString.withDefault("all"),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * EVENT_LIMIT;
  const params: GetOrionEventsParams = {
    limit: EVENT_LIMIT,
    offset,
    q: q.trim() || undefined,
    source: source === "all" ? undefined : source,
    type: type === "all" ? undefined : type,
  };
  const eventsQuery = useGetOrionEvents(params);
  const events = eventsQuery.data?.events ?? [];
  const count = eventsQuery.data?.count ?? events.length;
  const linkedEventCount = events.filter(hasRelatedData).length;
  const sourceCount = new Set(events.map((event) => event.source ?? "unknown")).size;
  const hasFilters = Boolean(q.trim()) || source !== "all" || type !== "all";
  const sourceLabel = sourceOptions.find((option) => option.value === source)?.label ?? source;
  const typeLabel = typeOptions.find((option) => option.value === type)?.label ?? type;

  const setOffset = (nextOffset: number) => {
    void setLogQuery({ page: Math.floor(nextOffset / EVENT_LIMIT) + 1 });
  };

  const setSearch = (nextSearch: string) => {
    void setLogQuery({ q: nextSearch, page: 1 });
  };

  const setSource = (nextSource: string) => {
    void setLogQuery({ source: nextSource, page: 1 });
  };

  const setType = (nextType: string) => {
    void setLogQuery({ type: nextType, page: 1 });
  };

  const clearFilters = () => {
    void setLogQuery({ q: "", source: "all", type: "all", page: 1 });
  };

  return (
    <div className="space-y-7">
      <PageHeader
        title="Logs"
        description="Agent, monitor, report, incident, alert, and lifecycle activity."
      />

      <section className="space-y-4">
        <div className="grid gap-1 text-sm sm:grid-cols-4">
          <div className="bg-neutral-100 p-3">
            <div className="text-neutral-600">matching events</div>
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

        <div className="flex flex-wrap items-center gap-2">
          <div className="relative w-full max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-400" />
            <Input
              value={q}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search events"
              className="pl-9"
            />
          </div>
          <Select value={source} onValueChange={setSource}>
            <SelectTrigger className="min-w-48 text-xs">
              <span data-slot="select-value">Source: {sourceLabel}</span>
            </SelectTrigger>
            <SelectContent>
              {sourceOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={type} onValueChange={setType}>
            <SelectTrigger className="min-w-56 text-xs">
              <span data-slot="select-value">Type: {typeLabel}</span>
            </SelectTrigger>
            <SelectContent>
              {typeOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {hasFilters && (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              Clear
            </Button>
          )}
        </div>

        {eventsQuery.error && <div className="text-sm">Unable to load event log.</div>}
        {!eventsQuery.error && (
          <DataTable
            columns={columns}
            data={events}
            emptyMessage={
              hasFilters ? "No events match the current filters." : "No Orion events recorded."
            }
            getRowId={(event) => event.id ?? ""}
            isLoading={eventsQuery.isLoading}
            loadingMessage="Loading event log..."
          />
        )}
        <ListPagination
          count={count}
          limit={EVENT_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      </section>
    </div>
  );
};
