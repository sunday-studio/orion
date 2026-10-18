import { Link } from "react-router-dom";
import { Button } from "@/design-system";
import type { ApiIncidentRelatedIncidentResponse } from "@/orion-sdk";
import { formatSpan } from "@/features/overview/overview-time";
import { formatDate } from "@/utils/date";
import { type NumberedIncident, incidentLabel } from "../incident-format";

const TIME = "HH:mm:ss";

const Section = ({ title, children }: { title: string; children: React.ReactNode }) => (
  <section className="border-b border-[#ececea] px-[18px] py-3.5 last:border-b-0 dark:border-hairline">
    <h4 className="mb-2 text-xs font-semibold text-[#3f3f46] dark:text-ink">{title}</h4>
    {children}
  </section>
);

const Facts = ({ children }: { children: React.ReactNode }) => (
  <div className="grid grid-cols-[110px_1fr] gap-x-2 gap-y-1.5 text-[12.5px] [&>:nth-child(odd)]:text-quiet">
    {children}
  </div>
);

type Props = {
  incident: NumberedIncident;
  now: number;
  notified?: string;
  notifiedAt?: string;
  acknowledgedAt?: string;
  related: ApiIncidentRelatedIncidentResponse[];
  onDraftUpdate: () => void;
};

/** Facts panel: source, people, timestamps, durations, status page, related. */
export const IncidentSidePanel = ({
  incident,
  now,
  notified,
  notifiedAt,
  acknowledgedAt,
  related,
  onDraftUpdate,
}: Props) => {
  const opened = incident.opened_at ? new Date(incident.opened_at).getTime() : undefined;
  const ackedAt = acknowledgedAt;
  const acked = ackedAt ? new Date(ackedAt).getTime() : undefined;
  const resolved = incident.resolved_at ? new Date(incident.resolved_at).getTime() : undefined;
  const clock = (iso?: string) => (iso ? formatDate(iso, TIME) : undefined);
  const muted = <span className="text-faint">Not yet</span>;
  return (
    <aside aria-label="Incident details" className="h-fit rounded-xl bg-[#f7f7f6] dark:bg-card">
      <Section title="Source">
        <Facts>
          <span>Monitor</span>
          <span className="truncate">{incident.monitor_name ?? "-"}</span>
          <span>Type</span>
          <span>{incident.monitor_type ?? "-"}</span>
          <span>Server</span>
          <span className="truncate">{incident.agent_name ?? "-"}</span>
        </Facts>
      </Section>
      <Section title="People">
        <Facts>
          <span>Responder</span>
          <span className="text-faint">Unassigned</span>
          <span>Notified</span>
          <span>{notified ?? "-"}</span>
        </Facts>
      </Section>
      <Section title="Timestamps">
        <Facts>
          <span>Started</span>
          <span className="font-mono">{clock(incident.opened_at) ?? "-"}</span>
          <span>Notified</span>
          <span className="font-mono">{clock(notifiedAt) ?? "-"}</span>
          <span>Acknowledged</span>
          <span className="font-mono">{clock(ackedAt) ?? muted}</span>
        </Facts>
      </Section>
      <Section title="Duration">
        <Facts>
          <span>Time to ack</span>
          {opened && acked ? (
            <span>{formatSpan(opened, acked, true)}</span>
          ) : opened && !resolved ? (
            <span className="text-[#be123c]">
              {formatSpan(opened, now, false, true)} and counting
            </span>
          ) : (
            <span className="text-faint">-</span>
          )}
          <span>Time to resolve</span>
          {opened && resolved ? (
            <span>{formatSpan(opened, resolved, true)}</span>
          ) : (
            <span className="text-faint">-</span>
          )}
        </Facts>
      </Section>
      <Section title="Status page">
        <div className="text-[12.5px] text-quiet">No public update yet</div>
        <Button size="sm" variant="secondary" className="mt-2" onClick={onDraftUpdate}>
          Draft update
        </Button>
      </Section>
      {related.length > 0 && (
        <Section title="Related">
          <div className="flex flex-col gap-1">
            {related.slice(0, 5).map((item) => (
              <Link
                key={item.id}
                to={`/incidents/${item.id ?? ""}`}
                className="truncate text-[12.5px] underline"
              >
                {incidentLabel(item)} {item.title}
              </Link>
            ))}
          </div>
        </Section>
      )}
    </aside>
  );
};
