import * as React from "react";
import { cn } from "@/utils/cn";

/**
 * A titled panel in the canvas card design: a pale tray with the title on top and the content on a
 * raised white card (`.field` + `.pc`). The first `BoxHeader` child supplies the title; everything
 * else sits on the raised card.
 */
export const Box = ({
  className,
  children,
  ...props
}: Omit<React.ComponentProps<"section">, "title">) => {
  const items = React.Children.toArray(children);
  const header = items.find(
    (child): child is React.ReactElement<BoxHeaderProps> =>
      React.isValidElement(child) && child.type === BoxHeader,
  );
  const body = items.filter((child) => child !== header);
  return (
    <FieldPanel
      compact
      centerHeader
      title={header?.props.title ?? ""}
      aside={header?.props.aside}
      className={className}
      {...props}
    >
      <RaisedCard>{body}</RaisedCard>
    </FieldPanel>
  );
};

type BoxHeaderProps = { title: React.ReactNode; aside?: React.ReactNode; className?: string };

/** Marks the title of a `Box`. It renders nothing itself; `Box` reads its props. */
export const BoxHeader = (_props: BoxHeaderProps): React.ReactNode => null;

/** Pale container holding raised cards (canvas `.field`). */
export const FieldPanel = ({
  title,
  aside,
  children,
  className,
  compact = false,
  centerHeader = false,
  ...props
}: Omit<React.ComponentProps<"section">, "title"> & {
  /** Centre the title row on the cross axis (for controls in the aside). */
  centerHeader?: boolean;
  title: React.ReactNode;
  aside?: React.ReactNode;
  /** Tight panel for a table: 12px on top, 4px on the other sides (canvas `.field` with a `.gt`). */
  compact?: boolean;
}) => (
  <section
    className={cn(
      "rounded-[14px] border border-hairline bg-field dark:shadow-[inset_0_2px_6px_rgba(0,0,0,0.55)]",
      compact ? "px-1 pb-1 pt-3" : "p-5",
      className,
    )}
    {...props}
  >
    <div
      className={cn(
        "flex justify-between",
        centerHeader ? "items-center" : "items-baseline",
        compact ? "mx-2.5 mb-2.5" : "mx-0.5 mb-3",
      )}
    >
      <h2 className="text-sm font-medium">{title}</h2>
      {aside !== undefined && <span className="text-[12.5px] text-faint">{aside}</span>}
    </div>
    {children}
  </section>
);

/** White raised card used inside a FieldPanel (canvas `.pc`). */
export const RaisedCard = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    className={cn(
      "overflow-hidden rounded-[10px] border border-hairline bg-card shadow-card dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]",
      className,
    )}
    {...props}
  />
);

/** Small number card: label above a large figure (canvas `.stat3 > div`). */
export const StatBox = ({
  label,
  value,
  of,
  className,
}: {
  label: string;
  value: React.ReactNode;
  of?: React.ReactNode;
  className?: string;
}) => (
  <div
    className={cn(
      "rounded-xl border border-hairline bg-card px-3.5 py-3 shadow-card dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]",
      className,
    )}
  >
    <span className="block text-xs leading-[18px] text-quiet">{label}</span>
    <b className="block text-xl font-semibold leading-[30px] tracking-[-0.02em]">
      {value}
      {of !== undefined && (
        <small className="ml-1 text-[13px] font-normal text-faint tracking-normal">of {of}</small>
      )}
    </b>
  </div>
);

/** Status dot used at the start of rows (8px). */
export const Dot = ({ color, className }: { color: string; className?: string }) => (
  <span
    aria-hidden
    className={cn("inline-block size-2 shrink-0 rounded-full", className)}
    style={{ background: color }}
  />
);
