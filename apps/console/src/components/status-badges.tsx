import { cn } from "@/lib/utils";

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

const badgeBaseClass =
  "inline-flex w-fit items-center rounded-sm text-xs leading-5 font-medium px-1";

const severityClass: Record<Severity, string> = {
  low: "bg-teal-100 text-teal-950",
  medium: "bg-amber-100 text-amber-900",
  high: "bg-rose-100 text-rose-900",
  critical: "bg-red-400 text-red-950",
  error: "bg-rose-100 text-rose-900",
  warning: "bg-amber-100 text-amber-900",
  info: "bg-teal-100 text-teal-900",
};

const statusClass: Record<Status, string> = {
  up: "bg-teal-100 text-teal-900",
  down: "bg-rose-100 text-rose-900",
  degraded: "bg-amber-100 text-amber-900",
  maintenance: "bg-cyan-100 text-cyan-900",
  unknown: "bg-neutral-100 text-neutral-900",
  stale: "bg-slate-100 text-slate-900",
  open: "bg-rose-100 text-rose-900",
  acknowledged: "bg-amber-100 text-amber-900",
  resolved: "bg-teal-100 text-teal-900",
};

const notificationClass: Record<NotificationStatus, string> = {
  pending: "bg-neutral-100 text-neutral-900",
  sent: "bg-teal-100 text-teal-900",
  failed: "bg-rose-100 text-rose-900",
  suppressed: "bg-slate-100 text-slate-900",
  cooldown: "bg-cyan-100 text-cyan-900",
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

export const SeverityBadge = ({ value, fallback = "unknown", className }: BadgeProps<Severity>) => (
  <span className={cn(badgeBaseClass, value ? severityClass[value] : severityClass.low, className)}>
    {label(value, fallback)}
  </span>
);

export const StatusBadge = ({ value, fallback = "unknown", className }: BadgeProps<Status>) => (
  <span className={cn(badgeBaseClass, value ? statusClass[value] : statusClass.unknown, className)}>
    {label(value, fallback)}
  </span>
);

export const NotificationBadge = ({
  value,
  fallback = "unknown",
  className,
}: BadgeProps<NotificationStatus>) => (
  <span
    className={cn(
      badgeBaseClass,
      value ? notificationClass[value] : notificationClass.pending,
      className,
    )}
  >
    {label(value, fallback)}
  </span>
);
