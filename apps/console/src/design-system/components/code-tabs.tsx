import { useState } from "react";
import { cn } from "@/utils/cn";

export type CodeTab = { id: string; label: string; code: string };

/** Dark code panel with language tabs along the top (canvas "Send a log" block). */
export const CodeTabs = ({
  tabs,
  "aria-label": ariaLabel,
}: {
  tabs: readonly CodeTab[];
  "aria-label": string;
}) => {
  const [active, setActive] = useState(tabs[0]?.id);
  const current = tabs.find((tab) => tab.id === active) ?? tabs[0];

  return (
    <div className="overflow-hidden rounded-lg bg-[#142a23]">
      <div
        role="tablist"
        aria-label={ariaLabel}
        className="flex gap-0 border-b border-white/10 px-1"
      >
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={tab.id === current?.id}
            onClick={() => setActive(tab.id)}
            className={cn(
              "-mb-px border-b-2 px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.08em] outline-none focus-visible:ring-2 focus-visible:ring-[#80ffaa]/40",
              tab.id === current?.id
                ? "border-[#80ffaa] text-[#80ffaa]"
                : "border-transparent text-[#7f9a8e] hover:text-[#cfe5da]",
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <pre
        role="tabpanel"
        className="m-0 overflow-x-auto whitespace-pre px-3 py-3.5 font-mono text-[12.5px] leading-[1.65] text-[#e6f1ec]"
      >
        {current?.code}
      </pre>
    </div>
  );
};
