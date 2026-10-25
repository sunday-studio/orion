import type { ApiAlertDeliveryResponse } from "@/orion-sdk";

export type DeliveryRow = ApiAlertDeliveryResponse;

const EVENT_LABELS: Record<string, string> = {
  incident_opened: "Incident opened",
  incident_resolved: "Incident resolved",
  incident_reopened: "Incident reopened",
  test: "Test",
};

export const deliveryTitle = (delivery: DeliveryRow) =>
  delivery.alert_title ?? EVENT_LABELS[delivery.event_type ?? ""] ?? delivery.event_type ?? "Alert";

export const statusLabel = (status?: string) =>
  status === "failed"
    ? "Failed"
    : status
      ? status.charAt(0).toUpperCase() + status.slice(1)
      : "Unknown";

export const deliveryStatusOptions = [
  { value: "failed", label: "Failed" },
  { value: "sent", label: "Sent" },
  { value: "cooldown", label: "Cooldown" },
  { value: "suppressed", label: "Suppressed" },
  { value: "pending", label: "Pending" },
] as const;

export const sinceOptions = [
  { value: "1h", label: "last hour" },
  { value: "24h", label: "last 24 hours" },
  { value: "7d", label: "last 7 days" },
] as const;

export const healthLabel = (health?: string) =>
  health === "failing" ? "Failing" : health === "healthy" ? "Healthy" : "Untested";

export const channelTypeLabel = (type?: string) =>
  type === "email" ? "Email" : type === "webhook" ? "Webhook" : (type ?? "-");
