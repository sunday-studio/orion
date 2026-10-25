import { cn } from "@/utils/cn";

export type ChoiceTile = { value: string; title: string; caption: string };

/** Segmented tiles for picking one option, for example an operating system (canvas `.ctiles`). */
export const ChoiceTiles = ({
  tiles,
  value,
  onValueChange,
  "aria-label": ariaLabel,
}: {
  tiles: readonly ChoiceTile[];
  value: string;
  onValueChange: (value: string) => void;
  "aria-label": string;
}) => (
  <div
    role="radiogroup"
    aria-label={ariaLabel}
    className="grid overflow-hidden rounded-[10px] border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]"
    style={{ gridTemplateColumns: `repeat(${tiles.length}, 1fr)` }}
  >
    {tiles.map((tile) => {
      const selected = tile.value === value;
      return (
        <button
          key={tile.value}
          type="button"
          role="radio"
          aria-checked={selected}
          tabIndex={selected ? 0 : -1}
          onClick={() => onValueChange(tile.value)}
          onKeyDown={(event) => {
            const step =
              event.key === "ArrowRight" || event.key === "ArrowDown"
                ? 1
                : event.key === "ArrowLeft" || event.key === "ArrowUp"
                  ? -1
                  : 0;
            if (step === 0) return;
            event.preventDefault();
            const index = tiles.findIndex((candidate) => candidate.value === value);
            const next = tiles[(index + step + tiles.length) % tiles.length];
            if (next) onValueChange(next.value);
            const buttons =
              event.currentTarget.parentElement?.querySelectorAll<HTMLElement>("[role=radio]");
            buttons?.[(index + step + tiles.length) % tiles.length]?.focus();
          }}
          className={cn(
            "flex flex-col items-start gap-0.5 border-l border-hairline px-3.5 py-2.5 text-left outline-none first:border-l-0 focus-visible:ring-[3px] focus-visible:ring-focus",
            selected
              ? "bg-gradient-to-b from-[#1b3a2e] to-[#142a23] text-white"
              : "text-ink hover:bg-sub",
          )}
        >
          <b className="text-[13.5px] font-medium leading-[18px]">{tile.title}</b>
          <span
            className={cn(
              "font-mono text-[10.5px] uppercase leading-4 tracking-[0.08em]",
              selected ? "text-[#80ffaa]" : "text-faint",
            )}
          >
            {tile.caption}
          </span>
        </button>
      );
    })}
  </div>
);
