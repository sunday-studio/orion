import { cn } from "@/utils/cn";

export type LifecycleStep = {
  label: string;
  /** done = filled check, now = the step waiting on someone, todo = hollow. */
  state: "done" | "now" | "todo";
  time?: string;
};

/** Detected, Notified, Acknowledge, Resolved tracker (canvas `.track`). */
export const LifecycleTrack = ({ steps }: { steps: readonly LifecycleStep[] }) => {
  const doneCount = steps.filter((step) => step.state === "done").length;
  // The connector spans between the first and last step centres (12.5% inset each side for 4 steps).
  const fillPercent = steps.length > 1 ? (doneCount / (steps.length - 1)) * 75 : 0;

  return (
    <ol
      aria-label="Incident progress"
      className="relative mx-1.5 mt-1 grid"
      style={{ gridTemplateColumns: `repeat(${steps.length}, 1fr)` }}
    >
      <span
        aria-hidden
        className="absolute left-[12.5%] right-[12.5%] top-[9px] h-0.5 bg-input-border"
      />
      <span
        aria-hidden
        className="absolute left-[12.5%] top-[9px] h-0.5 bg-[#2a9f69]"
        style={{ width: `${Math.min(fillPercent, 75)}%` }}
      />
      {steps.map((step) => (
        <li
          key={step.label}
          className="relative z-[1] flex flex-col items-center gap-1.5 text-center"
        >
          <i
            aria-hidden
            className={cn(
              "flex size-5 items-center justify-center rounded-full border-2 bg-chip not-italic text-white",
              step.state === "done" && "border-[#2a9f69] bg-[#2a9f69]",
              step.state === "now" && "border-[#e5586f] shadow-[0_0_0_4px_rgba(229,88,111,0.15)]",
              step.state === "todo" && "border-input-border",
            )}
          >
            {step.state === "done" ? "✓" : ""}
          </i>
          <b className="text-[12.5px] font-medium leading-4">{step.label}</b>
          <small className="font-mono text-[11px] leading-[14px] text-quiet">
            {step.time ?? "-"}
          </small>
        </li>
      ))}
    </ol>
  );
};
