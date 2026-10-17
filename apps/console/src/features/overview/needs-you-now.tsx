import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Button,
  Dot,
  FieldPanel,
  type LifecycleStep,
  LifecycleTrack,
  RaisedCard,
  StatusBadge,
} from "@/design-system";
import {
  type ApiIncidentResponse,
  getGetIncidentsQueryKey,
  useAcknowledgeIncident,
} from "@/orion-sdk";
import { formatSpan, formatWhen } from "./overview-time";

const SEVERITY_RANK: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

export const mostSevere = (incidents: ApiIncidentResponse[]) =>
  [...incidents].sort(
    (a, b) => (SEVERITY_RANK[b.severity ?? ""] ?? 0) - (SEVERITY_RANK[a.severity ?? ""] ?? 0),
  )[0];

const severityDot = (severity?: string) =>
  severity === "critical" || severity === "high"
    ? "#e5484d"
    : severity === "medium"
      ? "#f59e0b"
      : "#a1a1aa";

/** Detected, Notified, Acknowledge, Resolved; times come from the incident record. */
export const lifecycleSteps = (incident: ApiIncidentResponse, now: number): LifecycleStep[] => {
  const opened = formatWhen(incident.opened_at, now);
  const acknowledged = incident.status === "acknowledged" || incident.status === "covered";
  const resolved = incident.status === "resolved";
  return [
    { label: "Detected", state: "done", time: opened },
    {
      label: "Notified",
      state: incident.notification_status === "pending" ? "todo" : "done",
      time: opened,
    },
    {
      label: "Acknowledge",
      state: acknowledged || resolved ? "done" : "now",
      time: acknowledged || resolved ? formatWhen(incident.last_event_at, now) : "waiting",
    },
    {
      label: "Resolved",
      state: resolved ? "done" : "todo",
      time: resolved ? formatWhen(incident.resolved_at, now) : "-",
    },
  ];
};

export const NeedsYouNow = ({ incident, now }: { incident: ApiIncidentResponse; now: number }) => {
  const queryClient = useQueryClient();
  const acknowledge = useAcknowledgeIncident({
    mutation: {
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() }),
    },
  });
  const openedMs = incident.opened_at ? new Date(incident.opened_at).getTime() : now;
  const canAcknowledge =
    incident.allowed_actions?.acknowledge?.allowed ?? incident.status === "open";

  return (
    <FieldPanel
      title="Needs you now"
      aside={`Open for ${formatSpan(openedMs, now, true)}`}
      aria-label="Needs you now"
    >
      <RaisedCard>
        <div className="flex flex-col gap-3.5 px-4 py-3.5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <Dot color={severityDot(incident.severity)} />
              <b className="text-[15px] font-medium">{incident.title ?? "Incident"}</b>
            </div>
            <StatusBadge status={incident.severity}>{incident.severity}</StatusBadge>
          </div>
          <LifecycleTrack steps={lifecycleSteps(incident, now)} />
          <div className="flex items-center justify-between">
            <span className="text-[12.5px] text-mid">
              Latest: {incident.latest_event ?? "no events recorded yet"}
            </span>
            <div className="flex items-center gap-1.5">
              <Link to={`/incidents/${incident.id}`}>
                <Button size="sm" variant="secondary">
                  Details
                </Button>
              </Link>
              {canAcknowledge && (
                <Button
                  size="sm"
                  disabled={acknowledge.isPending}
                  onClick={() => incident.id && acknowledge.mutate({ id: incident.id })}
                >
                  Acknowledge
                </Button>
              )}
            </div>
          </div>
        </div>
      </RaisedCard>
    </FieldPanel>
  );
};
