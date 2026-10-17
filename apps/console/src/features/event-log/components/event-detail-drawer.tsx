import { Drawer, DrawerContent, DrawerDescription, DrawerTitle } from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type { ApiOrionEventResponse } from "@/orion-sdk";

type EventDetailDrawerProps = {
  event?: ApiOrionEventResponse;
  onOpenChange: (open: boolean) => void;
};

const formatValue = (value: unknown) =>
  value === null || value === undefined || value === ""
    ? "-"
    : typeof value === "string"
      ? value
      : JSON.stringify(value);

export const EventDetailDrawer = ({ event, onOpenChange }: EventDetailDrawerProps) => (
  <Drawer swipeDirection="right" open={Boolean(event)} onOpenChange={onOpenChange}>
    <DrawerContent>
      <DrawerTitle>{event?.type ?? "Event"}</DrawerTitle>
      <DrawerDescription>Operational event detail</DrawerDescription>
      {event && (
        <div className="space-y-4 text-[13px]">
          <dl className="grid grid-cols-[7rem_1fr] gap-x-3 gap-y-2">
            <dt className="text-quiet">Time</dt>
            <dd>{formatDate(event.created_at, DATE_TIME_FORMAT)}</dd>
            <dt className="text-quiet">Source</dt>
            <dd>{formatValue(event.source)}</dd>
            <dt className="text-quiet">Message</dt>
            <dd className="break-words">{formatValue(event.message)}</dd>
            <dt className="text-quiet">Event ID</dt>
            <dd className="font-mono text-xs text-faint">{formatValue(event.id)}</dd>
          </dl>
          <pre
            aria-label="Raw event"
            className="max-h-96 overflow-auto rounded-lg bg-field p-3 font-mono text-xs"
          >
            {JSON.stringify(event, null, 2)}
          </pre>
        </div>
      )}
    </DrawerContent>
  </Drawer>
);
