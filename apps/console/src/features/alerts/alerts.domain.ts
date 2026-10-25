import type {
  ApiAlertChannelResponse,
  ApiAlertRuleRequest,
  ApiAlertRuleResponse,
} from "@/orion-sdk";
import { alertEventOptions } from "./components/alert-constants";

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
  cooldownSeconds: string;
  nodeIds: string;
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

export const defaultRouteForm = (channels: ApiAlertChannelResponse[]): RouteFormState => ({
  nodeIds: "",
  cooldownSeconds: "0",
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

const splitList = (value: string) =>
  value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);

const joinList = (value?: string[]) => (value ?? []).join("\n");

export const routeToForm = (
  route: ApiAlertRuleResponse,
  channels: ApiAlertChannelResponse[],
): RouteFormState => ({
  nodeIds: joinList(route.node_ids),
  cooldownSeconds: String(route.cooldown_seconds ?? 0),
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

export const formToRequest = (form: RouteFormState): ApiAlertRuleRequest => ({
  node_ids: splitList(form.nodeIds),
  cooldown_seconds: Number.parseInt(form.cooldownSeconds, 10) || 0,
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

export const toggleValue = (values: string[], value: string, checked: boolean) => {
  if (checked) return Array.from(new Set([...values, value]));
  return values.filter((item) => item !== value);
};
