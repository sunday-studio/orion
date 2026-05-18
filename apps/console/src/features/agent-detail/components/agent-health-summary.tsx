import type { ApiUptimeDayBucketResponse } from "@/orion-sdk";
import { formatPercent } from "./agent-detail-utils";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";

type AgentHealthSummaryProps = {
  activeIncidentCount: number;
  degradedCount: number;
  downCount: number;
  status: string;
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

export const AgentHealthSummary = ({
  activeIncidentCount,
  degradedCount,
  downCount,
  upCount,
  uptimeBuckets,
  uptimePercent,
  agentId,
}: AgentHealthSummaryProps) => {
  const recentBuckets = uptimeBuckets.slice(-7);
  const hasActiveIncidents = activeIncidentCount > 0;

  return (
    <section className="space-y-3">
      <div className="grid gap-1 sm:grid-cols-4">
        <SummaryCell label="up monitors" value={upCount} />
        <SummaryCell label="down monitors" value={downCount} />
        <SummaryCell label="degraded monitors" value={degradedCount} />
        <SummaryCell label="90d uptime" value={formatPercent(uptimePercent)} />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
        {hasActiveIncidents && (
          <Link
            to={`/incidents?agent=${encodeURIComponent(agentId)}`}
            className="text-neutral-600 hover:text-indigo-700 hover:underline"
          >
            <p>
              {activeIncidentCount} active incident{activeIncidentCount === 1 ? "" : "s"} on this
              agent.
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

const SummaryCell = ({ label, value }: { label: string; value: string | number }) => (
  <div className="flex min-h-24 flex-col justify-between bg-neutral-100 px-3 py-2">
    <div className="text-neutral-600 text-sm capitalize">{label}</div>
    <div className="font-medium text-2xl text-neutral-950">{value}</div>
  </div>
);
