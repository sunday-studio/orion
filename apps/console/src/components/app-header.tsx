import { RadialAvatar } from "@/components/radial-avatar";
import { useGetHealthSummary } from "@/orion-sdk";
import { NavLink } from "react-router-dom";

const navItems = [
  { label: "Incidents", to: "/incidents" },
  { label: "Alerts", to: "/alerts" },
  { label: "Agents", to: "/agents" },
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
    <header className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <NavLink to="/incidents" className="font-medium">
          Orion
        </NavLink>
        <nav className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                isActive
                  ? "font-medium text-neutral-950"
                  : "text-neutral-600 hover:text-neutral-950"
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
