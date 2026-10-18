import { useNavigate } from "react-router-dom";
import { cn } from "@/utils/cn";
import {
  Checkbox,
  GridFooter,
  GridRow,
  GridTable,
  ListPagination,
  type ListPaginationProps,
  StatusBadge,
} from "@/design-system";
import { type NumberedIncident, incidentLabel, relativeAgo, statusLabel } from "../incident-format";

export const INCIDENT_COLUMNS = "18px 70px minmax(0,1.6fr) 120px 80px minmax(0,0.8fr) 110px 70px";

export type IncidentTableProps = {
  incidents: NumberedIncident[];
  selected: ReadonlySet<string>;
  onToggle: (id: string) => void;
  onToggleAll: (checked: boolean) => void;
  now: number;
  /** Count of closed incidents the status filter is hiding, for the footer hint. */
  hiddenResolved?: number;
  onShowResolved?: () => void;
  /** Pagination footer, drawn inside the table container. */
  pagination?: ListPaginationProps;
  /** Dim the table while a refetch (filter, page) is in flight. */
  fetching?: boolean;
};

const notificationLabel = (status?: string) =>
  status === "failed"
    ? "Not delivered"
    : status
      ? status.charAt(0).toUpperCase() + status.slice(1)
      : "None";

const severityLabel = (severity?: string) =>
  severity ? severity.charAt(0).toUpperCase() + severity.slice(1) : "Unknown";

export const IncidentTable = ({
  incidents,
  selected,
  onToggle,
  onToggleAll,
  now,
  hiddenResolved = 0,
  onShowResolved,
  pagination,
  fetching,
}: IncidentTableProps) => {
  const navigate = useNavigate();
  const allSelected = incidents.length > 0 && incidents.every((i) => i.id && selected.has(i.id));

  return (
    <div
      className={cn(
        "ds-settled overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
        fetching && "ds-fetching",
      )}
    >
      <GridTable>
        <GridRow columns={INCIDENT_COLUMNS} header>
          <span>
            <Checkbox
              aria-label="Select all incidents"
              checked={allSelected}
              onCheckedChange={(checked) => onToggleAll(Boolean(checked))}
            />
          </span>
          <span>ID</span>
          <span>Incident</span>
          <span>Status</span>
          <span>Severity</span>
          <span>Monitor</span>
          <span>Notified</span>
          <span>Opened</span>
        </GridRow>
        {incidents.map((incident) => {
          const id = incident.id ?? "";
          const isSelected = selected.has(id);
          return (
            <GridRow
              key={id}
              flipId={id}
              columns={INCIDENT_COLUMNS}
              className={isSelected ? "bg-[var(--color-selected-row)]" : "cursor-pointer"}
              onClick={() => navigate(`/incidents/${id}`)}
            >
              <span onClick={(event) => event.stopPropagation()}>
                <Checkbox
                  aria-label={`Select ${incidentLabel(incident)}`}
                  checked={isSelected}
                  onCheckedChange={() => onToggle(id)}
                />
              </span>
              <span className="font-mono text-xs text-faint">{incidentLabel(incident)}</span>
              <span>
                <b className="font-medium">{incident.title ?? "Untitled incident"}</b>
              </span>
              <span>
                <StatusBadge
                  status={incident.status === "acknowledged" ? "degraded" : incident.status}
                  shape={incident.status === "open" ? "square" : undefined}
                >
                  {statusLabel(incident.status)}
                </StatusBadge>
              </span>
              <span>
                <StatusBadge status={incident.severity}>
                  {severityLabel(incident.severity)}
                </StatusBadge>
              </span>
              <span className="font-mono text-xs text-faint">
                {incident.monitor_name ?? incident.agent_name ?? "-"}
              </span>
              <span>
                <StatusBadge
                  status={incident.notification_status}
                  shape={incident.notification_status === "failed" ? "square" : undefined}
                >
                  {notificationLabel(incident.notification_status)}
                </StatusBadge>
              </span>
              <span className="text-faint">{relativeAgo(incident.opened_at, now)}</span>
            </GridRow>
          );
        })}
        {hiddenResolved > 0 && (
          <GridFooter>
            {hiddenResolved} resolved {hiddenResolved === 1 ? "incident is" : "incidents are"}{" "}
            hidden by this filter.{" "}
            <button type="button" className="underline" onClick={onShowResolved}>
              Show them
            </button>
          </GridFooter>
        )}
      </GridTable>
      {pagination && <ListPagination {...pagination} />}
    </div>
  );
};
