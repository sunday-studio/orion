import {
  NotificationBadge,
  SeverityBadge,
  StatusBadge,
  toNotificationStatus,
  toSeverity,
  toStatus,
} from "@/components/shared/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type { ApiIncidentResponse, ApiIncidentTimelineItemResponse } from "@/orion-sdk";
import { Link } from "react-router-dom";
import {
  ComponentImpactList,
  DetailGroup,
  type IncidentComponentImpact,
  durationLabel,
} from "./incident-detail-utils";

const hasText = (value?: string | null) => Boolean(value?.trim());

const SummaryField = ({ label, value }: { label: string; value: string }) => (
  <div className="min-w-0">
    <div className="text-xs font-medium text-neutral-500">{label}</div>
    <div className="mt-1 break-words text-sm font-medium text-neutral-950">{value}</div>
  </div>
);

const TimelineEventCard = ({ item }: { item?: ApiIncidentTimelineItemResponse }) => (
  <DetailGroup
    title="Latest Timeline Event"
    description={item ? formatDate(item.created_at, DATE_TIME_FORMAT) : undefined}
    contentClassName="space-y-4"
  >
    <div className="space-y-3">
      <div className="break-words text-sm font-medium text-neutral-950">
        {item?.message ?? "No timeline event recorded."}
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-2 border-t border-neutral-200 pt-3">
        <SummaryField label="type" value={item?.type ?? "unknown"} />
        <SummaryField label="source" value={item?.source ?? "unknown"} />
      </div>
    </div>
  </DetailGroup>
);

export const IncidentOverviewCards = ({
  impactedComponents,
  incident,
}: {
  impactedComponents: IncidentComponentImpact[];
  incident: ApiIncidentResponse;
}) => {
  const hasAffectedLinks = Boolean(incident.agent_id || incident.monitor_id);
  const resolution = incident.resolution_kind || (incident.status === "covered" ? "covered" : "");
  const statusVariant = incident.status === "resolved" ? "emerald" : "rose";

  return (
    <div className="grid gap-3 lg:grid-cols-3">
      <DetailGroup
        title="Incident"
        description="Current lifecycle state"
        variant={statusVariant}
        contentClassName="space-y-4"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="text-xs font-medium text-neutral-500">current state</div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <StatusBadge value={toStatus(incident.status)} />
              <SeverityBadge value={toSeverity(incident.severity)} />
              <NotificationBadge
                value={toNotificationStatus(incident.notification_status)}
                fallback="no notification status"
              />
            </div>
          </div>
          <div className="shrink-0 text-right">
            <div className="text-xs font-medium text-neutral-500">duration</div>
            <div className="mt-1 text-lg font-medium leading-none text-neutral-950 tabular-nums">
              {durationLabel(incident)}
            </div>
          </div>
        </div>
        {resolution && (
          <div className="border-t border-neutral-200 pt-3">
            <SummaryField label="resolution" value={resolution} />
          </div>
        )}
      </DetailGroup>

      <DetailGroup
        title="Affected"
        description="Monitor and service impact"
        variant="indigo"
        contentClassName="space-y-4"
      >
        <div className="min-w-0">
          <div className="text-xs font-medium text-neutral-500">monitor</div>
          <div className="mt-1 truncate text-base font-medium text-neutral-950">
            {incident.monitor_name ?? "Unknown monitor"}
          </div>
          <div className="mt-1 truncate text-sm text-neutral-600">
            {incident.agent_name ?? "Unknown server"} / {incident.monitor_type ?? "unknown"}
          </div>
        </div>
        <div className="border-t border-neutral-200 pt-3">
          <div className="mb-2 text-xs font-medium text-neutral-500">components</div>
          <ComponentImpactList components={impactedComponents} />
        </div>
        {hasAffectedLinks && (
          <div className="flex flex-wrap gap-4 border-t border-neutral-200 pt-3 text-sm">
            {incident.agent_id && (
              <Link
                to={`/servers/${incident.agent_id}?tab=monitors&incident=${encodeURIComponent(incident.id ?? "")}`}
                className="font-medium hover:text-neutral-600"
              >
                View server
              </Link>
            )}
            {incident.monitor_id && (
              <Link
                to={`/monitors/${incident.monitor_id}?incident=${encodeURIComponent(incident.id ?? "")}`}
                className="font-medium hover:text-neutral-600"
              >
                View monitor
              </Link>
            )}
          </div>
        )}
      </DetailGroup>

      <DetailGroup
        title="Timing"
        description="When the incident changed"
        variant="amber"
        contentClassName="space-y-4"
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <SummaryField label="opened" value={formatDate(incident.opened_at, DATE_TIME_FORMAT)} />
          <SummaryField
            label="latest event"
            value={formatDate(incident.last_event_at, DATE_TIME_FORMAT)}
          />
          {hasText(incident.resolved_at) && (
            <SummaryField
              label="resolved"
              value={formatDate(incident.resolved_at, DATE_TIME_FORMAT)}
            />
          )}
          {hasText(incident.covered_until) && (
            <SummaryField
              label="covered until"
              value={formatDate(incident.covered_until, DATE_TIME_FORMAT)}
            />
          )}
        </div>
        {incident.coverage_note && (
          <div className="border-t border-neutral-200 pt-3">
            <SummaryField label="coverage note" value={incident.coverage_note} />
          </div>
        )}
      </DetailGroup>
    </div>
  );
};

export const IncidentEvidenceCards = ({
  latestTimelineItem,
}: {
  latestTimelineItem?: ApiIncidentTimelineItemResponse;
}) => <TimelineEventCard item={latestTimelineItem} />;
