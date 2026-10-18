import { useState } from "react";
import { Button, Input } from "@/design-system";
import type { ApiIncidentTimelineItemResponse } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import type { IncidentTimelineItemWithMeta } from "../components/incident-detail-utils";

const Avatar = () => (
  <span className="inline-block size-5 flex-none rounded-full bg-[conic-gradient(from_40deg,#6366f1,#14b8a6,#f59e0b,#6366f1)]" />
);

const dotColor = (item: IncidentTimelineItemWithMeta) => {
  if (item.status === "failed" || item.status === "error") return "#f59e0b";
  if (item.type?.includes("open") || item.type?.includes("trigger")) return "#e5484d";
  if (item.type?.includes("resolve")) return "#2a9f69";
  return "#a1a1aa";
};

const isNote = (item: IncidentTimelineItemWithMeta) =>
  item.type === "incident_note" || Boolean(item.note);
const noteText = (item: IncidentTimelineItemWithMeta) => item.note ?? item.message;

export const NoteComposer = ({
  pending,
  failed,
  onPost,
}: {
  pending: boolean;
  failed: boolean;
  onPost: (text: string, done: () => void) => void;
}) => {
  const [draft, setDraft] = useState("");
  const submit = () => draft.trim() && onPost(draft.trim(), () => setDraft(""));
  return (
    <div className="flex flex-col gap-1.5">
      <form
        className="flex items-center gap-2.5"
        onSubmit={(event) => {
          event.preventDefault();
          submit();
        }}
      >
        <Avatar />
        <Input
          aria-label="Note"
          placeholder="Leave a note or a link"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
        />
        <Button type="submit" size="sm" variant="secondary" disabled={!draft.trim() || pending}>
          Post
        </Button>
      </form>
      {failed && (
        <p role="alert" className="pl-[30px] text-xs text-[#be123c]">
          Unable to post the note. Try again.
        </p>
      )}
    </div>
  );
};

type Props = {
  items: ApiIncidentTimelineItemResponse[];
  retrying?: boolean;
  onRetry: (deliveryId: string) => void;
  onOpen: (item: ApiIncidentTimelineItemResponse) => void;
};

/** Vertical event feed (canvas `.tlx`), newest first. */
export const IncidentTimeline = ({ items, retrying, onRetry, onOpen }: Props) => {
  const sorted = [...items].sort(
    (a, b) => new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime(),
  );
  if (sorted.length === 0) {
    return <p className="py-4 text-[13px] text-quiet">No timeline events recorded.</p>;
  }
  return (
    <ol className="flex flex-col pl-1">
      {sorted.map((raw, index) => {
        const item = raw as IncidentTimelineItemWithMeta;
        const last = index === sorted.length - 1;
        return (
          <li
            key={item.id ?? index}
            className="relative grid grid-cols-[22px_1fr_auto] items-start gap-2.5 py-2 text-[13px]"
          >
            {!last && (
              <span
                aria-hidden
                className="absolute bottom-[-8px] left-2.5 top-7 w-px bg-line-soft"
              />
            )}
            <span className="relative z-[1] flex size-[22px] items-center justify-center rounded-full bg-chip dark:bg-tray">
              {isNote(item) ? (
                <Avatar />
              ) : (
                <span className="size-2 rounded-full" style={{ background: dotColor(item) }} />
              )}
            </span>
            <div className="min-w-0">
              {isNote(item) ? (
                <>
                  <b className="font-semibold">{item.actor_id ?? item.actor_type ?? "Someone"}</b>{" "}
                  added a note
                  <div className="mt-1 rounded-lg border border-line-soft bg-sub px-2.5 py-2 text-[12.5px] text-mid">
                    {noteText(item)}
                  </div>
                </>
              ) : (
                <button
                  type="button"
                  className="text-left"
                  onClick={() => onOpen(item)}
                  disabled={!item.monitor_report_id && !item.alert_delivery_id}
                >
                  {item.message ?? item.type ?? "Event"}
                </button>
              )}
              {item.status === "failed" && item.alert_delivery_id && (
                <>
                  {" "}
                  &middot;{" "}
                  <button
                    type="button"
                    className="underline"
                    disabled={retrying}
                    onClick={() => onRetry(item.alert_delivery_id ?? "")}
                  >
                    Retry
                  </button>
                </>
              )}
            </div>
            <time className="whitespace-nowrap text-xs text-faint">
              {formatDate(item.created_at, TIME_FORMAT)}
            </time>
          </li>
        );
      })}
    </ol>
  );
};
