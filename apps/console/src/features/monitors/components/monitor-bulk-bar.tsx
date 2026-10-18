import { Button, Callout, FloatingBar } from "@/design-system";
import { useState } from "react";

/** Core's POST /v1/monitors/bulk is proposed (F4); flip when it lands. */
export const MONITOR_BULK_API_AVAILABLE = false;

type Action = "pause" | "maintenance" | "clone" | "delete";

export const MonitorBulkBar = ({
  ids,
  onClear,
}: {
  ids: readonly string[];
  onClear: () => void;
}) => {
  const [notice, setNotice] = useState(false);
  const run = (_action: Action) => {
    if (!MONITOR_BULK_API_AVAILABLE) setNotice(true);
  };
  return (
    <FloatingBar aria-label="Bulk actions">
      {notice && (
        <Callout tone="warn" className="mr-2 py-1">
          Bulk actions need a Core update that is not available yet.
        </Callout>
      )}
      <span className="pr-1 font-medium">{ids.length} selected</span>
      <span aria-hidden className="mx-1 h-5 w-px bg-hairline" />
      <Button size="sm" variant="secondary" onClick={() => run("pause")}>
        Pause
      </Button>
      <Button size="sm" variant="secondary" onClick={() => run("maintenance")}>
        Maintenance
      </Button>
      <Button size="sm" variant="secondary" onClick={() => run("clone")}>
        Clone
      </Button>
      <Button size="sm" variant="destructive" onClick={() => run("delete")}>
        Delete
      </Button>
      <Button size="sm" variant="ghost" onClick={onClear}>
        Clear
      </Button>
    </FloatingBar>
  );
};
