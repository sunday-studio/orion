import type { LifecycleStep } from "@/design-system";
import type {
  ApiAlertDeliveryResponse,
  ApiIncidentDetailSummary,
  ApiIncidentResponse,
  ApiIncidentTimelineItemResponse,
} from "@/orion-sdk";
import { formatWhen } from "@/features/overview/overview-time";

export type IncidentMoments = {
  notifiedAt?: string;
  acknowledgedAt?: string;
};

const earliest = (values: (string | undefined)[]) =>
  values
    .filter((value): value is string => Boolean(value))
    .sort((a, b) => new Date(a).getTime() - new Date(b).getTime())[0];

/** Notified / acknowledged moments, read from the incident record, deliveries and timeline. */
export const incidentMoments = (
  incident: ApiIncidentResponse,
  deliveries: ApiAlertDeliveryResponse[],
  timeline: ApiIncidentTimelineItemResponse[],
  summary?: ApiIncidentDetailSummary,
): IncidentMoments => ({
  notifiedAt: summary?.notified_at ?? earliest(deliveries.map((delivery) => delivery.created_at)),
  acknowledgedAt:
    summary?.acknowledged_at ??
    (incident as { acknowledged_at?: string }).acknowledged_at ??
    earliest(
      timeline
        .filter((item) => item.type?.toLowerCase().includes("acknowledge"))
        .map((item) => item.created_at),
    ),
});

export const lifecycleSteps = (
  incident: ApiIncidentResponse,
  moments: IncidentMoments,
  now: number,
): LifecycleStep[] => {
  const resolved = incident.status === "resolved";
  const acknowledged = Boolean(moments.acknowledgedAt) || incident.status === "acknowledged";
  return [
    { label: "Detected", state: "done", time: formatWhen(incident.opened_at, now) },
    {
      label: "Notified",
      state: moments.notifiedAt ? "done" : "todo",
      time: formatWhen(moments.notifiedAt, now),
    },
    {
      label: "Acknowledged",
      state: acknowledged || resolved ? "done" : "now",
      time: moments.acknowledgedAt
        ? formatWhen(moments.acknowledgedAt, now)
        : resolved
          ? "-"
          : "waiting",
    },
    {
      label: "Resolved",
      state: resolved ? "done" : "todo",
      time: resolved ? formatWhen(incident.resolved_at, now) : "-",
    },
  ];
};
