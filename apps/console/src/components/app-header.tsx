import { RadialAvatar } from "@/components/radial-avatar";
import { useGetHealthSummary } from "@/orion-sdk";
import { NavLink } from "react-router-dom";

const navItems = [
  { label: "Home", to: "/" },
  { label: "Servers", to: "/servers" },
  { label: "Incidents", to: "/incidents" },
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
    <header className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <NavLink to="/" className="font-medium">
          Orion
        </NavLink>
        <div className="flex items-center gap-4">
          <span className="text-sm text-neutral-600">
            {summaryResponse.isLoading ? "Checking..." : label}
          </span>
          <RadialAvatar seed="casprin-eSs" size="sm" />
        </div>
      </div>
      <nav className="flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === "/"}
            className={({ isActive }) =>
              isActive ? "font-medium text-neutral-950" : "text-neutral-600 hover:text-neutral-950"
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>
    </header>
  );
};
