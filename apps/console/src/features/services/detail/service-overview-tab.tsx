import { useState } from "react";
import { Callout, StatBox } from "@/design-system";
import { incidentLabel } from "@/features/incidents/incident-format";
import { formatSpan, useNow } from "@/features/overview/overview-time";
import {
  type ServiceServiceDetail,
  type ServiceServiceSuggestionMember,
  linkServiceItem,
  useGetIncident,
  useGetServiceLogCounts,
  useListServiceSuggestions,
} from "@/orion-sdk";
import { firstActiveIncident, memberKey, memberReason, memberRef } from "../services.domain";
import {
  formatAvailability,
  lowestAvailability,
  timesNormal,
  totalErrors,
} from "../timeline-model";
import { useRefreshServices } from "../use-service-mutations";
import {
  AppsTable,
  EmptyRow,
  LinkedPanel,
  MonitorsTable,
  useAvailability30d,
} from "./linked-tables";
import { ServiceResourcesTab } from "./service-resources-tab";
import { ServiceTimeline } from "./service-timeline";

/** A member Core thinks belongs here. Linking is the person's call; "Not this one" hides it for this visit. */
const SuggestedMember = ({
  member,
  serviceId,
  onDismiss,
}: {
  member: ServiceServiceSuggestionMember;
  serviceId: string;
  onDismiss: () => void;
}) => {
  const refresh = useRefreshServices();
  const [error, setError] = useState("");
  const link = async () => {
    try {
      await linkServiceItem(serviceId, memberRef(member));
      refresh(serviceId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to link.");
    }
  };
  return (
    <Callout tone="warn">
      <b>{member.name}</b> looks like part of this service. {memberReason(member)}{" "}
      <button type="button" className="underline" onClick={() => void link()}>
        Link it
      </button>
      {" · "}
      <button type="button" className="underline" onClick={onDismiss}>
        Not this one
      </button>
      {error && <span className="ml-2 text-(--tone-down-fg)">{error}</span>}
    </Callout>
  );
};

export const ServiceOverviewTab = ({
  service,
  onEditLinks,
}: {
  service: ServiceServiceDetail;
  onEditLinks: () => void;
}) => {
  const serviceId = service.id ?? "";
  const now = useNow(30_000);
  const monitors = service.monitors ?? [];
  const counts = useGetServiceLogCounts(serviceId, { from: "-1h", step_seconds: 300 });
  const day = useGetServiceLogCounts(serviceId, { from: "-24h", step_seconds: 3600 });
  const incidentId = firstActiveIncident(monitors);
  const incident = useGetIncident(incidentId ?? "", { query: { enabled: Boolean(incidentId) } });
  const suggestions = useListServiceSuggestions({ service_id: serviceId });
  const [dismissed, setDismissed] = useState<ReadonlySet<string>>(new Set());
  const members = (suggestions.data?.suggestions ?? [])
    .flatMap((suggestion) => suggestion.members ?? [])
    .filter((member) => !dismissed.has(memberKey(member)));
  const down = monitors.filter((monitor) => monitor.health === "down").length;
  const availability = lowestAvailability([
    ...useAvailability30d(monitors.map((monitor) => monitor.id ?? "")).values(),
  ]);
  const hasApps = (service.apps ?? []).length > 0;
  const normal = hasApps ? timesNormal(day.data?.buckets ?? [], now) : undefined;
  const openedAt = incident.data?.incident?.opened_at;
  const opened = openedAt ? new Date(openedAt).getTime() : undefined;

  return (
    <div className="flex flex-col gap-3.5">
      <div className="grid gap-3 sm:grid-cols-4">
        <StatBox
          label="Availability, 30 days"
          value={availability === undefined ? "-" : formatAvailability(availability)}
        />
        <StatBox
          label="Errors, last hour"
          value={
            <>
              {hasApps && counts.data
                ? totalErrors(counts.data.buckets ?? []).toLocaleString()
                : "-"}
              {normal !== undefined && (
                <small className="ml-2 text-[13px] font-normal tracking-normal text-faint">
                  {normal}× normal
                </small>
              )}
            </>
          }
        />
        <StatBox
          label="Open incidents"
          value={
            <>
              {service.open_incident_count ?? 0}
              {incident.data?.incident && (
                <small className="ml-2 text-[13px] font-normal tracking-normal text-faint">
                  {incidentLabel(incident.data.incident)}
                  {opened !== undefined && `, ${formatSpan(opened, now, false, true)}`}
                </small>
              )}
            </>
          }
        />
        <StatBox
          label="Monitors down"
          value={<span className={down > 0 ? "text-(--tone-down-fg)" : undefined}>{down}</span>}
          of={monitors.length}
        />
      </div>
      {members.slice(0, 3).map((member) => (
        <SuggestedMember
          key={memberKey(member)}
          member={member}
          serviceId={serviceId}
          onDismiss={() => setDismissed(new Set(dismissed).add(memberKey(member)))}
        />
      ))}
      <ServiceTimeline
        serviceId={serviceId}
        monitors={monitors}
        incidentOpenedAt={incident.data?.incident?.opened_at}
      />
      <LinkedPanel title="Monitors" action="Link monitor" onAction={onEditLinks}>
        {monitors.length > 0 ? (
          <MonitorsTable monitors={monitors} />
        ) : (
          <EmptyRow>No monitors linked yet.</EmptyRow>
        )}
      </LinkedPanel>
      <LinkedPanel title="Apps" action="Link app" onAction={onEditLinks}>
        {hasApps ? (
          <AppsTable apps={service.apps ?? []} now={now} />
        ) : (
          <EmptyRow>No apps linked yet.</EmptyRow>
        )}
      </LinkedPanel>
      <LinkedPanel title="Resources" action="Link resource" onAction={onEditLinks}>
        <ServiceResourcesTab resources={service.resources ?? []} />
      </LinkedPanel>
    </div>
  );
};
