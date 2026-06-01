import type {
  ApiAlertChannelResponse,
  ApiAlertRouteRequest,
  ApiAlertRouteResponse,
} from "@/orion-sdk";
import { alertEventOptions } from "./alert-constants";

export const severityOptions = [
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "critical", label: "Critical" },
  { value: "error", label: "Error" },
] as const;

export const groupingOptions = [
  { value: "suppress", label: "Suppress repeats" },
  { value: "delayed_summary", label: "Delayed summary" },
  { value: "none", label: "No grouping" },
] as const;

export type RouteFormState = {
  agentIds: string;
  channelIds: string[];
  enabled: boolean;
  eventTypes: string[];
  groupingDelaySeconds: string;
  groupingPolicy: string;
  monitorIds: string;
  monitorTypes: string;
  name: string;
  priority: string;
  severities: string[];
  suppress: boolean;
};

export type DryRunFormState = {
  agentId: string;
  eventType: string;
  incidentId: string;
  monitorId: string;
  monitorType: string;
  severity: string;
};

export const defaultRouteForm = (channels: ApiAlertChannelResponse[]): RouteFormState => ({
  agentIds: "",
  channelIds: channels.find((channel) => channel.id)?.id ? [channels[0].id ?? ""] : [],
  enabled: true,
  eventTypes: alertEventOptions.map((option) => option.value),
  groupingDelaySeconds: "300",
  groupingPolicy: "suppress",
  monitorIds: "",
  monitorTypes: "",
  name: "",
  priority: "100",
  severities: [],
  suppress: false,
});

export const defaultDryRunForm: DryRunFormState = {
  agentId: "",
  eventType: "incident_opened",
  incidentId: "",
  monitorId: "",
  monitorType: "",
  severity: "high",
};

const splitList = (value: string) =>
  value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);

const joinList = (value?: string[]) => (value ?? []).join("\n");

export const routeToForm = (
  route: ApiAlertRouteResponse,
  channels: ApiAlertChannelResponse[],
): RouteFormState => ({
  agentIds: joinList(route.agent_ids),
  channelIds: (route.channel_ids ?? []).filter((id) =>
    channels.some((channel) => channel.id === id),
  ),
  enabled: route.enabled ?? true,
  eventTypes: route.event_types?.length
    ? route.event_types
    : alertEventOptions.map((option) => option.value),
  groupingDelaySeconds: String(route.grouping_delay_seconds ?? 300),
  groupingPolicy: route.grouping_policy ?? "suppress",
  monitorIds: joinList(route.monitor_ids),
  monitorTypes: joinList(route.monitor_types),
  name: route.name ?? "",
  priority: String(route.priority ?? 100),
  severities: route.severities ?? [],
  suppress: route.suppress ?? false,
});

export const routeToRequest = (route: ApiAlertRouteResponse): ApiAlertRouteRequest => ({
  agent_ids: route.agent_ids ?? [],
  channel_ids: route.suppress ? [] : (route.channel_ids ?? []),
  enabled: route.enabled ?? true,
  event_types: route.event_types ?? alertEventOptions.map((option) => option.value),
  grouping_delay_seconds: route.grouping_delay_seconds ?? 300,
  grouping_policy: route.grouping_policy ?? "suppress",
  monitor_ids: route.monitor_ids ?? [],
  monitor_types: route.monitor_types ?? [],
  name: route.name ?? "",
  priority: route.priority ?? 100,
  severities: route.severities ?? [],
  suppress: route.suppress ?? false,
});

export const formToRequest = (form: RouteFormState): ApiAlertRouteRequest => ({
  agent_ids: splitList(form.agentIds),
  channel_ids: form.suppress ? [] : form.channelIds,
  enabled: form.enabled,
  event_types: form.eventTypes,
  grouping_delay_seconds: Math.max(Number.parseInt(form.groupingDelaySeconds, 10) || 300, 1),
  grouping_policy: form.groupingPolicy,
  monitor_ids: splitList(form.monitorIds),
  monitor_types: splitList(form.monitorTypes),
  name: form.name.trim(),
  priority: Math.max(Number.parseInt(form.priority, 10) || 0, 0),
  severities: form.severities,
  suppress: form.suppress,
});

export const channelName = (channels: ApiAlertChannelResponse[], id?: string) =>
  channels.find((channel) => channel.id === id)?.name ?? id ?? "unknown";

export const formatList = (values?: string[], formatter?: (value: string) => string) => {
  if (!values?.length) return "Any";
  return values.map((value) => formatter?.(value) ?? value).join(", ");
};

export const toggleValue = (values: string[], value: string, checked: boolean) => {
  if (checked) return Array.from(new Set([...values, value]));
  return values.filter((item) => item !== value);
};
