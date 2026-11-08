import { StatusBadge } from "@/design-system";
import { useGetServiceLogCounts } from "@/orion-sdk";
import { serviceHealth } from "../services.domain";
import { errorRate } from "../timeline-model";

/** Errors per minute over the last five minutes, from the service's own apps. "-" when no app is linked. */
export const ServiceErrorRate = ({
  serviceId,
  appCount,
  health,
}: {
  serviceId: string;
  appCount: number;
  health?: string;
}) => {
  const counts = useGetServiceLogCounts(
    serviceId,
    { from: "-5m", step_seconds: 60 },
    { query: { enabled: appCount > 0, staleTime: 30_000 } },
  );
  if (appCount === 0) return <span className="text-faint">-</span>;
  if (!counts.data) return <span className="text-faint">…</span>;
  const rate = errorRate(counts.data.buckets ?? [], 5);
  return <StatusBadge status={serviceHealth(health)}>{rate}</StatusBadge>;
};
