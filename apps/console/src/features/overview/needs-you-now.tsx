import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Button,
  FieldPanel,
  type LifecycleStep,
  LifecycleTrack,
  RaisedCard,
  StatusBadge,
} from "@/design-system";
import {
  type ApiOverviewIncidentResponse,
  getGetOverviewQueryKey,
  useAcknowledgeIncident,
} from "@/orion-sdk";
import { formatSpan, formatWhen } from "./overview-time";

/** Detected, Notified, Acknowledge, Resolved; times come from the overview record. */
const lifecycleSteps = (incident: ApiOverviewIncidentResponse, now: number): LifecycleStep[] => {
  const acknowledged = Boolean(incident.acknowledged_at);
  const resolved = incident.status === "resolved";
  return [
    { label: "Detected", state: "done", time: formatWhen(incident.opened_at, now) },
    {
      label: "Notified",
      state: incident.notified_at ? "done" : "todo",
      time: incident.notified_at ? formatWhen(incident.notified_at, now) : "-",
    },
    {
      label: "Acknowledge",
      state: acknowledged || resolved ? "done" : "now",
      time: acknowledged ? formatWhen(incident.acknowledged_at, now) : resolved ? "-" : "waiting",
    },
    {
      label: "Resolved",
      state: resolved ? "done" : "todo",
      time: resolved ? formatWhen(incident.resolved_at, now) : "-",
    },
  ];
};

export const NeedsYouNow = ({
  incident,
  now,
}: {
  incident: ApiOverviewIncidentResponse;
  now: number;
}) => {
  const queryClient = useQueryClient();
  const acknowledge = useAcknowledgeIncident({
    mutation: {
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() }),
    },
  });
  const openedMs = incident.opened_at ? new Date(incident.opened_at).getTime() : now;
  const canAcknowledge = incident.status === "open";

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
              <b className="text-[15px] font-medium">{incident.title ?? "Incident"}</b>
            </div>
            <StatusBadge status={incident.severity}>{incident.severity}</StatusBadge>
          </div>
          <LifecycleTrack steps={lifecycleSteps(incident, now)} />
          <div className="flex items-center justify-between">
            <span className="text-[12.5px] text-mid">
              Latest: {incident.summary ?? "no events recorded yet"}
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
