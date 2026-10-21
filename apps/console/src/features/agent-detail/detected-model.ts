import type { ServiceAgentDiscoveryItem } from "@/orion-sdk";

/** The seven kinds the agent reports, in display order. */
const KINDS = [
  { kind: "service", title: "Services" },
  { kind: "container", title: "Containers" },
  { kind: "pm2", title: "Apps (pm2)" },
  { kind: "port", title: "Ports" },
  { kind: "disk", title: "Disks" },
  { kind: "log", title: "Logs" },
  { kind: "storage", title: "Storage" },
] as const;

export const kindTitle = (kind?: string) =>
  KINDS.find((group) => group.kind === kind)?.title ??
  `${(kind ?? "Other").charAt(0).toUpperCase()}${(kind ?? "other").slice(1)}`;

const kindRank = (kind?: string) => {
  const index = KINDS.findIndex((group) => group.kind === kind);
  return index === -1 ? KINDS.length : index;
};

export const groupItems = (items: ServiceAgentDiscoveryItem[]) => {
  const groups = new Map<string, ServiceAgentDiscoveryItem[]>();
  for (const item of items) {
    const key = item.kind ?? "other";
    groups.set(key, [...(groups.get(key) ?? []), item]);
  }
  return [...groups.entries()].sort(([a], [b]) => kindRank(a) - kindRank(b));
};

/** Core turns every kind except log files into a monitor (or, for storage, a tracking request). */
export const canAccept = (item: ServiceAgentDiscoveryItem) =>
  item.status !== "monitored" && !item.dismissed && item.kind !== "log";

/** Why a suggestion has no checkbox. */
export const acceptHint = (item: ServiceAgentDiscoveryItem) =>
  item.kind === "log"
    ? "Needs an ingest key, set up with orion-agent setup"
    : "Accept with orion-agent setup";

export const managedLabel = (managedIn?: string) =>
  managedIn === "file"
    ? "Managed in file"
    : managedIn === "console"
      ? "Managed in Console"
      : managedIn === "scan"
        ? "Accepted from scan"
        : "Monitored";
