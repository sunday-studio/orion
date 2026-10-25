import {
  Button,
  FieldPanel,
  LifecycleTrack,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  RaisedCard,
  StatBox,
  StatusBadge,
} from "@/design-system";
import type { ApiIncidentDetailSummary, ApiIncidentTimelineItemResponse } from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { type NumberedIncident, incidentLabel } from "../incident-format";
import { type IncidentMoments, lifecycleSteps } from "./incident-lifecycle";

const eventTone = (type = "") =>
  type.includes("resolve") || type.includes("recover")
    ? { name: "Resolved", color: "#2a9f69" }
    : type.includes("acknowledge")
      ? { name: "Acknowledged", color: "#a1a1aa" }
      : type.includes("open")
        ? { name: "Opened", color: "#e5484d" }
        : { name: type.replace(/_/g, " ") || "Event", color: "#a1a1aa" };

const span = (seconds?: number) =>
  seconds === undefined ? "-" : formatSpan(0, seconds * 1000, false, true);

type Props = {
  incident: NumberedIncident;
  summary?: ApiIncidentDetailSummary;
  moments: IncidentMoments;
  timeline: ApiIncidentTimelineItemResponse[];
  now: number;
  canReopen: boolean;
  pending: boolean;
  onReopen: () => void;
  onPublicUpdate: () => void;
};

/** Resolved incident: summary stats, finished lifecycle and a short event table. */
export const ResolvedIncident = ({
  incident,
  summary,
  moments,
  timeline,
  now,
  canReopen,
  pending,
  onReopen,
  onPublicUpdate,
}: Props) => {
  const events = timeline
    .filter((item) => !item.alert_delivery_id)
    .sort((a, b) => new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime());
  const duration = summary?.duration_seconds ?? summary?.time_to_resolve_seconds;
  return (
    <div className="w-full pb-8">
      <div className="flex items-start justify-between gap-3 pb-4 pt-7">
        <div className="min-w-0">
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
            {incident.title ?? "Untitled incident"}
          </h1>
          <div className="mt-1 flex items-center gap-2 text-[13.5px] text-quiet">
            <StatusBadge status="resolved">Resolved</StatusBadge>
            <span>
              {incidentLabel(incident)} · <span className="capitalize">{incident.severity}</span>
              {duration !== undefined && ` · ${Math.max(1, Math.round(duration / 60))} minutes`}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <Menu>
            <MenuTrigger render={<Button size="sm" variant="secondary" />}>More</MenuTrigger>
            <MenuContent align="end">
              <MenuItem disabled={!canReopen || pending} onClick={onReopen}>
                Reopen incident
              </MenuItem>
            </MenuContent>
          </Menu>
          <Button size="sm" onClick={onPublicUpdate}>
            Post public update
          </Button>
        </div>
      </div>
      <div className="grid grid-cols-3 gap-3">
        <StatBox label="Time to acknowledge" value={span(summary?.time_to_acknowledge_seconds)} />
        <StatBox label="Time to resolve" value={span(summary?.time_to_resolve_seconds)} />
        <StatBox label="Failed checks" value={summary?.failed_checks ?? "-"} />
      </div>
      <FieldPanel title="Lifecycle" className="mt-3">
        <RaisedCard>
          <div className="px-4 py-5">
            <LifecycleTrack steps={lifecycleSteps(incident, moments, now)} />
          </div>
        </RaisedCard>
      </FieldPanel>
      <FieldPanel
        title="Timeline"
        aside={`${events.length} ${events.length === 1 ? "event" : "events"}`}
        className="mt-3"
      >
        <RaisedCard>
          <table className="w-full text-left text-[13px]">
            <thead>
              <tr className="border-b border-line-soft text-xs text-quiet">
                <th className="px-3.5 py-2 font-normal">Time</th>
                <th className="px-3.5 py-2 font-normal">Event</th>
                <th className="px-3.5 py-2 font-normal">Detail</th>
              </tr>
            </thead>
            <tbody>
              {events.map((item, index) => {
                const tone = eventTone(item.type);
                return (
                  <tr key={item.id ?? index} className="border-b border-line-soft last:border-b-0">
                    <td className="px-3.5 py-2.5 font-mono text-quiet">
                      {formatDate(item.created_at, TIME_FORMAT)}
                    </td>
                    <td className="px-3.5 py-2.5 font-medium">
                      <span
                        aria-hidden
                        className="mr-1 inline-block size-[7px] rounded-full align-middle"
                        style={{ background: tone.color }}
                      />
                      {tone.name}
                    </td>
                    <td className="px-3.5 py-2.5 text-quiet">{item.note ?? item.message}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </RaisedCard>
      </FieldPanel>
    </div>
  );
};
