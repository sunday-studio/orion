// Span attributes, events and links as rows for the inspector. Pure, no React.
import type { ApiTraceSpan, ApiTraceSpanEvent } from "../../orion-sdk/index.ts";

export type AttrRow = {
  key: string;
  text: string;
  /** A short scalar: worth a "find traces with this value" link. */
  filterable: boolean;
};

const MAX_FILTERABLE = 200;

const asText = (value: unknown): string => {
  if (value === null) return "null";
  if (value === undefined) return "";
  if (typeof value === "string") return value === "" ? '""' : value;
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
};

/** Attributes sorted by key. Anything that is not a plain object gives no rows. */
export const attributeRows = (attributes: unknown): AttrRow[] => {
  if (!attributes || typeof attributes !== "object" || Array.isArray(attributes)) return [];
  return Object.entries(attributes as Record<string, unknown>)
    .map(([key, value]) => {
      const text = asText(value);
      const scalar = value !== null && typeof value !== "object" && value !== "";
      return {
        key,
        text,
        filterable: scalar && text.length <= MAX_FILTERABLE && !text.includes("\n"),
      };
    })
    .sort((a, b) => a.key.localeCompare(b.key));
};

export const serviceLabel = (span: ApiTraceSpan): string =>
  span.service_name || span.app_name || "unknown service";

export const KIND_LABEL: Record<string, string> = {
  internal: "Internal",
  server: "Server",
  client: "Client",
  producer: "Producer",
  consumer: "Consumer",
};

/** "Server", or an empty string for an unspecified kind. */
export const kindLabel = (kind?: string): string => KIND_LABEL[kind ?? ""] ?? "";

/** Where an event sits inside its span, in nanoseconds (never negative). */
export const eventOffsetInSpan = (
  event: ApiTraceSpanEvent,
  spanStartNs: number | undefined,
): number | undefined => {
  if (typeof event.offset_ns !== "number" || !Number.isFinite(event.offset_ns)) return undefined;
  return Math.max(event.offset_ns - (spanStartNs ?? 0), 0);
};

/** Events in time order. Exporters do not promise any. */
export const sortedEvents = (events?: readonly ApiTraceSpanEvent[]): ApiTraceSpanEvent[] =>
  [...(events ?? [])].sort(
    (a, b) => (a.offset_ns ?? Number.POSITIVE_INFINITY) - (b.offset_ns ?? Number.POSITIVE_INFINITY),
  );

/** The exception an instrumented library recorded as a span event, if any: type and message. */
export const exceptionOf = (
  span: ApiTraceSpan,
): { type?: string; message?: string; stack?: string } | undefined => {
  const event = (span.events ?? []).find((item) => item.name === "exception");
  const attributes = event?.attributes as Record<string, unknown> | undefined;
  if (!attributes) return undefined;
  const text = (key: string) =>
    typeof attributes[key] === "string" ? (attributes[key] as string) : undefined;
  const found = {
    type: text("exception.type"),
    message: text("exception.message"),
    stack: text("exception.stacktrace"),
  };
  return found.type || found.message || found.stack ? found : undefined;
};
