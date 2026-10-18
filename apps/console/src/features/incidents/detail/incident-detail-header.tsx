import { Link } from "react-router-dom";
import { Button, StatusBadge } from "@/design-system";
import { formatSpan } from "@/features/overview/overview-time";
import { type NumberedIncident, incidentLabel, statusLabel } from "../incident-format";

type Props = {
  incident: NumberedIncident;
  now: number;
  actionPending: boolean;
  canAcknowledge: boolean;
  canCover: boolean;
  canReopen: boolean;
  canResolve: boolean;
  onAcknowledge: () => void;
  onCover: () => void;
  onReopen: () => void;
  onResolve: () => void;
};

/** Breadcrumb, 28px title, status badges and lifecycle actions (canvas incident-detail `.phd`). */
export const IncidentDetailHeader = ({
  incident,
  now,
  actionPending,
  canAcknowledge,
  canCover,
  canReopen,
  canResolve,
  onAcknowledge,
  onCover,
  onReopen,
  onResolve,
}: Props) => {
  const opened = incident.opened_at ? new Date(incident.opened_at).getTime() : now;
  const closed = incident.resolved_at ? new Date(incident.resolved_at).getTime() : now;
  const span = formatSpan(opened, closed, true);
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
      <div className="min-w-0">
        <div className="text-[13.5px] leading-[1.4] text-quiet">
          <Link to="/incidents" className="hover:text-ink">
            Incidents
          </Link>{" "}
          / {incidentLabel(incident)}
        </div>
        <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
          {incident.title ?? "Untitled incident"}
        </h1>
        <div className="mt-1.5 flex items-center gap-2">
          <StatusBadge status={incident.status}>{statusLabel(incident.status)}</StatusBadge>
          <StatusBadge status={incident.severity}>{incident.severity ?? "unknown"}</StatusBadge>
          <span className="text-[13.5px] text-quiet">
            {incident.resolved_at ? `Lasted ${span}` : `Open for ${span}`}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-1.5">
        {canCover && (
          <Button size="sm" variant="secondary" disabled={actionPending} onClick={onCover}>
            Cover
          </Button>
        )}
        {canResolve && (
          <Button size="sm" variant="secondary" disabled={actionPending} onClick={onResolve}>
            Resolve
          </Button>
        )}
        {canReopen && (
          <Button size="sm" variant="secondary" disabled={actionPending} onClick={onReopen}>
            Reopen
          </Button>
        )}
        {canAcknowledge && (
          <Button size="sm" disabled={actionPending} onClick={onAcknowledge}>
            Acknowledge
          </Button>
        )}
      </div>
    </div>
  );
};
