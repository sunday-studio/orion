import { useGetOverview } from "@/orion-sdk";

export type HealthChip = {
  text: string;
  /** "issues" when something needs a look, "ok" when all is well, "idle" while there is nothing to say. */
  tone: "issues" | "ok" | "idle" | "error";
  /** Ongoing incidents, shown as the count on the Incidents item. */
  incidents: number;
};

const issues = (count: number) => `${count} ${count === 1 ? "issue" : "issues"}`;

/** The issue count shown at the bottom of the sidebar (was the header chip). */
export const useHealthChip = (): HealthChip => {
  const overview = useGetOverview();
  const data = overview.data;
  const incidents = data?.ongoing_incident_count ?? 0;
  if (overview.isError) return { text: "Unable to load", tone: "error", incidents };
  if (data?.state === "empty") return { text: "No data yet", tone: "idle", incidents };
  if (incidents > 0) return { text: issues(incidents), tone: "issues", incidents };
  const health = data?.health_issue_count ?? 0;
  if (health > 0) return { text: issues(health), tone: "issues", incidents };
  if (overview.isLoading) return { text: "Checking...", tone: "idle", incidents };
  return { text: "All systems normal", tone: "ok", incidents };
};
