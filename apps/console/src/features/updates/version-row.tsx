import { useState } from "react";
import { Badge, Button, RaisedCard } from "@/design-system";
import { formatDate } from "@/utils/date";
import { SettingRow } from "../settings/components-ui";
import { CoreUpdateDialog } from "./core-update-dialog";
import { pendingUpdate, stateBadge } from "./update-model";
import type { ApiUpdateSettings } from "./use-updates";

/** The running version, and a way in when a newer release exists. */
export const VersionRows = ({
  updates,
  version,
  bare = false,
}: {
  updates?: ApiUpdateSettings;
  version?: string;
  /** Skip the raised card, for use inside another list. */
  bare?: boolean;
}) => {
  const [open, setOpen] = useState(false);
  const pending = pendingUpdate(updates);
  const badge = stateBadge(updates);
  const current = version ?? updates?.current_version;
  const row = (
    <SettingRow
      label={`Version ${current ?? ""}`}
      detail={
        pending
          ? undefined
          : updates?.state === "error"
            ? (updates.error ?? "Could not check")
            : undefined
      }
    >
      <span className="flex items-center gap-2">
        {updates && (
          <Badge tone={badge.tone}>
            {pending && updates.released_at
              ? `${badge.text}, ${formatDate(updates.released_at, "MMM d")}`
              : badge.text}
          </Badge>
        )}
        {pending && (
          <Button size="sm" onClick={() => setOpen(true)}>
            View update
          </Button>
        )}
      </span>
    </SettingRow>
  );
  return (
    <>
      {bare ? row : <RaisedCard>{row}</RaisedCard>}
      {pending && updates && (
        <CoreUpdateDialog open={open} onOpenChange={setOpen} updates={updates} />
      )}
    </>
  );
};
