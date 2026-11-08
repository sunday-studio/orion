import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { OPEN_PALETTE_EVENT } from "@/components/shared/global-shortcut-events";
import { cn } from "@/utils/cn";
import {
  type ActiveNav,
  type GroupOverrides,
  NAV_GROUPS,
  NAV_LEAVES,
  isGroupOpen,
  toggleGroup,
} from "./nav-model";
import { ChevronIcon, LogoMark, NavIcon, PanelIcon, SearchIcon } from "./sidebar-icons";
import { SidebarFooter } from "./sidebar-footer";
import { childClass, countBadge, iconClass, itemClass } from "./sidebar-styles";
import type { HealthChip } from "./use-health-chip";

type SidebarProps = {
  active: ActiveNav;
  chip: HealthChip;
  onCollapse: () => void;
};

/** The open sidebar: search, two top-level pages, four groups, issue count and account. */
export const Sidebar = ({ active, chip, onCollapse }: SidebarProps) => {
  const [overrides, setOverrides] = useState<GroupOverrides>({});

  // Moving into another section starts fresh: that section opens and the others close.
  useEffect(() => setOverrides({}), [active.groupId]);

  return (
    <aside
      aria-label="Main"
      className="flex w-[216px] shrink-0 flex-col border-r border-hairline bg-page px-2 pb-2 pt-4 text-[13px]"
    >
      <div className="flex items-center justify-between px-1.5 pb-3 pt-0.5">
        <Link
          to="/"
          aria-label="Orion, go to Overview"
          className="flex items-center gap-2 rounded-md text-sm font-semibold tracking-[-0.01em] text-ink outline-none focus-visible:ring-[3px] focus-visible:ring-focus"
        >
          <LogoMark />
          Orion
        </Link>
        <button
          type="button"
          aria-label="Collapse the sidebar"
          title="Collapse the sidebar"
          onClick={onCollapse}
          className="flex size-7 items-center justify-center rounded-lg text-faint outline-none hover:bg-tray focus-visible:ring-[3px] focus-visible:ring-focus"
        >
          <PanelIcon />
        </button>
      </div>

      <button
        type="button"
        aria-label="Search"
        onClick={() => window.dispatchEvent(new Event(OPEN_PALETTE_EVENT))}
        className="flex h-[30px] w-full items-center gap-[9px] rounded-lg border border-input-border px-[9px] text-left text-[13px] text-quiet outline-none hover:bg-field focus-visible:ring-[3px] focus-visible:ring-focus"
      >
        <SearchIcon className="shrink-0 text-faint" />
        <span className="flex-1">Search</span>
        <kbd className="font-sans text-[11px] text-faint">⌘K</kbd>
      </button>

      <nav aria-label="Primary" className="mt-[18px] flex min-h-0 flex-col gap-px overflow-y-auto">
        {NAV_LEAVES.map((leaf) => {
          const on = active.pageId === leaf.id && active.groupId === null;
          return (
            <Link
              key={leaf.id}
              to={leaf.to}
              aria-current={on ? "page" : undefined}
              className={itemClass({ page: on })}
            >
              <NavIcon name={leaf.icon} className={iconClass} />
              <span className="flex-1">{leaf.label}</span>
              {leaf.id === "incidents" && chip.incidents > 0 && (
                <em
                  className={cn(countBadge, "not-italic")}
                  aria-label={`${chip.incidents} ongoing`}
                >
                  {chip.incidents}
                </em>
              )}
            </Link>
          );
        })}

        {NAV_GROUPS.map((group) => {
          const open = isGroupOpen(group.id, active.groupId, overrides);
          const here = active.groupId === group.id;
          const listId = `nav-group-${group.id}`;
          return (
            <div key={group.id}>
              <button
                type="button"
                aria-expanded={open}
                aria-controls={listId}
                onClick={() => setOverrides(toggleGroup(group.id, active.groupId, overrides))}
                className={itemClass({ group: here || open })}
              >
                <NavIcon name={group.icon} className={iconClass} />
                <span className="flex-1">{group.label}</span>
                {!open && <ChevronIcon className="shrink-0 text-[#c4c4cc] dark:text-faint" />}
              </button>
              {open && (
                <div
                  id={listId}
                  role="group"
                  aria-label={group.label}
                  className="mb-1.5 ml-[17px] mt-0.5 flex flex-col gap-px border-l-[1.5px] border-hairline"
                >
                  {group.pages.map((page) => {
                    const on = here && active.pageId === page.id;
                    return (
                      <Link
                        key={page.id}
                        to={page.to}
                        aria-current={on ? "page" : undefined}
                        className={childClass(on)}
                      >
                        {page.label}
                      </Link>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </nav>

      <SidebarFooter chip={chip} />
    </aside>
  );
};
