import { DataTable } from "@/components/shared/data-table";
import { type DataTableFilterField, filterParam } from "@/components/shared/data-table-schema";
import { DataTableToolbar } from "@/components/shared/data-table-toolbar";
import { useDataTableUrlState } from "@/components/shared/use-data-table-url-state";
import { DataTableLink } from "@/components/shared/data-table-link";
import { EmptyState } from "@/components/shared/empty-state";
import { ListPagination } from "@/components/shared/list-pagination";
import { PageHeader } from "@/components/shared/page-header";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import {
  type ApiOrionEventResponse,
  type ApiServiceLogEntryResponse,
  type GetOrionEventsParams,
  type GetServiceLogsParams,
  useGetOrionEvents,
  useGetServiceLogs,
} from "@/orion-sdk";
import { type ColumnDef } from "@tanstack/react-table";
import { parseAsString, useQueryState } from "nuqs";

const EVENT_LIMIT = 40;
const SERVICE_LOG_LIMIT = 40;

const sourceOptions = [
  { value: "all", label: "All sources" },
  { value: "agent", label: "Servers" },
  { value: "monitor", label: "Monitors" },
  { value: "agent_report", label: "Server reports" },
  { value: "monitor_report", label: "Monitor reports" },
  { value: "incident_event", label: "Incidents" },
  { value: "alert_delivery", label: "Alerts" },
  { value: "data_lifecycle", label: "Data lifecycle" },
] as const;

const typeOptions = [
  { value: "all", label: "All types" },
  { value: "agent_registered", label: "Server registered" },
  { value: "monitor_registered", label: "Monitor registered" },
  { value: "agent_report_received", label: "Server report received" },
  { value: "monitor_report_received", label: "Monitor report received" },
  { value: "incident_opened", label: "Incident opened" },
  { value: "incident_resolved", label: "Incident resolved" },
  { value: "monitor_failed", label: "Monitor failed" },
  { value: "alert_pending", label: "Alert pending" },
  { value: "alert_sent", label: "Alert sent" },
  { value: "alert_failed", label: "Alert failed" },
  { value: "alert_suppressed", label: "Alert suppressed" },
  { value: "alert_cooldown", label: "Alert cooldown" },
  { value: "data_lifecycle_settings_updated", label: "Lifecycle settings updated" },
  { value: "data_lifecycle_rollup_run", label: "Lifecycle rollup run" },
  { value: "data_lifecycle_archive_run", label: "Lifecycle archive run" },
  { value: "retention_rollup_ran", label: "Retention rollup ran" },
  { value: "retention_archive_ran", label: "Retention archive ran" },
] as const;

const levelOptions = [
  { value: "all", label: "All levels" },
  { value: "DEBUG", label: "Debug" },
  { value: "INFO", label: "Info" },
  { value: "WARN", label: "Warn" },
  { value: "ERROR", label: "Error" },
] as const;

const eventFields: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search events" },
  { type: "select", key: "source", label: "Source", options: sourceOptions },
  { type: "select", key: "type", label: "Type", options: typeOptions },
];

const serviceLogFields: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search service logs" },
  { type: "select", key: "level", label: "Level", options: levelOptions },
  { type: "input", key: "component", placeholder: "Component" },
];

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
            <DataTableLink to={`/servers/${event.agent_id}`} className="font-medium">
              server
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

const serviceLogColumns: ColumnDef<ApiServiceLogEntryResponse>[] = [
  {
    accessorKey: "occurred_at",
    header: "Time",
    cell: ({ row }) => <span>{formatDate(row.original.occurred_at, DATE_TIME_FORMAT)}</span>,
  },
  {
    accessorKey: "level",
    header: "Level",
    cell: ({ row }) => row.original.level ?? "INFO",
  },
  {
    accessorKey: "component",
    header: "Component",
    cell: ({ row }) => row.original.component || "server",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[30rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
  {
    id: "agent",
    header: "Server",
    cell: ({ row }) => {
      const log = row.original;
      if (!log.agent_id) return "—";
      return (
        <DataTableLink to={`/servers/${log.agent_id}?tab=service-logs`} className="font-medium">
          {log.agent_name || "server"}
        </DataTableLink>
      );
    },
  },
  {
    id: "monitor",
    header: "Monitor",
    cell: ({ row }) => {
      const log = row.original;
      if (!log.monitor_id) return log.monitor_name || "—";
      return (
        <DataTableLink to={`/monitors/${log.monitor_id}`} className="font-medium">
          {log.monitor_name || "monitor"}
        </DataTableLink>
      );
    },
  },
];

export const LogsPage = () => {
  const [tab, setTabState] = useQueryState("tab", parseAsString.withDefault("events"));
  const eventTable = useDataTableUrlState({ fields: eventFields, limit: EVENT_LIMIT });
  const serviceTable = useDataTableUrlState({
    fields: serviceLogFields,
    limit: SERVICE_LOG_LIMIT,
    prefix: "service",
  });
  const [eventQ, eventSource, eventType] = eventFields.map((f) =>
    filterParam(f, eventTable.values[f.key]),
  );
  const eventsQuery = useGetOrionEvents({
    limit: EVENT_LIMIT,
    offset: eventTable.offset,
    q: eventQ,
    source: eventSource,
    type: eventType,
  } satisfies GetOrionEventsParams);
  const events = eventsQuery.data?.events ?? [];
  const count = eventsQuery.data?.count ?? events.length;
  const [serviceQ, serviceLevel, serviceComponent] = serviceLogFields.map((f) =>
    filterParam(f, serviceTable.values[f.key]),
  );
  const serviceLogsQuery = useGetServiceLogs({
    limit: SERVICE_LOG_LIMIT,
    offset: serviceTable.offset,
    q: serviceQ,
    level: serviceLevel,
    component: serviceComponent,
  } satisfies GetServiceLogsParams);
  const serviceLogs = serviceLogsQuery.data?.logs ?? [];
  const serviceLogCount = serviceLogsQuery.data?.count ?? serviceLogs.length;
  const linkedEventCount = events.filter(hasRelatedData).length;
  const sourceCount = new Set(events.map((event) => event.source ?? "unknown")).size;

  const setTab = (nextTab: string) => {
    if (nextTab !== "events" && nextTab !== "service") return;
    void setTabState(nextTab);
  };

  return (
    <div className="space-y-7">
      <PageHeader
        title="Logs"
        description="Operational events and structured service logs from Orion runtimes."
      />

      <Tabs value={tab} onValueChange={setTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="events">Operational Events</TabsTrigger>
          <TabsTrigger value="service">Service Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="events">
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

            <DataTableToolbar
              fields={eventFields}
              values={eventTable.values}
              onChange={eventTable.setFilter}
              onClear={eventTable.clearFilters}
            />

            {eventsQuery.error && (
              <EmptyState
                title="Unable to load event log"
                description="Retry after Core is reachable."
                tone="error"
                action={
                  <Button size="sm" variant="outline" onClick={() => void eventsQuery.refetch()}>
                    Retry
                  </Button>
                }
              />
            )}
            {!eventsQuery.error && (
              <DataTable
                columns={columns}
                data={events}
                emptyMessage={
                  eventTable.hasFilters
                    ? "No events match the current filters."
                    : "No Orion events recorded."
                }
                getRowId={(event) => event.id ?? ""}
                isLoading={eventsQuery.isLoading}
                loadingMessage="Loading event log..."
              />
            )}
            {count > 0 && (
              <ListPagination
                count={count}
                limit={EVENT_LIMIT}
                offset={eventTable.offset}
                onOffsetChange={eventTable.setOffset}
              />
            )}
          </section>
        </TabsContent>

        <TabsContent value="service">
          <section className="space-y-4">
            <div className="grid gap-1 text-sm sm:grid-cols-3">
              <div className="bg-neutral-100 p-3">
                <div className="text-neutral-600">matching logs</div>
                <div className="font-medium">{serviceLogCount}</div>
              </div>
              <div className="bg-neutral-100 p-3">
                <div className="text-neutral-600">visible</div>
                <div className="font-medium">{serviceLogs.length}</div>
              </div>
              <div className="bg-neutral-100 p-3">
                <div className="text-neutral-600">servers</div>
                <div className="font-medium">
                  {new Set(serviceLogs.map((log) => log.agent_id ?? "unknown")).size}
                </div>
              </div>
            </div>

            <DataTableToolbar
              fields={serviceLogFields}
              values={serviceTable.values}
              onChange={serviceTable.setFilter}
              onClear={serviceTable.clearFilters}
            />

            {serviceLogsQuery.error && (
              <EmptyState
                title="Unable to load service logs"
                description="Retry after Core is reachable."
                tone="error"
                action={
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void serviceLogsQuery.refetch()}
                  >
                    Retry
                  </Button>
                }
              />
            )}
            {!serviceLogsQuery.error && (
              <DataTable
                columns={serviceLogColumns}
                data={serviceLogs}
                emptyMessage={
                  serviceTable.hasFilters
                    ? "No service logs match the current filters."
                    : "No service logs recorded."
                }
                getRowId={(log) => log.id ?? ""}
                isLoading={serviceLogsQuery.isLoading}
                loadingMessage="Loading service logs..."
              />
            )}
            {serviceLogCount > 0 && (
              <ListPagination
                count={serviceLogCount}
                limit={SERVICE_LOG_LIMIT}
                offset={serviceTable.offset}
                onOffsetChange={serviceTable.setOffset}
              />
            )}
          </section>
        </TabsContent>
      </Tabs>
    </div>
  );
};
