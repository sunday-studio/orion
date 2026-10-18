import { cn } from "@/utils/cn";

const COLOR: Record<string, string> = {
  up: "bg-[#2a9f69]",
  degraded: "bg-[#f59e0b]",
  down: "bg-[#e5484d]",
};

/** "Last 24 checks": one thin bar per check, oldest left (canvas `.ub`). */
export const CheckStrip = ({ checks }: { checks?: readonly string[] | null }) => {
  if (!checks || checks.length === 0) return <span className="text-faint">-</span>;
  return (
    <span aria-hidden className="flex h-4 items-end gap-px">
      {checks.slice(-24).map((check, index) => (
        <i
          key={index}
          title={check}
          className={cn("block h-4 w-[3px] rounded-[1px]", COLOR[check] ?? "bg-[#d4d4d8]")}
        />
      ))}
    </span>
  );
};
