import type { ServiceNodeDiscoveryItem, ServiceNodeDiscoveryKind } from "@/orion-sdk";

/** The seven kinds the node reports, in display order. */
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

export const groupItems = (items: ServiceNodeDiscoveryItem[]) => {
  const groups = new Map<string, ServiceNodeDiscoveryItem[]>();
  for (const item of items) {
    const key = item.kind ?? "other";
    groups.set(key, [...(groups.get(key) ?? []), item]);
  }
  return [...groups.entries()].sort(([a], [b]) => kindRank(a) - kindRank(b));
};

/** Core turns every kind except log files into a monitor (or, for storage, a tracking request). */
export const canAccept = (item: ServiceNodeDiscoveryItem) =>
  item.status !== "monitored" && !item.dismissed && item.kind !== "log";

export const managedLabel = (managedIn?: string) =>
  managedIn === "file"
    ? "Managed in file"
    : managedIn === "console"
      ? "Managed in Console"
      : managedIn === "scan"
        ? "Accepted from scan"
        : "Monitored";

/** What accepting a suggestion sets up, in the words of the review dialog. */
export const acceptWhat = (item: ServiceNodeDiscoveryItem) =>
  item.kind === "disk"
    ? { what: "Above 90% full", when: "First breach" }
    : item.kind === "storage"
      ? { what: "Size over time", when: "No alerts" }
      : item.kind === "port"
        ? { what: "Listening", when: "2 failed checks" }
        : item.kind === "pm2"
          ? { what: "Online", when: "2 failed checks" }
          : { what: "Running", when: "2 failed checks" };

export const confidenceLabel = (confidence?: string) =>
  confidence ? `${confidence.charAt(0).toUpperCase()}${confidence.slice(1)}` : "Low";

/** Scan states the node reports for one kind: scanned, off, blocked, failed or absent. */
export const kindScan = (kind: ServiceNodeDiscoveryKind) =>
  kind.state === "off"
    ? "off"
    : kind.state === "blocked" || kind.state === "failed"
      ? "problem"
      : kind.state === "absent"
        ? "absent"
        : "scanned";

export const isKindProblem = (kind: ServiceNodeDiscoveryKind) => kindScan(kind) === "problem";
