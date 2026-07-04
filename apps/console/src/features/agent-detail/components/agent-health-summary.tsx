import { DetailCard, type DetailCardVariant } from "@/components/shared/detail-card";
import type { ApiUptimeDayBucketResponse } from "@/orion-sdk";
import { formatPercent } from "../agent-detail.domain";
import { Link } from "react-router-dom";
import { cn } from "@/utils/cn";
import { StatusBadge, toStatus } from "@/components/shared/status-badges";

type AgentHealthSummaryProps = {
  activeIncidentCount: number;
  availabilityHealth?: string;
  degradedCount: number;
  downCount: number;
  monitorHealth?: string;
  staleCount: number;
  status: string;
  statusReason?: string;
  totalCount: number;
  unknownCount: number;
  upCount: number;
  uptimeBuckets: ApiUptimeDayBucketResponse[];
  uptimePercent?: number;
  agentId: string;
};

const bucketFillClassName = (bucket: ApiUptimeDayBucketResponse) => {
  const percent = bucket.uptime_percent ?? 0;

  if (!bucket.total) return "bg-neutral-300";
  if (percent >= 99) return "bg-emerald-400";
  if (percent >= 95) return "bg-amber-300";
  return "bg-rose-400";
};

const statusVariant = (status?: string): DetailCardVariant => {
  if (status === "up" || status === "healthy") return "emerald";
  if (status === "down" || status === "error") return "rose";
  if (status === "degraded" || status === "stale" || status === "maintenance") return "amber";
  return "neutral";
};

export const AgentHealthSummary = ({
  activeIncidentCount,
  availabilityHealth,
  degradedCount,
  downCount,
  monitorHealth,
  staleCount,
  upCount,
  status,
  statusReason,
  totalCount,
  unknownCount,
  uptimeBuckets,
  uptimePercent,
  agentId,
}: AgentHealthSummaryProps) => {
  const recentBuckets = uptimeBuckets.slice(-7);
  const hasActiveIncidents = activeIncidentCount > 0;

  return (
    <section className="space-y-3">
      <div className="grid gap-1 sm:grid-cols-4">
        <StatusCell
          label="server availability"
          status={availabilityHealth ?? status}
          detail={statusReason}
          variant={statusVariant(availabilityHealth ?? status)}
        />
        <StatusCell
          label="monitor rollup"
          status={monitorHealth ?? "unknown"}
          detail={`${totalCount} active`}
          variant={statusVariant(monitorHealth)}
        />
        <SummaryCell
          label="monitor issues"
          value={downCount + degradedCount + staleCount + unknownCount}
          detail={`${upCount} up / ${downCount} down / ${degradedCount} degraded / ${staleCount} stale / ${unknownCount} unknown`}
          variant={downCount + degradedCount + staleCount + unknownCount > 0 ? "rose" : "emerald"}
        />
        <SummaryCell label="90d uptime" value={formatPercent(uptimePercent)} variant="indigo" />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
        {hasActiveIncidents && (
          <Link
            to={`/incidents?agent=${encodeURIComponent(agentId)}`}
            className="text-neutral-600 hover:text-indigo-700 hover:underline"
          >
            <p>
              {activeIncidentCount} active incident{activeIncidentCount === 1 ? "" : "s"} on this
              server.
            </p>
          </Link>
        )}

        {recentBuckets.length > 0 && (
          <div className="flex gap-0.5 ml-auto">
            {recentBuckets.map((bucket) => (
              <div
                key={bucket.date}
                title={`${bucket.date}: ${formatPercent(bucket.uptime_percent)}`}
                className="flex h-7 w-2 items-end bg-neutral-100"
              >
                <div
                  className={cn("mt-auto w-full", bucketFillClassName(bucket))}
                  style={{ height: `${Math.max(4, bucket.uptime_percent ?? 0)}%` }}
                />
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
};

const SummaryCell = ({
  detail,
  label,
  variant = "neutral",
  value,
}: {
  detail?: string;
  label: string;
  variant?: DetailCardVariant;
  value: string | number;
}) => (
  <DetailCard
    className="min-h-24"
    contentClassName="flex min-h-16 flex-col justify-between"
    variant={variant}
  >
    <div className="text-neutral-600 text-sm capitalize">{label}</div>
    <div className="space-y-1">
      <div className="font-medium text-2xl text-neutral-950">{value}</div>
      {detail && <div className="text-xs leading-snug text-neutral-600">{detail}</div>}
    </div>
  </DetailCard>
);

const StatusCell = ({
  detail,
  label,
  status,
  variant = "neutral",
}: {
  detail?: string;
  label: string;
  status: string;
  variant?: DetailCardVariant;
}) => (
  <DetailCard
    className="min-h-24"
    contentClassName="flex min-h-16 flex-col justify-between"
    variant={variant}
  >
    <div className="text-neutral-600 text-sm capitalize">{label}</div>
    <div className="space-y-1">
      <StatusBadge value={toStatus(status)} />
      {detail && <div className="text-xs leading-snug text-neutral-600">{detail}</div>}
    </div>
  </DetailCard>
);
