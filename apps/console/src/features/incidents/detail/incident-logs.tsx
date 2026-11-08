import type { ColumnDef } from "@tanstack/react-table";
import { Link } from "react-router-dom";
import { Checkbox, DataTable, StatusBadge } from "@/design-system";
import { levelLabel, levelTone } from "@/features/logs/log-model";
import type { ApiIncidentLogsResponse, ApiLogEntry } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";

const columns: ColumnDef<ApiLogEntry>[] = [
  {
    id: "time",
    header: "Time",
    cell: ({ row }) => (
      <span className="font-mono text-xs text-quiet">
        {formatDate(row.original.timestamp, `${TIME_FORMAT}:ss`)}
      </span>
    ),
  },
  {
    id: "level",
    header: "Level",
    cell: ({ row }) => (
      <StatusBadge status={levelTone(row.original.severity)}>
        {levelLabel(row.original.severity)}
      </StatusBadge>
    ),
  },
  {
    id: "app",
    header: "App",
    cell: ({ row }) => row.original.app_name ?? (row.original.source === "node" ? "Node" : "-"),
  },
  {
    id: "message",
    header: "Message",
    cell: ({ row }) => (
      <span className="block max-w-[60ch] truncate">
        {row.original.body ?? row.original.event ?? ""}
      </span>
    ),
  },
  {
    id: "trace",
    header: "Trace",
    cell: ({ row }) =>
      row.original.trace_id ? (
        <span className="font-mono text-xs text-quiet">{row.original.trace_id.slice(0, 6)}</span>
      ) : (
        <span className="text-faint">-</span>
      ),
  },
];

/** Which apps the logs came from, said plainly so a wrong link is easy to spot. */
const sourceNote = (response: ApiIncidentLogsResponse | undefined, monitorId?: string) => {
  if (!response) return undefined;
  if (response.source === "service") {
    return (
      <>
        From the apps of service{" "}
        <Link to={`/services/${response.service_id}`} className="underline">
          {response.service_name ?? "linked"}
        </Link>
        .
      </>
    );
  }
  if (response.source === "log_rule") return "From the app behind this monitor's log alert rule.";
  if (response.source === "none") {
    return (
      <>
        {response.service_id
          ? `Service ${response.service_name ?? ""} has no app linked.`
          : "This monitor is not linked to a service."}{" "}
        {monitorId && (
          <Link to={`/monitors/${monitorId}/edit`} className="underline">
            Link it from Configure
          </Link>
        )}
      </>
    );
  }
  return undefined;
};

/** Logs around the incident, resolved through the monitor's service, newest first. */
export const IncidentLogs = ({
  response,
  loading,
  failed,
  onRetry,
  preOnly,
  onPreOnlyChange,
  monitorId,
}: {
  response?: ApiIncidentLogsResponse;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  preOnly: boolean;
  onPreOnlyChange: (next: boolean) => void;
  monitorId?: string;
}) => {
  const note = sourceNote(response, monitorId);
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-[12.5px] text-quiet">
        <label className="flex cursor-pointer items-center gap-2">
          <Checkbox
            aria-label="Pre-incident only"
            checked={preOnly}
            onCheckedChange={(next) => onPreOnlyChange(Boolean(next))}
          />
          Pre-incident only
          <span className="text-faint">stop at the first failed check</span>
        </label>
        {response?.from && response.to && (
          <span className="whitespace-nowrap text-faint">
            {formatDate(response.from, TIME_FORMAT)} to {formatDate(response.to, TIME_FORMAT)}
          </span>
        )}
      </div>
      {note && <p className="text-[12.5px] text-quiet">{note}</p>}
      <DataTable
        columns={columns}
        data={response?.entries ?? []}
        isLoading={loading}
        error={failed ? { message: "Unable to load logs", onRetry } : undefined}
        getRowId={(entry, index) => entry.id ?? `log-${index}`}
        emptyMessage={
          preOnly
            ? "No logs before this incident opened."
            : "No logs recorded around this incident."
        }
        defaultPageSize={25}
      />
    </div>
  );
};
