import { StatusBadge as DsStatusBadge } from "@/design-system";

export const severityValues = [
  "low",
  "medium",
  "high",
  "critical",
  "error",
  "warning",
  "info",
] as const;
export type Severity = (typeof severityValues)[number];

export const statusValues = [
  "up",
  "down",
  "degraded",
  "maintenance",
  "unknown",
  "stale",
  "open",
  "acknowledged",
  "covered",
  "resolved",
] as const;
export type Status = (typeof statusValues)[number];

export const notificationValues = ["pending", "sent", "failed", "suppressed", "cooldown"] as const;
export type NotificationStatus = (typeof notificationValues)[number];

type BadgeProps<TValue extends string> = {
  value?: TValue;
  fallback?: string;
  className?: string;
};

const normalize = (value?: string | null) => value?.trim().toLowerCase();

const includesValue = <TValue extends string>(
  values: readonly TValue[],
  value?: string | null,
): value is TValue => {
  const normalized = normalize(value);
  return values.includes(normalized as TValue);
};

export const toSeverity = (value?: string | null): Severity | undefined =>
  includesValue(severityValues, value) ? (normalize(value) as Severity) : undefined;

export const toStatus = (value?: string | null): Status | undefined =>
  includesValue(statusValues, value) ? (normalize(value) as Status) : undefined;

export const toNotificationStatus = (value?: string | null): NotificationStatus | undefined =>
  includesValue(notificationValues, value) ? (normalize(value) as NotificationStatus) : undefined;

const label = (value?: string, fallback = "unknown") => value ?? fallback;

/** Legacy adapters; each renders the design-system StatusBadge (label plus glyph shape). */
export const SeverityBadge = ({ value, fallback = "unknown", className }: BadgeProps<Severity>) => (
  <DsStatusBadge status={value ?? "low"} className={className}>
    {label(value, fallback)}
  </DsStatusBadge>
);

export const StatusBadge = ({ value, fallback = "unknown", className }: BadgeProps<Status>) => (
  <DsStatusBadge status={value ?? "unknown"} className={className}>
    {label(value, fallback)}
  </DsStatusBadge>
);

export const NotificationBadge = ({
  value,
  fallback = "unknown",
  className,
}: BadgeProps<NotificationStatus>) => (
  <DsStatusBadge status={value ?? "pending"} className={className}>
    {label(value, fallback)}
  </DsStatusBadge>
);
