import type { ApiAgentSummaryResponse } from "@/orion-sdk";
import { cn } from "@/lib/utils";

export type AgentSummaryFilter =
  | "all"
  | "up"
  | "down"
  | "degraded"
  | "unknown"
  | "maintenance"
  | "stale"
  | "incidents";

type AgentSummaryProps = {
  selectedFilter: AgentSummaryFilter;
  summary?: ApiAgentSummaryResponse;
  onFilterChange: (filter: AgentSummaryFilter) => void;
};

const ditherBackground =
  "after:pointer-events-none after:absolute after:right-0 after:bottom-0 after:bg-[radial-gradient(currentColor_1px,transparent_1px)] after:bg-[size:5px_5px] after:opacity-35";

export const AgentSummary = ({ selectedFilter, summary, onFilterChange }: AgentSummaryProps) => {
  const total = summary?.total ?? 0;
  const label = total === 1 ? "server" : "servers";
  const items: Array<{
    filter: AgentSummaryFilter;
    label: string;
    value: string | number;
    selectedClassName: string;
    selectedTextClassName: string;
    ditherClassName: string;
  }> = [
    {
      filter: "all",
      label: "total",
      value: `${total} ${label}`,
      selectedClassName: "bg-neutral-200 hover:bg-neutral-300",
      selectedTextClassName: "text-neutral-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-15 after:w-22 after:[clip-path:polygon(50%_0,61%_30%,94%_18%,74%_48%,100%_78%,64%_72%,50%_100%,36%_72%,0_78%,26%_48%,6%_18%,39%_30%)]",
    },
    {
      filter: "up",
      label: "up",
      value: summary?.up ?? 0,
      selectedClassName: "bg-emerald-200",
      selectedTextClassName: "text-emerald-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-16 after:w-20 after:[clip-path:polygon(52%_0,72%_28%,100%_34%,80%_58%,84%_100%,52%_78%,18%_100%,24%_58%,0_34%,32%_28%)]",
    },
    {
      filter: "down",
      label: "down",
      value: summary?.down ?? 0,
      selectedClassName: "bg-rose-200",
      selectedTextClassName: "text-rose-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-18 after:w-16 after:[clip-path:polygon(48%_0,68%_20%,62%_38%,100%_46%,72%_62%,88%_100%,50%_82%,12%_100%,28%_62%,0_46%,38%_38%,32%_20%)]",
    },
    {
      filter: "degraded",
      label: "degraded",
      value: summary?.degraded ?? 0,
      selectedClassName: "bg-amber-200",
      selectedTextClassName: "text-amber-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-16 after:w-22 after:[clip-path:polygon(0_0,70%_0,58%_34%,100%_34%,42%_100%,54%_62%,12%_62%)]",
    },
    {
      filter: "unknown",
      label: "unknown",
      value: summary?.unknown ?? 0,
      selectedClassName: "bg-zinc-200",
      selectedTextClassName: "text-zinc-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-16 after:w-20 after:[clip-path:polygon(50%_0,58%_18%,78%_8%,72%_30%,96%_38%,76%_52%,88%_76%,62%_70%,50%_100%,38%_70%,12%_76%,24%_52%,4%_38%,28%_30%,22%_8%,42%_18%)]",
    },
    {
      filter: "maintenance",
      label: "maintenance",
      value: summary?.maintenance ?? 0,
      selectedClassName: "bg-sky-200",
      selectedTextClassName: "text-sky-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-14 after:w-22 after:[clip-path:polygon(12%_18%,42%_18%,50%_0,58%_18%,88%_18%,72%_42%,100%_54%,68%_66%,80%_100%,50%_78%,20%_100%,32%_66%,0_54%,28%_42%)]",
    },
    {
      filter: "stale",
      label: "stale",
      value: summary?.stale ?? 0,
      selectedClassName: "bg-orange-200",
      selectedTextClassName: "text-orange-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-18 after:w-20 after:[clip-path:polygon(50%_0,62%_24%,88%_12%,76%_40%,100%_54%,72%_64%,78%_96%,50%_76%,22%_96%,28%_64%,0_54%,24%_40%,12%_12%,38%_24%)]",
    },
    {
      filter: "incidents",
      label: "incidents",
      value: summary?.has_incidents ?? 0,
      selectedClassName: "bg-red-200",
      selectedTextClassName: "text-red-900",
      ditherClassName:
        "after:-right-4 after:-bottom-4 after:h-18 after:w-20 after:[clip-path:polygon(50%_0,58%_28%,84%_10%,72%_42%,100%_38%,78%_58%,98%_86%,64%_76%,50%_100%,36%_76%,2%_86%,22%_58%,0_38%,28%_42%,16%_10%,42%_28%)]",
    },
  ];

  return (
    <div className="grid gap-1 py-2 text-sm sm:grid-cols-4 xl:grid-cols-8">
      {items.map((item) => {
        const isSelected = selectedFilter === item.filter;

        return (
          <button
            key={item.filter}
            type="button"
            className={cn(
              "relative flex h-26 flex-col justify-between overflow-hidden p-3 text-left transition-colors",
              ditherBackground,
              item.ditherClassName,
              isSelected
                ? cn(item.selectedClassName, item.selectedTextClassName)
                : "bg-neutral-100 text-neutral-300 hover:bg-neutral-100/90",
            )}
            onClick={() => onFilterChange(item.filter)}
          >
            <span
              className={cn(
                "text-neutral-700 capitalize",
                isSelected && item.selectedTextClassName,
              )}
            >
              {item.label}
            </span>
            <span
              className={cn(
                "font-medium text-lg text-neutral-600",
                isSelected && item.selectedTextClassName,
              )}
            >
              {item.value}
            </span>
          </button>
        );
      })}
    </div>
  );
};
