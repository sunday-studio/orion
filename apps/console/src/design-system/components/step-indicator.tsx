import { cn } from "@/utils/cn";

/** Centered "1 Create app  2 Add the key  3 First log" progress line; the current step is dark. */
export const StepIndicator = ({
  steps,
  current,
}: {
  steps: readonly string[];
  /** One-based index of the active step. */
  current: number;
}) => (
  <ol aria-label="Progress" className="flex items-baseline justify-center gap-2 py-3 text-[13px]">
    {steps.map((label, index) => {
      const active = index + 1 === current;
      return (
        <li
          key={label}
          aria-current={active ? "step" : undefined}
          className={active ? "text-ink" : "text-faint"}
        >
          <span className={cn("mr-1", active && "font-medium")}>{index + 1}</span>
          {label}
        </li>
      );
    })}
  </ol>
);
