// Number and duration formatting for the trace views. Pure, no React, so it runs under `node --test`.

const finite = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);

/** Two significant digits under 10, whole numbers above: 1.2, 9.9, 10, 120. */
const sig = (value: number) => (value < 10 ? Math.round(value * 10) / 10 : Math.round(value));

const UNITS: readonly (readonly [string, number])[] = [
  ["µs", 1e3],
  ["ms", 1e6],
  ["s", 1e9],
];

/** A duration in nanoseconds as people read it: 850ns, 2.5ms, 1.2s, 3m 5s, 1h 2m. "-" when unknown. */
export const formatDurationNs = (ns?: number | null): string => {
  if (!finite(ns) || ns < 0) return "-";
  if (ns < 1000) return `${Math.round(ns)}ns`;
  if (ns >= 60e9) {
    const totalSeconds = Math.round(ns / 1e9);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;
    if (hours > 0) return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
  }
  for (const [unit, scale] of UNITS) {
    const value = sig(ns / scale);
    // 999.6µs rounds to 1000: move up a unit instead of printing "1000µs".
    if (value < 1000 || unit === "s") return `${value}${unit}`;
  }
  return `${sig(ns / 1e9)}s`;
};

/** A position on the timeline: "+12ms" after the start, "-3ms" before it. */
export const formatOffsetNs = (ns?: number | null): string => {
  if (!finite(ns)) return "-";
  return `${ns < 0 ? "-" : "+"}${formatDurationNs(Math.abs(ns))}`;
};

export type AxisTick = { fraction: number; label: string };

/** Evenly spaced labels for the waterfall header, from 0 to the whole trace. */
export const axisTicks = (totalNs: number, steps = 4): AxisTick[] => {
  const count = Math.max(Math.floor(steps), 1);
  const total = finite(totalNs) && totalNs > 0 ? totalNs : 0;
  return Array.from({ length: count + 1 }, (_, index) => {
    const fraction = index / count;
    return { fraction, label: formatDurationNs(total * fraction) };
  });
};

/** 0 to 1 as a percentage: 0%, 1.2%, 12%, 100%. "-" when unknown. */
export const formatPercent = (rate?: number | null): string => {
  if (!finite(rate) || rate < 0) return "-";
  const percent = Math.min(rate, 1) * 100;
  if (percent === 0) return "0%";
  if (percent < 10) return `${Math.round(percent * 10) / 10}%`;
  return `${Math.round(percent)}%`;
};

/** First characters of an id, for rows where the whole 32 digits would not fit. */
export const shortId = (id?: string | null, length = 8): string =>
  id ? (id.length > length ? id.slice(0, length) : id) : "-";

const DURATION_INPUT = /^(\d+(?:\.\d+)?)\s*(ms|s|m|h)?$/i;
const UNIT_MS = { ms: 1, s: 1000, m: 60_000, h: 3_600_000 } as const;

/** A minimum duration the way Core reads it ("250ms", "2s", or plain milliseconds), in milliseconds. */
export const parseDurationMs = (input?: string | null): number | undefined => {
  const match = DURATION_INPUT.exec((input ?? "").trim());
  if (!match) return undefined;
  const unit = (match[2]?.toLowerCase() ?? "ms") as keyof typeof UNIT_MS;
  const value = Number(match[1]) * UNIT_MS[unit];
  return Number.isFinite(value) ? value : undefined;
};

/** The input to send to Core, or "" when it is not a duration Core would accept. */
export const normalizeMinDuration = (input?: string | null): string => {
  const trimmed = (input ?? "").trim().replace(/\s+/g, "");
  return parseDurationMs(trimmed) === undefined ? "" : trimmed;
};

/** "250ms" or "2s" as a label such as "≥ 250ms". */
export const minDurationLabel = (input: string): string => `≥ ${input}`;
