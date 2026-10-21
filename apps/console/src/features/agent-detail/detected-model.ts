import type { ApiAgentMonitorDefinitionRequest, ServiceAgentDiscoveryItem } from "@/orion-sdk";

/** Group titles in display order; anything the agent reports that is not listed is shown last. */
export const KIND_GROUPS: { match: RegExp; title: string }[] = [
  { match: /^(systemd|service)s?$/i, title: "Services" },
  { match: /^(docker|container)s?$/i, title: "Containers" },
  { match: /^pm2$/i, title: "Apps (pm2)" },
  { match: /^(port|listen)s?$/i, title: "Ports" },
  { match: /^(disk|mount)s?$/i, title: "Disks" },
  { match: /^(log|logfile)s?$/i, title: "Logs" },
  { match: /^(process|proc)(es)?$/i, title: "Processes" },
];

export const kindTitle = (kind?: string) =>
  KIND_GROUPS.find((group) => group.match.test(kind ?? ""))?.title ??
  `${(kind ?? "Other").charAt(0).toUpperCase()}${(kind ?? "other").slice(1)}`;

const kindRank = (kind?: string) => {
  const index = KIND_GROUPS.findIndex((group) => group.match.test(kind ?? ""));
  return index === -1 ? KIND_GROUPS.length : index;
};

export const groupItems = (items: ServiceAgentDiscoveryItem[]) => {
  const groups = new Map<string, ServiceAgentDiscoveryItem[]>();
  for (const item of items) {
    const key = item.kind ?? "other";
    groups.set(key, [...(groups.get(key) ?? []), item]);
  }
  return [...groups.entries()].sort(([a], [b]) => kindRank(a) - kindRank(b));
};

/**
 * Core can turn disk, memory, process and port suggestions into monitor definitions today.
 * Services, containers, pm2 apps and log sources are applied from the agent's config.
 */
export const monitorDefinition = (
  item: ServiceAgentDiscoveryItem,
): ApiAgentMonitorDefinitionRequest | undefined => {
  const name = item.name ?? "";
  const kind = item.kind ?? "";
  if (/^(disk|mount)s?$/i.test(kind)) {
    return {
      check_type: "disk",
      name: `Disk ${name}`,
      params: { path: name, threshold_percent: 90 },
    };
  }
  if (/^(port|listen)s?$/i.test(kind)) {
    const port = Number(/\d{2,5}/.exec(`${name} ${item.detail ?? ""}`)?.[0]);
    return port ? { check_type: "port", name: `Port ${port}`, params: { port } } : undefined;
  }
  if (/^(process|proc)(es)?$/i.test(kind)) {
    return { check_type: "process", name: `Process ${name}`, params: { process_name: name } };
  }
  return undefined;
};

export const canAccept = (item: ServiceAgentDiscoveryItem) =>
  item.status !== "monitored" &&
  !item.dismissed &&
  !item.managed_in?.includes("file") &&
  Boolean(monitorDefinition(item));
