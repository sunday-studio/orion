import { useEffect, useRef, useState } from "react";
import { getApiBase, getToken } from "@/api/client";
import {
  type ApiLogEntry,
  type ApiStatusLogEntry,
  getStreamInternalLogsUrl,
  getStreamLogsUrl,
  getStreamStatusLogsUrl,
} from "@/orion-sdk";
import { type LogRow, serviceRow, statusRow } from "./log-model";
import { type LogKind, type LogQueryState, filterParams } from "./logs-state";

const MAX_ROWS = 300;

export type LiveRow = LogRow & { arrivedAt: number };

/** Reads the log stream with fetch so the auth header can be sent (EventSource cannot). */
export const useLogStream = (
  kind: LogKind,
  state: LogQueryState,
  enabled: boolean,
  paused: boolean,
) => {
  const [rows, setRows] = useState<LiveRow[]>([]);
  const [connected, setConnected] = useState(false);
  const pausedRef = useRef(paused);
  pausedRef.current = paused;
  const params = filterParams(state, kind);
  const { from: _from, to: _to, ...live } = params;
  const key = JSON.stringify(live);

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    let lastId = "";
    setRows([]);

    const run = async () => {
      while (!controller.signal.aborted) {
        try {
          const clean = Object.fromEntries(
            Object.entries(live).filter(([, v]) => v !== undefined),
          ) as Record<string, string>;
          const path =
            kind === "status"
              ? getStreamStatusLogsUrl(clean)
              : kind === "internal"
                ? getStreamInternalLogsUrl(clean)
                : getStreamLogsUrl(clean);
          const token = getToken();
          const response = await fetch(`${getApiBase()}${path}`, {
            signal: controller.signal,
            headers: {
              Accept: "text/event-stream",
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
              ...(lastId ? { "Last-Event-ID": lastId } : {}),
            },
          });
          if (response.status === 401 || response.status === 403) return;
          if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`);
          setConnected(true);
          const reader = response.body.getReader();
          const decoder = new TextDecoder();
          let buffer = "";
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");
            const frames = buffer.split("\n\n");
            buffer = frames.pop() ?? "";
            const incoming: LiveRow[] = [];
            for (const frame of frames) {
              let data = "";
              for (const line of frame.split("\n")) {
                if (line.startsWith("id:")) lastId = line.slice(3).trim();
                if (line.startsWith("data:")) data += line.slice(5).trim();
              }
              if (!data || pausedRef.current) continue;
              try {
                const parsed = JSON.parse(data) as ApiLogEntry & ApiStatusLogEntry;
                const row = kind === "status" ? statusRow(parsed) : serviceRow(parsed);
                incoming.push({ ...row, arrivedAt: Date.now() });
              } catch {
                // Skip frames that are not entries.
              }
            }
            if (incoming.length > 0)
              setRows((current) => [...incoming.reverse(), ...current].slice(0, MAX_ROWS));
          }
        } catch {
          if (controller.signal.aborted) return;
        }
        setConnected(false);
        await new Promise((resolve) => window.setTimeout(resolve, 3000));
      }
    };
    void run();
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, kind, key]);

  return { rows, connected, clear: () => setRows([]) };
};
