// Pure rules for the Services screens. No React and no `@/` imports, so `node --test` runs this file.
import type {
  ApiApp,
  ApiMonitorResponse,
  ServiceAppResourceSeries,
  ServiceServiceInput,
  ServiceServiceLinkRef,
  ServiceServiceSuggestion,
  ServiceServiceSuggestionMember,
  ServiceServiceSummary,
} from "../../orion-sdk/index.ts";

export const SERVICE_TABS = [
  "overview",
  "monitors",
  "logs",
  "resources",
  "incidents",
  "settings",
] as const;
export type ServiceTab = (typeof SERVICE_TABS)[number];

/** Reads `?tab=`; anything unknown falls back to the overview. */
export const serviceTabFromParam = (value: string | null | undefined): ServiceTab =>
  SERVICE_TABS.find((tab) => tab === value) ?? "overview";

export const serviceHref = (id: string, tab?: ServiceTab) =>
  `/services/${encodeURIComponent(id)}${tab && tab !== "overview" ? `?tab=${tab}` : ""}`;

export type ServiceHealth = "up" | "degraded" | "down" | "unknown";

export const serviceHealth = (value?: string): ServiceHealth =>
  value === "up" || value === "degraded" || value === "down" ? value : "unknown";

export const healthLabel = (health: ServiceHealth) =>
  ({ up: "Up", degraded: "Degraded", down: "Down", unknown: "No data" })[health];

export const HEALTH_FILTERS = ["all", "down", "degraded", "up", "unknown"] as const;
export type HealthFilter = (typeof HEALTH_FILTERS)[number];

export const healthFilterFromParam = (value: string | null | undefined): HealthFilter =>
  HEALTH_FILTERS.find((filter) => filter === value) ?? "all";

export const healthCounts = (services: readonly ServiceServiceSummary[]) => {
  const counts: Record<HealthFilter, number> = {
    all: services.length,
    down: 0,
    degraded: 0,
    up: 0,
    unknown: 0,
  };
  for (const service of services) counts[serviceHealth(service.health)] += 1;
  return counts;
};

const rank: Record<ServiceHealth, number> = { down: 0, degraded: 1, unknown: 2, up: 3 };

/** Worst first, then by name, so a broken service is never below the fold. */
export const filterServices = (
  services: readonly ServiceServiceSummary[],
  filter: { query?: string; health?: HealthFilter; environment?: string },
) => {
  const query = (filter.query ?? "").trim().toLowerCase();
  return services
    .filter((service) => {
      if (
        filter.health &&
        filter.health !== "all" &&
        serviceHealth(service.health) !== filter.health
      )
        return false;
      if (
        filter.environment &&
        filter.environment !== "all" &&
        service.environment !== filter.environment
      )
        return false;
      if (!query) return true;
      return [service.name, service.slug, service.environment].some((field) =>
        (field ?? "").toLowerCase().includes(query),
      );
    })
    .sort(
      (a, b) =>
        rank[serviceHealth(a.health)] - rank[serviceHealth(b.health)] ||
        (a.name ?? "").localeCompare(b.name ?? ""),
    );
};

export const environmentsOf = (services: readonly ServiceServiceSummary[]) =>
  [
    ...new Set(
      services.map((service) => service.environment).filter((env): env is string => Boolean(env)),
    ),
  ].sort();

export const summaryLine = (counts: { monitors?: number; apps?: number; resources?: number }) =>
  [
    plural(counts.monitors ?? 0, "monitor"),
    plural(counts.apps ?? 0, "app"),
    plural(counts.resources ?? 0, "resource"),
  ].join(", ");

export const plural = (count: number, noun: string) => `${count} ${noun}${count === 1 ? "" : "s"}`;

/** Likely when every member is a high-confidence match, otherwise a softer "Maybe". */
export type Likelihood = "likely" | "maybe";
export const suggestionLikelihood = (suggestion: ServiceServiceSuggestion): Likelihood =>
  (suggestion.members ?? []).length > 0 &&
  (suggestion.members ?? []).every((member) => member.confidence === "high")
    ? "likely"
    : "maybe";

/** Likely groups start ticked; softer ones are left for the person to opt into. */
export const defaultSelectedSuggestions = (suggestions: readonly ServiceServiceSuggestion[]) =>
  suggestions.flatMap((suggestion, index) =>
    suggestionLikelihood(suggestion) === "likely" ? [suggestionKey(suggestion, index)] : [],
  );

export const suggestionKey = (suggestion: ServiceServiceSuggestion, index: number) =>
  `${suggestion.service_id ?? "new"}:${suggestion.slug ?? suggestion.name ?? index}`;

export const memberRef = (member: ServiceServiceSuggestionMember): ServiceServiceLinkRef =>
  member.kind === "resource"
    ? { kind: "resource", node_id: member.node_id, key: member.key }
    : { kind: member.kind, id: member.id };

export const memberKey = (member: ServiceServiceSuggestionMember) =>
  member.kind === "resource"
    ? `resource:${member.node_id}:${member.key}`
    : `${member.kind}:${member.id}`;

export type SuggestionPlan = {
  /** Set for a group that starts a new service; absent when it joins an existing one. */
  create?: ServiceServiceInput;
  serviceId?: string;
  refs: ServiceServiceLinkRef[];
};

/** What accepting one suggestion does. Nothing is linked until the person accepts. */
export const planSuggestion = (
  suggestion: ServiceServiceSuggestion,
  skippedMembers: ReadonlySet<string> = new Set(),
): SuggestionPlan => {
  const refs = (suggestion.members ?? [])
    .filter((member) => !skippedMembers.has(memberKey(member)))
    .map(memberRef);
  if (suggestion.service_id) return { serviceId: suggestion.service_id, refs };
  return {
    create: {
      name: suggestion.name,
      slug: suggestion.slug,
      environment: suggestion.environment,
    },
    refs,
  };
};

/** Phrase for the reason line. Core sends a sentence; fall back to what matched. */
export const memberReason = (member: ServiceServiceSuggestionMember) => {
  if (member.reason) return member.reason;
  const field = (member.matched_on ?? "").replaceAll("_", " ");
  const against = (member.against ?? "").replaceAll("_", " ");
  return field && against ? `Its ${field} matches the ${against}.` : "The names look alike.";
};

export const unlinkParams = (ref: ServiceServiceLinkRef) =>
  ref.kind === "resource"
    ? { kind: "resource" as const, node_id: ref.node_id, key: ref.key }
    : { kind: ref.kind ?? "monitor", target_id: ref.id };

/** Linking something that already belongs to another service moves it. */
export const describeMove = (
  previousServiceId: string | undefined,
  services: readonly ServiceServiceSummary[],
) => {
  if (!previousServiceId) return undefined;
  const previous = services.find((service) => service.id === previousServiceId);
  return `Moved from ${previous?.name ?? "another service"}.`;
};

/** A Down/Degraded monitor with an open incident is the one to open from the header. */
export const firstActiveIncident = (
  monitors: readonly { active_incident_id?: string; health?: string }[],
) => monitors.find((monitor) => monitor.active_incident_id)?.active_incident_id;

export type LinkCandidate = {
  key: string;
  ref: ServiceServiceLinkRef;
  label: string;
  hint?: string;
  /** The service this thing belongs to now, when it is not the one being edited. */
  elsewhere?: string;
};

const elsewhereOf = (current: string | undefined, serviceId: string | undefined) =>
  current && current !== serviceId ? current : undefined;

/** Monitors that can be linked. System monitors cannot, and ones already here are left out. */
export const monitorCandidates = (
  monitors: readonly ApiMonitorResponse[],
  serviceId?: string,
): LinkCandidate[] =>
  monitors
    .filter((monitor) => monitor.id && !monitor.system && monitor.service_id !== serviceId)
    .map((monitor) => ({
      key: `monitor:${monitor.id}`,
      ref: { kind: "monitor", id: monitor.id },
      label: monitor.name ?? monitor.id ?? "",
      hint: monitor.type,
      elsewhere: elsewhereOf(monitor.service_id, serviceId),
    }));

export const appCandidates = (apps: readonly ApiApp[], serviceId?: string): LinkCandidate[] =>
  apps
    .filter((app) => app.id && app.service_id !== serviceId)
    .map((app) => ({
      key: `app:${app.id}`,
      ref: { kind: "app", id: app.id },
      label: app.name ?? app.slug ?? app.id ?? "",
      hint: app.environment,
      elsewhere: elsewhereOf(app.service_id, serviceId),
    }));

export const resourceCandidates = (
  series: readonly ServiceAppResourceSeries[],
  node: { id: string; name?: string },
  serviceId?: string,
): LinkCandidate[] =>
  series
    .filter((item) => item.key && item.service_id !== serviceId)
    .map((item) => ({
      key: `resource:${node.id}:${item.key}`,
      ref: { kind: "resource", node_id: node.id, key: item.key },
      label: item.name ?? item.key ?? "",
      hint: [item.kind, node.name && `on ${node.name}`].filter(Boolean).join(" "),
      elsewhere: elsewhereOf(item.service_id, serviceId),
    }));

/** Turns ticked candidate keys back into link refs, in list order. */
export const refsForKeys = (candidates: readonly LinkCandidate[], keys: ReadonlySet<string>) =>
  candidates.filter((candidate) => keys.has(candidate.key)).map((candidate) => candidate.ref);

/** "Currently in billing" for a candidate that would move, looked up by service id. */
export const elsewhereNote = (
  candidate: LinkCandidate,
  services: readonly ServiceServiceSummary[],
) =>
  candidate.elsewhere
    ? `Moves from ${services.find((service) => service.id === candidate.elsewhere)?.name ?? "another service"}`
    : undefined;

/** Slug-ish default for a name typed into the new-service form. Core derives its own when blank. */
export const canCreateService = (name: string) => name.trim().length > 0;
