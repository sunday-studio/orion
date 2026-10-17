import { Search, Settings } from "lucide-react";
import { NavLink } from "react-router-dom";
import { getToken } from "@/api/client";
import { AccountMenu } from "@/components/shared/account-menu";
import { OPEN_PALETTE_EVENT } from "@/components/shared/global-shortcut-events";
import { ACTIVE_INCIDENT_PARAMS } from "@/features/overview/overview-queries";
import { useGetHealthIssues, useGetHealthSummary, useGetIncidents } from "@/orion-sdk";
import { cn } from "@/utils/cn";

const navItems = [
  { label: "Overview", to: "/" },
  { label: "Incidents", to: "/incidents" },
  { label: "Servers", to: "/servers" },
  { label: "Monitors", to: "/monitors" },
  { label: "Alerts", to: "/alerts" },
  { label: "Status", to: "/status-pages" },
  { label: "Logs", to: "/logs" },
];

/** Logo mark: a dashed orbit around a point (canvas artboards, header). */
const LogoMark = () => (
  <span className="flex size-[22px] items-center justify-center rounded-[7px] bg-[#1f7a50] text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.25),0_1px_2px_rgba(0,0,0,0.2)]">
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="3" />
      <circle cx="12" cy="12" r="9" strokeDasharray="2 4" />
    </svg>
  </span>
);

type HealthChip = { text: string; dot: string };

const useHealthChip = (): HealthChip => {
  const summary = useGetHealthSummary();
  const issues = useGetHealthIssues();
  const incidents = useGetIncidents(ACTIVE_INCIDENT_PARAMS);
  const monitorTotal = summary.data?.monitors?.total ?? 0;
  const agentTotal = summary.data?.agents?.total ?? 0;
  // The chip counts open incidents; health issues stand in until an incident exists.
  const issueCount =
    incidents.data?.count ?? incidents.data?.incidents?.length ?? issues.data?.count ?? 0;

  if (summary.isSuccess && monitorTotal === 0 && agentTotal === 0) {
    return { text: "No data yet", dot: "bg-[#a1a1aa]" };
  }
  if (issueCount > 0) {
    return { text: `${issueCount} ${issueCount === 1 ? "issue" : "issues"}`, dot: "bg-[#f59e0b]" };
  }
  if (summary.isLoading) return { text: "Checking...", dot: "bg-[#a1a1aa]" };
  return { text: "All systems normal", dot: "bg-[#2a9f69]" };
};

export const AppHeader = () => {
  const chip = useHealthChip();
  const hasSession = Boolean(getToken());

  return (
    <header className="z-50 grid h-[52px] shrink-0 grid-cols-[1fr_auto_1fr] items-center gap-4 border-b border-[#ececea] bg-page px-5">
      <NavLink
        to="/"
        className="flex items-center gap-[9px] text-sm font-semibold tracking-[-0.01em] text-ink"
      >
        <LogoMark />
        Orion
      </NavLink>
      <nav
        aria-label="Primary navigation"
        className="flex gap-0.5 rounded-[9px] bg-[#f4f4f2] p-[3px] shadow-[inset_0_0_0_1px_rgba(0,0,0,0.03)]"
      >
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              cn(
                "whitespace-nowrap rounded-md px-[11px] py-[5px] text-[13px] leading-[18px] text-[#71717a] hover:text-ink",
                isActive &&
                  "bg-white font-medium text-ink shadow-[0_1px_2px_rgba(0,0,0,0.08),0_0_0_1px_rgba(0,0,0,0.04)]",
              )
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
      <div className="flex items-center justify-end gap-2.5">
        <button
          type="button"
          aria-label="Search"
          onClick={() => window.dispatchEvent(new Event(OPEN_PALETTE_EVENT))}
          className="hidden h-[30px] items-center gap-2 rounded-lg border border-[#e4e4e7] bg-white pl-2.5 pr-2 text-[12.5px] text-[#a1a1aa] shadow-[0_1px_1px_rgba(0,0,0,0.04)] md:flex"
        >
          <Search className="size-3.5" />
          Search
          <kbd className="rounded bg-[#f4f4f2] px-[5px] py-px font-mono text-[11px] text-[#71717a]">
            ⌘K
          </kbd>
        </button>
        <span
          className="hidden h-[30px] items-center gap-1.5 rounded-lg bg-[#fafaf9] px-2.5 text-[12.5px] text-[#52525b] shadow-[inset_0_0_0_1px_#ececea] sm:flex"
          role="status"
        >
          <i className={cn("block size-1.5 rounded-full", chip.dot)} />
          {chip.text}
        </span>
        <NavLink
          to="/settings"
          aria-label="Settings"
          className={({ isActive }) =>
            cn(
              "flex size-[30px] items-center justify-center rounded-lg text-[#71717a]",
              isActive && "bg-[#f4f4f2] text-ink",
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
