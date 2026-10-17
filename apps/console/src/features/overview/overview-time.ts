import { useEffect, useState } from "react";
import { getTimezone, toZoned } from "@/utils/date";

/** Current time, refreshed on an interval so "open for 30m 12s" keeps counting. */
export const useNow = (intervalMs = 1000) => {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
};

/** "30m 12s", "1h 12m", "16h 18m", "2d 4h". */
export const formatSpan = (fromMs: number, toMs: number, withSeconds = false, compact = false) => {
  const total = Math.max(0, Math.floor((toMs - fromMs) / 1000));
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (withSeconds) return `${minutes}m ${seconds}s`;
  if (compact) return `${minutes}m`;
  return `${Math.max(minutes, total > 0 ? 1 : 0)} min`;
};

const pad = (value: number) => String(value).padStart(2, "0");

/** "14:02" for today, "Yesterday", otherwise "Mon 09:10". */
export const formatWhen = (iso: string | undefined, now: number) => {
  if (!iso) return "-";
  const date = toZoned(new Date(iso));
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  const startOfToday = toZoned(new Date(now));
  startOfToday.setHours(0, 0, 0, 0);
  const dayMs = 86400000;
  if (date.getTime() >= startOfToday.getTime()) return time;
  if (date.getTime() >= startOfToday.getTime() - dayMs) return "Yesterday";
  const weekday = date.toLocaleDateString(undefined, { weekday: "short" });
  return `${weekday} ${time}`;
};

const zoneLabel = () => {
  const zone = getTimezone();
  return zone === "local" ? "local time" : zone;
};

export const formatHeaderDate = (now: number) => {
  const date = toZoned(new Date(now));
  const day = date.toLocaleDateString("en-US", {
    weekday: "long",
    month: "short",
    day: "numeric",
  });
  return `${day} · ${pad(date.getHours())}:${pad(date.getMinutes())} ${zoneLabel()}`;
};
