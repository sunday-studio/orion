import { format, isValid, parseISO } from "date-fns";

export const DATE_FORMAT = "MMM d, yyyy";
export const TIME_FORMAT = "HH:mm";
export const DATE_TIME_FORMAT = `${DATE_FORMAT} ${TIME_FORMAT}`;
export const DATE_TIME_SECONDS_FORMAT = `${DATE_TIME_FORMAT}:ss`;
export const ISO_DATE_FORMAT = "yyyy-MM-dd";
export const ISO_DATE_TIME_FORMAT = "yyyy-MM-dd HH:mm";

type DateInput = Date | string | number | null | undefined;

const TIMEZONE_KEY = "orion.timezone";
export const DEFAULT_TIMEZONE = "UTC";

/** The display timezone: "UTC" by default, "local" for the browser zone, or any IANA name. */
export function getTimezone(): string {
  try {
    return localStorage.getItem(TIMEZONE_KEY) || DEFAULT_TIMEZONE;
  } catch {
    return DEFAULT_TIMEZONE;
  }
}

export function setTimezone(timezone: string): void {
  try {
    localStorage.setItem(TIMEZONE_KEY, timezone);
  } catch {
    // The choice applies until the page is reloaded.
  }
}

/** Shift a date so its local getters read as wall-clock time in the chosen timezone. */
export function toZoned(date: Date, timezone = getTimezone()): Date {
  if (timezone === "local") return date;
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: timezone,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    }).formatToParts(date);
    const get = (type: string) => Number(parts.find((part) => part.type === type)?.value ?? 0);
    return new Date(
      get("year"),
      get("month") - 1,
      get("day"),
      get("hour"),
      get("minute"),
      get("second"),
      date.getMilliseconds(),
    );
  } catch {
    return date;
  }
}

export function formatDate(
  value: DateInput,
  dateFormat = DATE_TIME_FORMAT,
  fallback = "—",
): string {
  const date = toDate(value);
  if (!date || !isValid(date)) return fallback;
  return format(toZoned(date), dateFormat);
}

function toDate(value: DateInput): Date | null {
  if (value == null || value === "") return null;
  if (value instanceof Date) return value;
  if (typeof value === "string") return parseISO(value);
  return new Date(value);
}
