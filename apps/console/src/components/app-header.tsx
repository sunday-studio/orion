import { RadialAvatar } from "@/components/radial-avatar";
import { useGetHealthSummary } from "@/orion-sdk";
import { NavLink } from "react-router-dom";

const navItems = [
  { label: "Incidents", to: "/incidents" },
  { label: "Agents", to: "/agents" },
  { label: "Alerts", to: "/alerts" },
  { label: "Logs", to: "/logs" },
  { label: "Settings", to: "/settings" },
];

const healthLabel = {
  up: "All good",
  down: "Issues",
  degraded: "Issues",
  maintenance: "Maintenance",
  stale: "Issues",
  unknown: "Unknown",
} as const;

export const AppHeader = () => {
  const summaryResponse = useGetHealthSummary();
  const overallHealth = summaryResponse.data?.overall_health ?? "unknown";
  const label = healthLabel[overallHealth as keyof typeof healthLabel] ?? "Unknown";

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
                isActive ? "text-lime-700" : "text-neutral-600 hover:text-neutral-950"
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
        </div>
      </div>
    </header>
  );
};
