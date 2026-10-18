export const stateTone = (state?: string) =>
  state === "published" ? "up" : state === "maintenance" ? "maintenance" : "unknown";

export const stateLabel = (state?: string) =>
  state === "published" ? "Published" : state === "maintenance" ? "Maintenance" : "Draft";

export const publicStatusLabel = (status?: string) =>
  ({
    operational: "Operational",
    up: "Operational",
    degraded: "Degraded",
    partial_outage: "Partial outage",
    major_outage: "Major outage",
    down: "Major outage",
    maintenance: "Maintenance",
  })[status ?? ""] ?? "Unknown";

export const publicStatusTone = (status?: string) =>
  status === "operational" || status === "up"
    ? "up"
    : status === "degraded" || status === "partial_outage"
      ? "degraded"
      : status === "major_outage" || status === "down"
        ? "down"
        : status === "maintenance"
          ? "maintenance"
          : "unknown";

export const statusDot = (status?: string) =>
  publicStatusTone(status) === "up"
    ? "#2a9f69"
    : publicStatusTone(status) === "degraded"
      ? "#f59e0b"
      : publicStatusTone(status) === "down"
        ? "#e5484d"
        : "#a1a1aa";

export const slugify = (value: string) =>
  value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
