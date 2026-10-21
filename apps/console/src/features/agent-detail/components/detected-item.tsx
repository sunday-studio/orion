import { MoreHorizontal } from "lucide-react";
import { Checkbox, Menu, MenuContent, MenuItem, MenuTrigger, StatusBadge } from "@/design-system";
import type { ServiceAgentDiscoveryItem } from "@/orion-sdk";
import { confidenceLabel, managedLabel } from "../detected-model";

/** One single-line suggestion: checkbox, name and detail, status and confidence, actions. */
export const DetectedItem = ({
  item,
  selectable,
  checked,
  canAct,
  pending,
  size,
  waitingSize,
  onViewApps,
  onToggle,
  onAction,
}: {
  item: ServiceAgentDiscoveryItem;
  selectable: boolean;
  checked: boolean;
  /** The agent can take requests from the Console. */
  canAct: boolean;
  /** A request for this row is waiting for the agent. */
  pending?: string;
  /** Measured size of an accepted storage row. */
  size?: string;
  waitingSize?: boolean;
  onViewApps?: () => void;
  onToggle: () => void;
  onAction: (type: "dismiss" | "restore" | "ignore") => void;
}) => {
  const monitored = item.status === "monitored";
  return (
    <div className="grid min-h-[46px] grid-cols-[20px_minmax(0,1fr)_230px_40px] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0">
      {selectable ? (
        <Checkbox aria-label={`Select ${item.name}`} checked={checked} onCheckedChange={onToggle} />
      ) : (
        <span />
      )}
      <span className="flex min-w-0 items-center gap-2.5">
        <b className="shrink-0 font-medium">{item.name}</b>
        {item.new && <StatusBadge status="info">New</StatusBadge>}
        {(item.detail || (monitored && item.kind === "storage")) && (
          <span className="min-w-0 truncate text-xs text-faint">
            {monitored && item.kind === "storage"
              ? waitingSize
                ? "Sizes can take a few minutes"
                : "Tracking"
              : item.detail}
          </span>
        )}
      </span>
      <span className="flex items-center justify-end gap-2">
        {pending ? (
          <StatusBadge status="degraded">{pending}</StatusBadge>
        ) : monitored ? (
          <>
            {size && <span className="tabular-nums">{size}</span>}
            {waitingSize && <StatusBadge status="degraded">Waiting for first size</StatusBadge>}
            <StatusBadge status="up">{managedLabel(item.managed_in)}</StatusBadge>
          </>
        ) : item.dismissed ? (
          <StatusBadge status="stale">Dismissed</StatusBadge>
        ) : null}
        {!monitored && (
          <span className="w-14 text-right text-[12.5px] text-faint">
            {confidenceLabel(item.confidence)}
          </span>
        )}
      </span>
      {!monitored ? (
        <Menu>
          <MenuTrigger
            aria-label={`Actions for ${item.name}`}
            disabled={!canAct}
            className="flex size-7 items-center justify-center rounded-md border border-input-border bg-card text-mid outline-none hover:bg-sub focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
          >
            <MoreHorizontal className="size-4" />
          </MenuTrigger>
          <MenuContent>
            {item.dismissed ? (
              <MenuItem onClick={() => onAction("restore")}>Restore</MenuItem>
            ) : (
              <>
                <MenuItem onClick={() => onAction("dismiss")}>Dismiss</MenuItem>
                <MenuItem onClick={() => onAction("ignore")}>Never suggest</MenuItem>
              </>
            )}
          </MenuContent>
        </Menu>
      ) : size && onViewApps ? (
        <button
          type="button"
          className="text-left text-[13px] text-brand hover:underline"
          onClick={onViewApps}
        >
          View in Apps
        </button>
      ) : (
        <span />
      )}
    </div>
  );
};
