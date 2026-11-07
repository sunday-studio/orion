import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

/*
 * Class fragments for the sidebar. Colours come from the Console theme tokens, so light and dark
 * follow the rest of the app. The two icon tones are CSS variables on each item: --nav-stroke is
 * the outline and --nav-fill the soft fill (canvas --xs and --xf). The active pill and an open
 * group get the stronger pair.
 */

const toneRest =
  "[--nav-stroke:#a8a8b3] [--nav-fill:#e7e7ed] dark:[--nav-stroke:#66726c] dark:[--nav-fill:#222a26]";
const toneStrong =
  "[--nav-stroke:#8f8f9b] [--nav-fill:#dcdce4] dark:[--nav-stroke:#8b9892] dark:[--nav-fill:#303a35]";

const itemBase = cn(
  focusRing,
  "relative flex items-center rounded-lg text-[13px] no-underline transition-colors",
);

/** Top-level link or group header in the open sidebar. */
export const itemClass = ({ page, group }: { page?: boolean; group?: boolean }) =>
  cn(
    itemBase,
    "min-h-[30px] w-full gap-[9px] px-2 text-left font-medium whitespace-nowrap",
    page
      ? cn("bg-tray font-semibold text-ink", toneStrong)
      : cn("text-mid hover:bg-field", toneRest),
    group && !page && cn("font-semibold text-ink", toneStrong),
  );

/** Page link inside an open group. */
export const childClass = (on: boolean) =>
  cn(
    itemBase,
    "ml-2 h-7 px-2.5",
    on ? "bg-tray font-semibold text-ink" : "text-mid hover:bg-field hover:text-ink",
  );

/** Icon button in the collapsed rail. */
export const railButtonClass = (on: boolean, strong = false) =>
  cn(
    itemBase,
    "size-[34px] justify-center rounded-[9px]",
    on ? cn("bg-tray", toneStrong) : cn("hover:bg-field", strong ? toneStrong : toneRest),
  );

export const iconClass = "shrink-0 text-[var(--nav-stroke)]";

export const countBadge =
  "rounded-[9px] bg-[var(--tone-down-bg)] px-1.5 text-[11px] font-semibold leading-[17px] text-[var(--tone-down-fg)]";
