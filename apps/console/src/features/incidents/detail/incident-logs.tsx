import type { ApiLogEntry } from "@/orion-sdk";
import { formatDate } from "@/utils/date";

/** Logs from the incident's monitor and server around the time it was open. */
export const IncidentLogs = ({ entries }: { entries: ApiLogEntry[] }) => {
  if (entries.length === 0) {
    return <p className="py-4 text-[13px] text-quiet">No logs recorded around this incident.</p>;
  }
  return (
    <ul className="flex flex-col divide-y divide-line-soft font-mono text-[12px]">
      {entries.map((entry, index) => (
        <li key={entry.id ?? index} className="grid grid-cols-[84px_56px_1fr] gap-3 py-1.5">
          <time className="text-faint">{formatDate(entry.timestamp, "HH:mm:ss")}</time>
          <span className="uppercase text-quiet">{entry.severity ?? "info"}</span>
          <span className="min-w-0 break-words">{entry.body ?? entry.event ?? ""}</span>
        </li>
      ))}
    </ul>
  );
};
