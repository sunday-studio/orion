import { useGetHealthSummary } from "@/orion-sdk";

const healthLabel = {
  up: "All good",
  down: "Issues",
  degraded: "Issues",
  maintenance: "Maintenance",
  stale: "Issues",
  unknown: "Unknown",
} as const;

export const HomeHeader = () => {
  const summaryResponse = useGetHealthSummary();
  const overallHealth = summaryResponse.data?.overall_health ?? "unknown";
  const monitorCount = summaryResponse.data?.monitors?.total ?? 0;
  const serverCount = summaryResponse.data?.agents?.total ?? 0;
  const label = healthLabel[overallHealth as keyof typeof healthLabel] ?? "Unknown";

  return (
    <div className="flex items-end justify-between gap-4">
      <div>
        <h1 className="text-base font-medium">Home</h1>
        <p className="text-sm text-neutral-600">
          {summaryResponse.isLoading
            ? "Checking server health..."
            : `${label}: ${serverCount} servers, ${monitorCount} monitors`}
        </p>
      </div>
      <div className="text-sm font-medium">{label}</div>
    </div>
  );
};
