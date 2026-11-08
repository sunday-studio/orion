import { type FocusEvent, type KeyboardEvent, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { OPEN_PALETTE_EVENT } from "@/components/shared/global-shortcut-events";
import { cn } from "@/utils/cn";
import { type ActiveNav, NAV_GROUPS, NAV_LEAVES, type NavGroup } from "./nav-model";
import { LogoMark, NavIcon, PanelIcon, SearchIcon } from "./sidebar-icons";
import { CollapsedFooter } from "./sidebar-footer";
import { iconClass, railButtonClass } from "./sidebar-styles";
import type { HealthChip } from "./use-health-chip";

type RailProps = {
  active: ActiveNav;
  chip: HealthChip;
  onExpand: () => void;
};

const flyoutLink = (on: boolean) =>
  cn(
    "flex h-7 items-center rounded-lg px-2.5 text-[13px] no-underline outline-none focus-visible:ring-[3px] focus-visible:ring-focus",
    on ? "bg-tray font-semibold text-ink" : "text-mid hover:bg-field",
  );

type GroupButtonProps = {
  group: NavGroup;
  active: ActiveNav;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** One group in the rail: an icon that opens a flyout of its pages on hover, focus or click. */
const RailGroup = ({ group, active, open, onOpenChange }: GroupButtonProps) => {
  const button = useRef<HTMLButtonElement>(null);
  const here = active.groupId === group.id;
  const listId = `rail-flyout-${group.id}`;

  const onBlur = (event: FocusEvent<HTMLDivElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) onOpenChange(false);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Escape" || !open) return;
    event.stopPropagation();
    onOpenChange(false);
    button.current?.focus();
  };

  return (
    <div
      className="relative"
      onMouseEnter={() => onOpenChange(true)}
      onMouseLeave={() => onOpenChange(false)}
      onFocus={() => onOpenChange(true)}
      onBlur={onBlur}
      onKeyDown={onKeyDown}
    >
      <button
        ref={button}
        type="button"
        aria-label={group.label}
        aria-haspopup="true"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        onClick={() => onOpenChange(true)}
        className={railButtonClass(here)}
      >
        <NavIcon name={group.icon} className={iconClass} />
      </button>
      {open && (
        <div
          id={listId}
          role="group"
          aria-label={group.label}
          // The ::before strip keeps the pointer inside the group while it crosses the gap.
          className="absolute left-[44px] top-[-4px] z-40 flex min-w-[168px] flex-col gap-px rounded-xl border border-input-border bg-card p-2 text-left text-[13px] shadow-overlay before:absolute before:inset-y-0 before:-left-3 before:w-3 before:content-['']"
        >
          <b className="px-2 pb-1.5 pt-0.5 text-xs font-semibold text-quiet">{group.label}</b>
          {group.pages.map((page) => {
            const on = here && active.pageId === page.id;
            return (
              <Link
                key={page.id}
                to={page.to}
                aria-current={on ? "page" : undefined}
                onClick={() => onOpenChange(false)}
                className={flyoutLink(on)}
              >
                {page.label}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
};

/** The sidebar collapsed to an icon rail. */
export const SidebarRail = ({ active, chip, onExpand }: RailProps) => {
  const [flyout, setFlyout] = useState<string | null>(null);
  return (
    <aside
      aria-label="Main"
      className="flex w-14 shrink-0 flex-col items-center border-r border-hairline bg-page pb-2.5 pt-3.5"
    >
      <Link
        to="/"
        aria-label="Orion, go to Overview"
        className="flex justify-center rounded-md pb-2 pt-0.5 outline-none focus-visible:ring-[3px] focus-visible:ring-focus"
      >
        <LogoMark />
      </Link>
      <button
        type="button"
        aria-label="Expand the sidebar"
        title="Expand the sidebar"
        onClick={onExpand}
        className={cn(railButtonClass(false), "text-faint")}
      >
        <PanelIcon direction="open" />
      </button>
      <button
        type="button"
        aria-label="Search"
        title="Search"
        onClick={() => window.dispatchEvent(new Event(OPEN_PALETTE_EVENT))}
        className={cn(railButtonClass(false), "text-faint")}
      >
        <SearchIcon />
      </button>

      <nav aria-label="Primary" className="mt-[18px] flex flex-col items-center gap-0.5">
        {NAV_LEAVES.map((leaf) => {
          const on = active.pageId === leaf.id && active.groupId === null;
          return (
            <Link
              key={leaf.id}
              to={leaf.to}
              aria-label={leaf.label}
              title={leaf.label}
              aria-current={on ? "page" : undefined}
              className={railButtonClass(on)}
            >
              <NavIcon name={leaf.icon} className={iconClass} />
              {leaf.id === "incidents" && chip.incidents > 0 && (
                <>
                  <i
                    aria-hidden="true"
                    className="absolute right-[7px] top-[7px] size-[7px] rounded-full border-[1.5px] border-page bg-dot-down"
                  />
                  <span className="sr-only">{chip.incidents} ongoing</span>
                </>
              )}
            </Link>
          );
        })}
        {NAV_GROUPS.map((group) => (
          <RailGroup
            key={group.id}
            group={group}
            active={active}
            open={flyout === group.id}
            onOpenChange={(next) =>
              setFlyout((current) => (next ? group.id : current === group.id ? null : current))
            }
          />
        ))}
      </nav>

      <CollapsedFooter chip={chip} />
    </aside>
  );
};

/** Closed state: the rail is gone and one button brings it back. */
export const SidebarReopen = ({ onOpen }: { onOpen: () => void }) => (
  <button
    type="button"
    aria-label="Open the sidebar"
    title="Open the sidebar"
    onClick={onOpen}
    className="absolute left-3.5 top-3.5 z-30 flex size-[34px] items-center justify-center rounded-[10px] border border-input-border bg-card text-faint outline-none hover:bg-field focus-visible:ring-[3px] focus-visible:ring-focus"
  >
    <PanelIcon direction="open" />
  </button>
);
