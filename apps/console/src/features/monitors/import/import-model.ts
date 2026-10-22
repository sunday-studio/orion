import type { ServiceCoreManagedMonitorCreateRequest } from "@/orion-sdk";

/** One row of an import, ready to create or with the reason it cannot be. */
export type ImportRow = {
  index: number;
  name: string;
  kind: string;
  target: string;
  intervalSeconds: number;
  request?: ServiceCoreManagedMonitorCreateRequest;
  error?: string;
};

export const MAX_IMPORT = 200;

export const EXAMPLE = JSON.stringify(
  {
    monitors: [
      { name: "Marketing site", url: "https://example.com" },
      { name: "API health", url: "https://api.example.com/health", interval_seconds: 30 },
      { name: "Database", kind: "tcp", host: "db.example.com", port: 5432 },
    ],
  },
  null,
  2,
);

const asInt = (value: unknown, fallback: number) => {
  if (value === undefined || value === null || value === "") return fallback;
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : Number.NaN;
};

const normalizeUrl = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
  try {
    const url = new URL(withScheme);
    if (!["http:", "https:"].includes(url.protocol) || !url.hostname.includes("."))
      return undefined;
    return url.toString();
  } catch {
    return undefined;
  }
};

const hostnameOf = (url: string) => {
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
};

const rowFromEntry = (entry: unknown, index: number): ImportRow => {
  const bad = (error: string, partial: Partial<ImportRow> = {}): ImportRow => ({
    index,
    name: "",
    kind: "http",
    target: "",
    intervalSeconds: 60,
    ...partial,
    error,
  });
  // A bare string is a website address.
  const item: Record<string, unknown> =
    typeof entry === "string"
      ? { url: entry }
      : entry && typeof entry === "object" && !Array.isArray(entry)
        ? (entry as Record<string, unknown>)
        : {};
  if (typeof entry !== "string" && Object.keys(item).length === 0)
    return bad("Expected a URL or an object.");

  const kind = String(item.kind ?? item.type ?? "http").toLowerCase();
  const interval = asInt(item.interval_seconds, 60);
  const timeout = asInt(item.timeout_seconds, 10);
  const failAfter = asInt(item.confirmation_check_count ?? item.fail_after, 3);
  if (Number.isNaN(interval) || Number.isNaN(timeout) || Number.isNaN(failAfter))
    return bad("Intervals, timeouts and fail_after must be whole numbers above zero.");

  const common = {
    confirmation_check_count: failAfter,
    interval_seconds: interval,
    timeout_seconds: timeout,
    ...(typeof item.description === "string" ? { description: item.description } : {}),
    ...(typeof item.runbook_url === "string" ? { runbook_url: item.runbook_url } : {}),
    ...(item.paused === true ? { paused: true } : {}),
  };

  if (kind === "http") {
    const url = normalizeUrl(String(item.url ?? ""));
    if (!url) return bad("Needs a valid website address in url.", { kind });
    const name = String(item.name ?? "").trim() || hostnameOf(url);
    const status = asInt(item.expected_status, 200);
    if (Number.isNaN(status)) return bad("expected_status must be a whole number.", { name, kind });
    return {
      index,
      name,
      kind,
      target: url,
      intervalSeconds: interval,
      request: {
        ...common,
        kind,
        type: kind,
        name,
        config: { url, expected_status: status },
      },
    };
  }
  if (kind === "tcp") {
    const host = String(item.host ?? "").trim();
    const port = asInt(item.port, Number.NaN);
    if (!host) return bad("Needs a host.", { kind });
    if (Number.isNaN(port) || port > 65535)
      return bad("Needs a port from 1 to 65535.", { kind, target: host });
    const name = String(item.name ?? "").trim() || `${host}:${port}`;
    return {
      index,
      name,
      kind,
      target: `${host}:${port}`,
      intervalSeconds: interval,
      request: { ...common, kind, type: kind, name, config: { host, port } },
    };
  }
  return bad(`Kind "${kind}" is not supported in imports. Use http or tcp.`, { kind });
};

export type ParseResult = { rows: ImportRow[]; fatal?: string };

/** Accepts an array, or an object with a monitors array. Entries are URLs or objects. */
export const parseImport = (text: string): ParseResult => {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch (failure) {
    return { rows: [], fatal: `That is not valid JSON. ${(failure as Error).message}` };
  }
  const list = Array.isArray(data)
    ? data
    : data && typeof data === "object" && Array.isArray((data as { monitors?: unknown }).monitors)
      ? (data as { monitors: unknown[] }).monitors
      : undefined;
  if (!list) return { rows: [], fatal: 'Expected a list, or an object like {"monitors": [...]}.' };
  if (list.length === 0) return { rows: [], fatal: "The list is empty." };
  if (list.length > MAX_IMPORT)
    return { rows: [], fatal: `Import at most ${MAX_IMPORT} monitors at a time.` };
  return { rows: list.map(rowFromEntry) };
};
