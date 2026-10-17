import { cn } from "@/utils/cn";

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

const badgeBaseClass = "inline-flex w-fit items-center gap-1.5 rounded px-1.5 text-xs leading-5";

const severityClass: Record<Severity, string> = {
  low: "bg-[#ccfbf1] text-[#115e59]",
  medium: "bg-[#fef9c3] text-[#854d0e]",
  high: "bg-[#ffe4e6] text-[#9f1239]",
  critical: "bg-[#f43f5e] text-white",
  error: "bg-rose-100 text-rose-900",
  warning: "bg-amber-100 text-amber-900",
  info: "bg-teal-100 text-teal-900",
};

const statusClass: Record<Status, string> = {
  up: "bg-[#ccfbf1] text-[#115e59]",
  down: "bg-[#ffe4e6] text-[#9f1239]",
  degraded: "bg-[#fef3c7] text-[#92400e]",
  maintenance: "bg-[#ede9fe] text-[#5b21b6]",
  unknown: "bg-[#f5f5f4] text-[#44403c]",
  stale: "bg-[#f1f5f9] text-[#334155]",
  open: "bg-[#ffe4e6] text-[#9f1239]",
  acknowledged: "bg-[#fef3c7] text-[#92400e]",
  covered: "bg-[#cffafe] text-[#155e75]",
  resolved: "bg-[#dbeafe] text-[#1e40af]",
};

const notificationClass: Record<NotificationStatus, string> = {
  pending: "bg-[#f5f5f4] text-[#44403c]",
  sent: "bg-[#ccfbf1] text-[#115e59]",
  failed: "bg-[#ffe4e6] text-[#9f1239]",
  suppressed: "bg-[#f1f5f9] text-[#334155]",
  cooldown: "bg-[#cffafe] text-[#155e75]",
};

type Shape = "dot" | "square" | "hollow";

const shapeFor = (value?: string): Shape => {
  if (["down", "open", "failed", "critical", "error"].includes(value ?? "")) return "square";
  if (["unknown", "pending", "stale", "suppressed"].includes(value ?? "")) return "hollow";
  return "dot";
};

const BadgeShape = ({ value }: { value?: string }) => {
  const shape = shapeFor(value);
  return (
    <span
      aria-hidden
      className={cn(
        "size-1.5 shrink-0 bg-current",
        shape === "dot" && "rounded-full",
        shape === "hollow" && "rounded-full bg-transparent ring-1 ring-current",
      )}
    />
  );
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
    <BadgeShape value={value} />
    {label(value, fallback)}
  </span>
);

export const StatusBadge = ({ value, fallback = "unknown", className }: BadgeProps<Status>) => (
  <span className={cn(badgeBaseClass, value ? statusClass[value] : statusClass.unknown, className)}>
    <BadgeShape value={value} />
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
    <BadgeShape value={value} />
    {label(value, fallback)}
  </span>
);
