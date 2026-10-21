import { MoreHorizontal } from "lucide-react";
import { Checkbox, Menu, MenuContent, MenuItem, MenuTrigger, StatusBadge } from "@/design-system";
import type { ServiceAgentDiscoveryItem } from "@/orion-sdk";
import { acceptHint, managedLabel } from "../detected-model";

const confidenceTone = (confidence?: string) =>
  confidence === "high" ? "up" : confidence === "medium" ? "degraded" : "unknown";

export const DetectedItem = ({
  item,
  selectable,
  checked,
  canAct,
  onToggle,
  onAction,
}: {
  item: ServiceAgentDiscoveryItem;
  selectable: boolean;
  checked: boolean;
  /** The agent can take requests from the Console. */
  canAct: boolean;
  onToggle: () => void;
  onAction: (type: "dismiss" | "restore" | "ignore") => void;
}) => (
  <div className="grid min-h-[44px] grid-cols-[20px_minmax(0,1fr)_auto_auto_28px] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0">
    {selectable ? (
      <Checkbox aria-label={`Select ${item.name}`} checked={checked} onCheckedChange={onToggle} />
    ) : (
      <span />
    )}
    <span className="min-w-0">
      <b className="block truncate font-medium">{item.name}</b>
      {item.detail && <small className="block truncate text-xs text-quiet">{item.detail}</small>}
      {!selectable && item.status !== "monitored" && !item.dismissed && (
        <small className="block truncate text-xs text-faint">{acceptHint(item)}</small>
      )}
    </span>
    <span className="flex items-center gap-1.5">
      {item.new && <StatusBadge status="info">New</StatusBadge>}
      {item.status === "monitored" ? (
        <StatusBadge status="up">{managedLabel(item.managed_in)}</StatusBadge>
      ) : item.dismissed ? (
        <StatusBadge status="stale">Dismissed</StatusBadge>
      ) : (
        <StatusBadge status="unknown" shape="none">
          Suggested
        </StatusBadge>
      )}
    </span>
    <StatusBadge status={confidenceTone(item.confidence)} shape="none">
      {item.confidence ?? "low"}
    </StatusBadge>
    {item.status !== "monitored" ? (
      <Menu>
        <MenuTrigger
          aria-label={`Actions for ${item.name}`}
          disabled={!canAct}
          className="flex size-7 items-center justify-center rounded-md text-faint outline-none hover:bg-sub hover:text-ink focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
        >
          <MoreHorizontal className="size-4" />
        </MenuTrigger>
        <MenuContent>
          {item.dismissed ? (
            <MenuItem onClick={() => onAction("restore")}>Restore</MenuItem>
          ) : (
            <>
              <MenuItem onClick={() => onAction("dismiss")}>Dismiss</MenuItem>
              <MenuItem onClick={() => onAction("ignore")}>Never suggest this</MenuItem>
            </>
          )}
        </MenuContent>
      </Menu>
    ) : (
      <span />
    )}
  </div>
);
