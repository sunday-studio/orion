import { Search, Settings } from "lucide-react";
import { NavLink } from "react-router-dom";
import { AccountMenu } from "@/components/shared/account-menu";
import { OPEN_PALETTE_EVENT } from "@/components/shared/global-shortcut-events";
import { getToken } from "@/api/client";
import { cn } from "@/utils/cn";
import { useGetHealthSummary } from "@/orion-sdk";

const navItems = [
  { label: "Overview", to: "/" },
  { label: "Incidents", to: "/incidents" },
  { label: "Servers", to: "/servers" },
  { label: "Monitors", to: "/monitors" },
  { label: "Alerts", to: "/alerts" },
  { label: "Status", to: "/status-pages" },
  { label: "Logs", to: "/logs" },
];

const healthChip = {
  up: { text: "All systems normal", tone: "bg-teal-100 text-teal-900", dot: "bg-dot-up" },
  down: { text: "Has issues", tone: "bg-amber-100 text-amber-900", dot: "bg-dot-warn" },
  degraded: { text: "Has issues", tone: "bg-amber-100 text-amber-900", dot: "bg-dot-warn" },
  maintenance: {
    text: "In maintenance",
    tone: "bg-violet-100 text-violet-900",
    dot: "bg-dot-idle",
  },
  stale: { text: "Has stale data", tone: "bg-amber-100 text-amber-900", dot: "bg-dot-warn" },
  unknown: { text: "No data yet", tone: "bg-neutral-100 text-neutral-700", dot: "bg-dot-idle" },
} as const;

export const AppHeader = () => {
  const summaryResponse = useGetHealthSummary();
  const overallHealth = summaryResponse.data?.overall_health ?? "unknown";
  const chip = healthChip[overallHealth as keyof typeof healthChip] ?? healthChip.unknown;
  const hasSession = Boolean(getToken());

  return (
    <header className="z-50 grid h-[52px] shrink-0 grid-cols-[1fr_auto_1fr] items-center border-b border-hairline bg-white px-4 sm:px-6">
      <NavLink to="/" className="font-medium">
        Orion
      </NavLink>
      <nav
        aria-label="Primary navigation"
        className="flex items-center gap-0.5 rounded-[9px] bg-[#f4f4f2] p-[3px] text-sm"
      >
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              cn(
                "rounded-md px-3 py-1 text-neutral-600 hover:text-ink",
                isActive && "bg-white text-ink shadow-[0_0_0_1px_rgba(0,0,0,0.06)]",
              )
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
      <div className="flex items-center justify-end gap-3">
        <button
          type="button"
          aria-label="Search"
          onClick={() => window.dispatchEvent(new Event(OPEN_PALETTE_EVENT))}
          className="hidden h-8 items-center gap-2 rounded-lg border border-input-border px-2.5 text-sm text-muted md:flex"
        >
          <Search className="size-4" />
          Search
          <kbd className="font-mono text-xs text-faint">⌘K</kbd>
        </button>
        <span
          className={cn(
            "hidden items-center gap-1.5 rounded-md px-2 py-0.5 text-xs sm:inline-flex",
            chip.tone,
          )}
        >
          <span aria-hidden className={cn("size-2 rounded-full", chip.dot)} />
          {summaryResponse.isLoading ? "Checking..." : chip.text}
        </span>
        <NavLink
          to="/settings"
          aria-label="Settings"
          className={({ isActive }) =>
            cn(
              "rounded-md p-1.5 text-neutral-600 hover:text-ink",
              isActive && "bg-neutral-100 text-ink",
            )
          }
        >
          <Settings className="size-4" />
        </NavLink>
        {hasSession && <AccountMenu />}
      </div>
    </header>
  );
};
