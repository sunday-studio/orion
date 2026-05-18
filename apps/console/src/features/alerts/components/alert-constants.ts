export const DELIVERY_LIMIT = 30;
export const alertTabs = ["logs", "channels", "rules"] as const;
export const deliveryStatuses = ["all", "pending", "sent", "failed", "suppressed", "cooldown"] as const;
export const alertEventOptions = [
  { value: "incident_opened", label: "Incident opened" },
  { value: "incident_resolved", label: "Incident resolved" },
] as const;
export const defaultAlertEvents = alertEventOptions.map((event) => event.value);

export const boolLabel = (value?: boolean) => (value ? "yes" : "no");

export const eventLabel = (value?: string) =>
  alertEventOptions.find((event) => event.value === value)?.label ?? value ?? "unknown";

export const getMutationErrorMessage = (error: unknown) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? "Unable to create webhook.");
  }
  return "Unable to create webhook.";
};
