import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  JsonBlock,
  StatusBadge,
} from "@/design-system";
import type { ApiLogEntry } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";

type LogEntryDrawerProps = {
  entry?: ApiLogEntry;
  onOpenChange: (open: boolean) => void;
};

export const logTitle = (entry: ApiLogEntry) =>
  entry.event || (entry.body ?? "").split("\n")[0] || "(empty)";

export const LogEntryDrawer = ({ entry, onOpenChange }: LogEntryDrawerProps) => (
  <Drawer swipeDirection="right" open={Boolean(entry)} onOpenChange={onOpenChange}>
    <DrawerContent>
      <DrawerTitle>{entry ? logTitle(entry) : "Log entry"}</DrawerTitle>
      <DrawerDescription>
        {entry?.source === "agent" ? "Server log" : (entry?.app_name ?? "App log")}
      </DrawerDescription>
      {entry && (
        <div className="space-y-4 text-[13px]">
          <dl className="grid grid-cols-[7rem_1fr] gap-x-3 gap-y-2">
            <dt className="text-quiet">Time</dt>
            <dd>{formatDate(entry.timestamp, DATE_TIME_FORMAT)}</dd>
            <dt className="text-quiet">Severity</dt>
            <dd>
              <StatusBadge status={entry.severity === "warn" ? "degraded" : entry.severity}>
                {entry.severity}
              </StatusBadge>
            </dd>
            {entry.trace_id && (
              <>
                <dt className="text-quiet">Trace</dt>
                <dd className="font-mono text-xs">{entry.trace_id}</dd>
              </>
            )}
            {entry.request_id && (
              <>
                <dt className="text-quiet">Request</dt>
                <dd className="font-mono text-xs">{entry.request_id}</dd>
              </>
            )}
          </dl>
          <JsonBlock value={entry} />
        </div>
      )}
    </DrawerContent>
  </Drawer>
);
