import { useLocation } from "react-router-dom";
import { resolveActiveNav } from "./nav-model";
import { Sidebar } from "./sidebar";
import { SidebarRail } from "./sidebar-rail";
import type { SidebarMode } from "./sidebar-state";
import { useHealthChip } from "./use-health-chip";

type AppSidebarProps = {
  mode: SidebarMode;
  onModeChange: (mode: SidebarMode) => void;
};

/** Picks the open sidebar or the icon rail, and tells it where we are. */
export const AppSidebar = ({ mode, onModeChange }: AppSidebarProps) => {
  const { pathname, search } = useLocation();
  const chip = useHealthChip();
  const active = resolveActiveNav(pathname, search);
  if (mode === "collapsed")
    return <SidebarRail active={active} chip={chip} onExpand={() => onModeChange("open")} />;
  return <Sidebar active={active} chip={chip} onCollapse={() => onModeChange("collapsed")} />;
};
