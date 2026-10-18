import { useState } from "react";
import { Link } from "react-router-dom";
import { Button, Input } from "@/design-system";
import type { ApiIncidentDetailSummary, ApiIncidentRelatedIncidentResponse } from "@/orion-sdk";
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

const seconds = (value?: number) =>
  value === undefined ? undefined : formatSpan(0, value * 1000, true);

const Responder = ({ name, onSave }: { name?: string; onSave: (name: string) => void }) => {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(name ?? "");
  if (!editing) {
    return (
      <button
        type="button"
        className={name ? "text-left" : "text-left underline"}
        onClick={() => {
          setDraft(name ?? "");
          setEditing(true);
        }}
      >
        {name || "Assign someone"}
      </button>
    );
  }
  return (
    <form
      className="flex gap-1"
      onSubmit={(event) => {
        event.preventDefault();
        onSave(draft.trim());
        setEditing(false);
      }}
    >
      <Input
        autoFocus
        aria-label="Responder"
        className="h-6 text-[12.5px]"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
      />
      <Button type="submit" size="sm" variant="secondary">
        Save
      </Button>
    </form>
  );
};

type Props = {
  incident: NumberedIncident;
  summary?: ApiIncidentDetailSummary;
  now: number;
  notifiedAt?: string;
  acknowledgedAt?: string;
  related: ApiIncidentRelatedIncidentResponse[];
  onDraftUpdate: () => void;
  onSetResponder: (name: string) => void;
};

/** Facts panel: source, people, timestamps, durations, status page, related. */
export const IncidentSidePanel = ({
  incident,
  summary,
  now,
  notifiedAt,
  acknowledgedAt,
  related,
  onDraftUpdate,
  onSetResponder,
}: Props) => {
  const opened = incident.opened_at ? new Date(incident.opened_at).getTime() : undefined;
  const resolved = incident.resolved_at ? new Date(incident.resolved_at).getTime() : undefined;
  const clock = (iso?: string) => (iso ? formatDate(iso, TIME) : undefined);
  const source = summary?.source;
  const toAck =
    seconds(summary?.time_to_acknowledge_seconds) ??
    (opened && acknowledgedAt
      ? formatSpan(opened, new Date(acknowledgedAt).getTime(), true)
      : undefined);
  const toResolve =
    seconds(summary?.time_to_resolve_seconds) ??
    (opened && resolved ? formatSpan(opened, resolved, true) : undefined);
  const updates = summary?.status_page?.published_updates ?? 0;
  return (
    <aside aria-label="Incident details" className="h-fit rounded-xl bg-[#f7f7f6] dark:bg-card">
      <Section title="Source">
        <Facts>
          <span>Monitor</span>
          <span className="truncate">{source?.monitor_name ?? incident.monitor_name ?? "-"}</span>
          <span>Target</span>
          <span className="truncate">{source?.target ?? incident.monitor_type ?? "-"}</span>
          <span>Server</span>
          <span className="truncate">{source?.server_name ?? incident.agent_name ?? "-"}</span>
        </Facts>
      </Section>
      <Section title="People">
        <Facts>
          <span>Responder</span>
          <Responder name={incident.responder} onSave={onSetResponder} />
          <span>Notified</span>
          <span>{summary?.notified_channels?.join(", ") || "-"}</span>
        </Facts>
      </Section>
      <Section title="Timestamps">
        <Facts>
          <span>Started</span>
          <span className="font-mono">{clock(incident.opened_at) ?? "-"}</span>
          <span>Notified</span>
          <span className="font-mono">{clock(notifiedAt) ?? "-"}</span>
          <span>Acknowledged</span>
          <span className="font-mono">
            {clock(acknowledgedAt) ?? <span className="text-faint">Not yet</span>}
          </span>
        </Facts>
      </Section>
      <Section title="Duration">
        <Facts>
          <span>Time to ack</span>
          {toAck ? (
            <span>{toAck}</span>
          ) : opened && !resolved ? (
            <span className="text-[#be123c]">
              {formatSpan(opened, now, false, true)} and counting
            </span>
          ) : (
            <span className="text-faint">-</span>
          )}
          <span>Time to resolve</span>
          {toResolve ? <span>{toResolve}</span> : <span className="text-faint">-</span>}
        </Facts>
      </Section>
      <Section title="Status page">
        <div className="text-[12.5px] text-quiet">
          {updates > 0
            ? `${updates} public ${updates === 1 ? "update" : "updates"} published`
            : summary?.status_page?.draft_exists
              ? "Draft saved, not published"
              : "No public update yet"}
        </div>
        <Button size="sm" variant="secondary" className="mt-2" onClick={onDraftUpdate}>
          {updates > 0 ? "Post update" : "Draft update"}
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
