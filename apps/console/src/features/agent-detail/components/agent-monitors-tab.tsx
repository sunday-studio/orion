import { DataTable } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { DataTableLink } from "@/components/data-table-link";
import { StatusBadge, toStatus } from "@/components/status-badges";
import {
  type ApiIncidentResponse,
  type ApiMonitorResponse,
  useGetAgentMonitors,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import type { ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, useQueryStates } from "nuqs";
import { monitorHealth, monitorPriority } from "./agent-detail-utils";

const AGENT_MONITOR_LIMIT = 20;

type AgentMonitorsTabProps = {
  agentId: string;
  highlightedIncident?: ApiIncidentResponse;
};

const monitorPath = (monitor: ApiMonitorResponse, highlightedIncident?: ApiIncidentResponse) => {
  const isHighlightedMonitor = highlightedIncident?.monitor_id === monitor.id;

  if (isHighlightedMonitor && highlightedIncident) {
    return `/monitors/${monitor.id}?incident=${encodeURIComponent(highlightedIncident.id ?? "")}`;
  }

  return `/monitors/${monitor.id}`;
};

const monitorColumns = (
  highlightedIncident?: ApiIncidentResponse,
): ColumnDef<ApiMonitorResponse>[] => [
  {
    accessorKey: "name",
    header: "Name",
    cell: ({ row }) => (
      <DataTableLink to={monitorPath(row.original, highlightedIncident)}>
        {row.original.name ?? row.original.id}
      </DataTableLink>
    ),
  },
  {
    id: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge value={toStatus(monitorHealth(row.original))} />,
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "last_successful_report_at",
    header: "Last success",
    cell: ({ row }) => formatDate(row.original.last_successful_report_at, DATE_TIME_FORMAT),
  },
];

export const AgentMonitorsTab = ({ agentId, highlightedIncident }: AgentMonitorsTabProps) => {
  const [{ monitorsPage }, setMonitorsQuery] = useQueryStates({
    monitorsPage: parseAsInteger.withDefault(1),
  });
  const offset = (Math.max(monitorsPage, 1) - 1) * AGENT_MONITOR_LIMIT;
  const monitorsResponse = useGetAgentMonitors(agentId, { limit: AGENT_MONITOR_LIMIT, offset });
  const monitors = [...(monitorsResponse.data?.monitors ?? [])].sort(
    (left, right) => monitorPriority(left) - monitorPriority(right),
  );
  const count = monitorsResponse.data?.count ?? monitors.length;
  const setOffset = (nextOffset: number) => {
    void setMonitorsQuery({ monitorsPage: Math.floor(nextOffset / AGENT_MONITOR_LIMIT) + 1 });
  };

  return (
    <div className="space-y-3">
      {monitorsResponse.error && <div className="text-sm">Unable to load monitors.</div>}
      {!monitorsResponse.error && (
        <DataTable
          columns={monitorColumns(highlightedIncident)}
          data={monitors}
          emptyMessage="No monitors registered"
          getRowId={(monitor) => monitor.id ?? ""}
          isLoading={monitorsResponse.isLoading}
          loadingMessage="Loading monitors..."
          rowClassName={(row) =>
            cn(highlightedIncident?.monitor_id === row.original.id && "bg-amber-50")
          }
        />
      )}
      {count > 0 && (
        <ListPagination
          count={count}
          limit={AGENT_MONITOR_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
