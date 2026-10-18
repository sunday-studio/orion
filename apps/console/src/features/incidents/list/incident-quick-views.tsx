import { useGetIncidents } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import type { IncidentFilters } from "./incident-filters";

type View = "active" | "review" | "resolved";

const countOf = (data?: { incidents?: unknown[]; count?: number }) =>
  data?.count ?? data?.incidents?.length ?? 0;

/** Active / Needs review / Resolved segmented quick views with live counts. */
export const IncidentQuickViews = ({
  filters,
  current,
}: {
  filters: IncidentFilters;
  current: View;
}) => {
  const active = useGetIncidents({ status: "open,acknowledged,covered", limit: 100 });
  const review = useGetIncidents({ needs_review: true, limit: 100 });
  const resolved = useGetIncidents({ status: "resolved", limit: 100 });
  const items: { view: View; label: string; count: number }[] = [
    { view: "active", label: "Active", count: countOf(active.data) },
    { view: "review", label: "Needs review", count: countOf(review.data) },
    { view: "resolved", label: "Resolved", count: countOf(resolved.data) },
  ];
  return (
    <div role="group" aria-label="Quick views" className="flex gap-0.5 rounded-lg bg-tray p-[3px]">
      {items.map((item) => (
        <button
          key={item.view}
          type="button"
          aria-pressed={current === item.view}
          onClick={() => filters.setView(item.view)}
          className={cn(
            "whitespace-nowrap rounded-md px-3 py-1 text-[13px] outline-none focus-visible:ring-2 focus-visible:ring-focus",
            current === item.view
              ? "bg-chip font-medium shadow-[0_1px_2px_rgba(0,0,0,0.08)]"
              : "text-mid",
          )}
        >
          {item.label} <span className="font-normal text-faint">{item.count}</span>
        </button>
      ))}
    </div>
  );
};
