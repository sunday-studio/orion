import type { ApiIncidentResponse } from "@/orion-sdk";

/** Core supplies `display_id` (INC-0231); a short id stands in for older records. */
export type NumberedIncident = ApiIncidentResponse & { number?: number; display_id?: string };

export const incidentLabel = (incident: NumberedIncident) => {
  if (incident.display_id) return incident.display_id;
  if (typeof incident.number === "number") return `INC-${String(incident.number).padStart(4, "0")}`;
  return `INC-${(incident.id ?? "")
    .replace(/[^a-zA-Z0-9]/g, "")
    .slice(0, 4)
    .toUpperCase()}`;
};

export const statusOptions = [
  { value: "open", label: "Open", group: "Active" },
  { value: "acknowledged", label: "Acknowledged", group: "Active" },
  { value: "covered", label: "Covered", group: "Active" },
  { value: "resolved", label: "Resolved", group: "Closed" },
] as const;

export const severityOptions = [
  { value: "critical", label: "Critical" },
  { value: "high", label: "High" },
  { value: "medium", label: "Medium" },
  { value: "low", label: "Low" },
] as const;

export const sinceOptions = [
  { value: "24h", label: "last 24 hours", ms: 86_400_000 },
  { value: "7d", label: "last 7 days", ms: 7 * 86_400_000 },
  { value: "30d", label: "last 30 days", ms: 30 * 86_400_000 },
] as const;

export const notificationOptions = [
  { value: "sent", label: "Sent" },
  { value: "failed", label: "Not delivered" },
  { value: "pending", label: "Pending" },
  { value: "suppressed", label: "Suppressed" },
] as const;

export const DEFAULT_STATUSES = ["open", "acknowledged"] as const;
export const DEFAULT_SINCE = "7d";

export const statusLabel = (status?: string) =>
  statusOptions.find((option) => option.value === status)?.label ?? status ?? "Unknown";

/** "30m ago", "16h ago", "3d ago". */
export const relativeAgo = (iso: string | undefined, now: number) => {
  if (!iso) return "-";
  const seconds = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
};
