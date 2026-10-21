import type { ServiceAppResourceSeries } from "@/orion-sdk";

/** Kinds that appear in the Top apps table. */
const TOP_KINDS = ["process", "unit", "container"];

export type Sort = "cpu" | "memory";

const num = (value?: number) =>
  typeof value === "number" && Number.isFinite(value) ? value : undefined;

export const bytes = (value?: number) => {
  const v = num(value);
  if (v === undefined) return undefined;
  if (v >= 1024 ** 3) return `${(v / 1024 ** 3).toFixed(1)} GB`;
  if (v >= 1024 ** 2) return `${Math.round(v / 1024 ** 2)} MB`;
  if (v >= 1024) return `${Math.round(v / 1024)} KB`;
  return `${Math.round(v)} B`;
};

/** "read / write", or a dash when neither was measured. */
export const pair = (a?: number, b?: number) =>
  num(a) === undefined && num(b) === undefined
    ? undefined
    : `${bytes(a) ?? "-"} / ${bytes(b) ?? "-"}`;

export const cpu = (value?: number) =>
  num(value) === undefined ? undefined : `${value?.toFixed(1)}%`;

/** CPU is a share of one core, so it can pass 100%; the bar stops at 120%. */
export const cpuWidth = (value?: number) =>
  num(value) === undefined ? 0 : Math.min(((value ?? 0) / 120) * 100, 100);

const sortKey = (series: ServiceAppResourceSeries, sort: Sort) =>
  num(sort === "cpu" ? series.cpu_percent : series.memory_bytes) ?? -1;

export const partition = (series: ServiceAppResourceSeries[], sort: Sort) => {
  const top = series
    .filter((item) => TOP_KINDS.includes(item.kind ?? ""))
    .sort((a, b) => sortKey(b, sort) - sortKey(a, sort));
  return {
    top,
    paths: series.filter((item) => item.kind === "path"),
    volumes: series.filter((item) => item.kind === "volume"),
    docker: series.filter((item) => item.kind === "docker"),
  };
};

export const dockerTitle = (key?: string) =>
  key === "docker:images"
    ? "Images"
    : key === "docker:build-cache"
      ? "Build cache"
      : (key ?? "Docker");

/** A report is stale after three intervals without a new one. */
export const isStale = (reportedAt?: string, intervalSeconds?: number) => {
  if (!reportedAt) return false;
  const interval = intervalSeconds && intervalSeconds > 0 ? intervalSeconds : 60;
  const limit = Math.max(interval * 3, 180) * 1000;
  return Date.now() - new Date(reportedAt).getTime() > limit;
};

export const minutesAgo = (iso?: string) =>
  iso ? Math.max(Math.round((Date.now() - new Date(iso).getTime()) / 60000), 0) : 0;
