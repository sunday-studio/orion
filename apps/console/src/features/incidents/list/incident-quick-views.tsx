import { Segmented } from "@/design-system";
import { useGetIncidents } from "@/orion-sdk";
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
    <Segmented
      role="group"
      aria-label="Quick views"
      options={items.map((item) => ({
        value: item.view,
        label: (
          <>
            {item.label} <span className="text-faint">{item.count}</span>
          </>
        ),
      }))}
      value={current}
      onValueChange={(next) => filters.setView(next as View)}
    />
  );
};
