import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
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
  <Dialog open={Boolean(event)} onOpenChange={onOpenChange}>
    <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{event?.type ?? "Event"}</DialogTitle>
      </DialogHeader>
      {event && (
        <div className="space-y-4 text-sm">
          <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2">
            <dt className="text-neutral-600">Time</dt>
            <dd>{formatDate(event.created_at, DATE_TIME_FORMAT)}</dd>
            <dt className="text-neutral-600">Source</dt>
            <dd>{formatValue(event.source)}</dd>
            <dt className="text-neutral-600">Message</dt>
            <dd className="break-words">{formatValue(event.message)}</dd>
            <dt className="text-neutral-600">Event ID</dt>
            <dd className="font-mono text-xs">{formatValue(event.id)}</dd>
          </dl>
          <pre
            aria-label="Raw event"
            className="max-h-72 overflow-auto bg-neutral-100 p-3 font-mono text-xs"
          >
            {JSON.stringify(event, null, 2)}
          </pre>
        </div>
      )}
    </DialogContent>
  </Dialog>
);
