export const DELIVERY_LIMIT = 30;
export const alertTabs = ["logs", "channels", "rules"] as const;
export const deliveryStatuses = [
  "all",
  "pending",
  "sent",
  "failed",
  "suppressed",
  "cooldown",
] as const;
export const deliveryTypeOptions = [
  { value: "all", label: "All types" },
  { value: "webhook", label: "Webhook" },
  { value: "slack", label: "Slack" },
  { value: "discord", label: "Discord" },
  { value: "email", label: "Email" },
] as const;
export const alertChannelTypeOptions = [
  { value: "webhook", label: "Webhook", urlPlaceholder: "https://example.com/webhook" },
  {
    value: "slack",
    label: "Slack",
    urlPlaceholder: "https://hooks.slack.com/services/...",
  },
  {
    value: "discord",
    label: "Discord",
    urlPlaceholder: "https://discord.com/api/webhooks/...",
  },
] as const;
export const manageableAlertChannelTypes = alertChannelTypeOptions.map((option) => option.value);
export const alertEventOptions = [
  { value: "incident_opened", label: "Incident opened" },
  { value: "incident_resolved", label: "Incident resolved" },
] as const;
export const deliveryEventOptions = [
  { value: "all", label: "All events" },
  ...alertEventOptions,
] as const;
export const defaultAlertEvents = alertEventOptions.map((event) => event.value);

export const boolLabel = (value?: boolean) => (value ? "yes" : "no");

export const alertChannelTypeLabel = (value?: string) =>
  alertChannelTypeOptions.find((option) => option.value === value)?.label ??
  (value === "email" ? "Email" : (value ?? "unknown"));

export const eventLabel = (value?: string) =>
  alertEventOptions.find((event) => event.value === value)?.label ?? value ?? "unknown";

export const getMutationErrorMessage = (error: unknown, fallback = "Unable to create webhook.") => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? fallback);
  }
  return fallback;
};
