import { RadialAvatar } from "@/components/radial-avatar";
import { Button } from "@/components/ui/button";
import { clearToken, getToken } from "@/lib/custom-instance";
import { useGetHealthSummary } from "@/orion-sdk";
import { LogOut } from "lucide-react";
import { NavLink, useNavigate } from "react-router-dom";

const navItems = [
  { label: "Incidents", to: "/incidents" },
  { label: "Agents", to: "/agents" },
  { label: "Monitors", to: "/monitors" },
  { label: "Alerts", to: "/alerts" },
  { label: "Logs", to: "/logs" },
  { label: "Settings", to: "/settings" },
];

const healthLabel = {
  up: "All good",
  down: "Has issues",
  degraded: "Has issues",
  maintenance: "In maintenance",
  stale: "Has stale data",
  unknown: "Unknown",
} as const;

export const AppHeader = () => {
  const navigate = useNavigate();
  const summaryResponse = useGetHealthSummary();
  const overallHealth = summaryResponse.data?.overall_health ?? "unknown";
  const label = healthLabel[overallHealth as keyof typeof healthLabel] ?? "Unknown";
  const hasSession = Boolean(getToken());

  const signOut = () => {
    clearToken();
    navigate("/login", { replace: true });
  };

  return (
    <header className="sticky inset-x-0 top-0 z-50 backdrop-blur h-14  flex items-center bg-neutral-100">
      <div className="flex w-full items-center justify-between gap-4 px-4 sm:px-6 max-w-7xl mx-auto">
        <NavLink to="/incidents" className="font-medium">
          Orion
        </NavLink>
        <nav
          aria-label="Primary navigation"
          className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm"
        >
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                isActive ? "text-indigo-700" : "text-neutral-700 hover:text-neutral-950"
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="flex items-center gap-4">
          <span className="text-sm text-neutral-600">
            {summaryResponse.isLoading ? "Checking..." : label}
          </span>
          <RadialAvatar seed="casprin-eSs" />
          {hasSession && (
            <Button
              aria-label="Sign out"
              className="size-8 rounded-full px-0"
              onClick={signOut}
              size="icon"
              variant="ghost"
            >
              <LogOut className="size-4" />
            </Button>
          )}
        </div>
      </div>
    </header>
  );
};
