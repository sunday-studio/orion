import { format, isValid, parseISO } from "date-fns";

export const DATE_FORMAT = "MMM d, yyyy";
export const TIME_FORMAT = "HH:mm";
export const DATE_TIME_FORMAT = `${DATE_FORMAT} ${TIME_FORMAT}`;
export const DATE_TIME_SECONDS_FORMAT = `${DATE_TIME_FORMAT}:ss`;
export const ISO_DATE_FORMAT = "yyyy-MM-dd";
export const ISO_DATE_TIME_FORMAT = "yyyy-MM-dd HH:mm";

type DateInput = Date | string | number | null | undefined;

export function formatDate(
  value: DateInput,
  dateFormat = DATE_TIME_FORMAT,
  fallback = "—",
): string {
  const date = toDate(value);
  if (!date || !isValid(date)) return fallback;
  return format(date, dateFormat);
}

function toDate(value: DateInput): Date | null {
  if (value == null || value === "") return null;
  if (value instanceof Date) return value;
  if (typeof value === "string") return parseISO(value);
  return new Date(value);
}
