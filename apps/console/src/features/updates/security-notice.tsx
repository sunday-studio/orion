import { useState } from "react";
import { Button, Callout } from "@/design-system";
import { CoreUpdateDialog } from "./core-update-dialog";
import { useUpdates } from "./use-updates";

/** On the Overview only when the newest release fixes a security issue. */
export const SecurityUpdateNotice = ({ className }: { className?: string }) => {
  const updates = useUpdates().data;
  const [open, setOpen] = useState(false);
  if (updates?.state !== "security") return null;
  return (
    <>
      <Callout
        tone="down"
        lead="Security update"
        className={`flex items-center justify-between gap-3 ${className ?? ""}`}
      >
        <span className="mr-auto">Core {updates.latest_version} fixes a security issue</span>
        <Button size="sm" onClick={() => setOpen(true)}>
          View update
        </Button>
      </Callout>
      <CoreUpdateDialog open={open} onOpenChange={setOpen} updates={updates} />
    </>
  );
};
