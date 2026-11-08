import { useState } from "react";
import { Button, Popover, PopoverContent, PopoverTrigger } from "@/design-system";
import { CoreUpdateDialog } from "@/features/updates/core-update-dialog";
import { pendingUpdate } from "@/features/updates/update-model";
import { useUpdates } from "@/features/updates/use-updates";
import { formatDate } from "@/utils/date";

/** "Update available" above the issue count. Opens a short note and the update dialog. */
export const UpdatePill = () => {
  const updates = useUpdates().data;
  const pending = pendingUpdate(updates);
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState(false);
  if (!pending || !updates) return null;
  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger className="mb-[3px] flex h-[26px] w-full cursor-pointer items-center gap-2 rounded-lg bg-(--tone-info-bg) px-[9px] text-left text-[12.5px] font-medium text-(--tone-info-fg) outline-none focus-visible:ring-[3px] focus-visible:ring-focus">
          <i aria-hidden="true" className="block size-[7px] rounded-full bg-current" />
          {pending.state === "security" ? "Security update" : "Update available"}
        </PopoverTrigger>
        <PopoverContent side="right" align="end" className="grid w-64 gap-1.5">
          <b className="font-semibold">{pending.latest_version} is available</b>
          <span className="text-[12.5px] leading-normal text-quiet">
            {pending.released_at && `Released ${formatDate(pending.released_at, "MMM d")}. `}You are
            on {pending.current_version}.
          </span>
          <div className="mt-1.5 flex gap-2">
            <Button
              size="sm"
              onClick={() => {
                setOpen(false);
                setDialog(true);
              }}
            >
              View update
            </Button>
            {pending.release_url && (
              <a href={pending.release_url} target="_blank" rel="noreferrer">
                <Button size="sm" variant="secondary">
                  Release notes
                </Button>
              </a>
            )}
          </div>
        </PopoverContent>
      </Popover>
      <CoreUpdateDialog open={dialog} onOpenChange={setDialog} updates={updates} />
    </>
  );
};
