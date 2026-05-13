import { useGetAgents, useGetHealthSummary, useGetIncidents } from "@/orion-sdk";

const SummaryItem = ({ label, value }: { label: string; value: number | string }) => {
  return (
    <div className="min-w-28">
      <div className="text-base font-medium">{value}</div>
      <div className="text-sm text-neutral-600">{label}</div>
    </div>
  );
};

export const AttentionSummary = () => {
  const summaryResponse = useGetHealthSummary();
  const incidentsResponse = useGetIncidents({ limit: 100 });
  const staleServersResponse = useGetAgents({ stale_only: true, limit: 1 });

  if (summaryResponse.error || incidentsResponse.error || staleServersResponse.error) {
    return <div className="py-2 text-sm">Unable to load the full attention summary.</div>;
  }

  return (
    <div className="flex flex-wrap gap-x-8 gap-y-3 py-2">
      <SummaryItem label="open incidents" value={incidentsResponse.data?.count ?? 0} />
      <SummaryItem label="down monitors" value={summaryResponse.data?.monitors?.down ?? 0} />
      <SummaryItem
        label="degraded monitors"
        value={summaryResponse.data?.monitors?.degraded ?? 0}
      />
      <SummaryItem label="stale servers" value={staleServersResponse.data?.count ?? 0} />
    </div>
  );
};
