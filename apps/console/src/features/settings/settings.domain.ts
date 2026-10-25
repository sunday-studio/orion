export const asNumber = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
};

export const archiveCount = (result: {
  node_reports_archived?: number;
  monitor_reports_archived?: number;
}) => (result.node_reports_archived ?? 0) + (result.monitor_reports_archived ?? 0);
