import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { Search, X } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Dot } from "@/design-system";
import { useGetIncidents, useGetMonitors } from "@/orion-sdk";
import { cn } from "@/utils/cn";

type PaletteItem = {
  id: string;
  group: "Monitors" | "Incidents" | "Actions";
  label: string;
  to: string;
  hint?: string;
  dot?: string;
};

const HEALTH_DOT: Record<string, string> = {
  up: "#2a9f69",
  down: "#e5484d",
  degraded: "#f59e0b",
};

const actions: PaletteItem[] = [
  { id: "a-monitor", group: "Actions", label: "New monitor", to: "/monitors?new=1", hint: "N" },
  { id: "a-server", group: "Actions", label: "Add server", to: "/servers/new", hint: "S" },
  { id: "a-app", group: "Actions", label: "Connect an app", to: "/apps/new" },
  { id: "g-overview", group: "Actions", label: "Go to Overview", to: "/", hint: "G O" },
  { id: "g-incidents", group: "Actions", label: "Go to Incidents", to: "/incidents", hint: "G I" },
  { id: "g-servers", group: "Actions", label: "Go to Servers", to: "/servers", hint: "G S" },
  { id: "g-monitors", group: "Actions", label: "Go to Monitors", to: "/monitors", hint: "G M" },
  { id: "g-alerts", group: "Actions", label: "Go to Alerts", to: "/alerts", hint: "G A" },
  { id: "g-status", group: "Actions", label: "Go to Status", to: "/status-pages" },
  { id: "g-logs", group: "Actions", label: "Go to Logs", to: "/logs", hint: "G L" },
  { id: "g-settings", group: "Actions", label: "Go to Settings", to: "/settings" },
];

const GROUPS: PaletteItem["group"][] = ["Monitors", "Incidents", "Actions"];

type CommandPaletteProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** Cmd K search: monitors and incidents from Core, plus actions and navigation (canvas "command-palette"). */
export const CommandPalette = ({ open, onOpenChange }: CommandPaletteProps) => {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const needle = query.trim().toLowerCase();

  const monitors = useGetMonitors(
    { search: needle || undefined, limit: 5 },
    { query: { enabled: open } },
  );
  const incidents = useGetIncidents({ limit: 20 }, { query: { enabled: open } });

  const items = useMemo(() => {
    const monitorItems: PaletteItem[] = (monitors.data?.monitors ?? [])
      .slice(0, 4)
      .map((monitor) => ({
        id: `m-${monitor.id}`,
        group: "Monitors",
        label: monitor.name ?? monitor.id ?? "Monitor",
        to: `/monitors/${monitor.id}`,
        dot: HEALTH_DOT[monitor.health ?? monitor.computed_health ?? ""] ?? "#a1a1aa",
      }));
    const incidentItems: PaletteItem[] = (incidents.data?.incidents ?? [])
      .filter((incident) => !needle || (incident.title ?? "").toLowerCase().includes(needle))
      .slice(0, 3)
      .map((incident) => ({
        id: `i-${incident.id}`,
        group: "Incidents",
        label: incident.title ?? incident.id ?? "Incident",
        to: `/incidents/${incident.id}`,
        dot: incident.status === "resolved" ? "#2a9f69" : "#e5484d",
      }));
    const actionItems = actions.filter(
      (item) => !needle || item.label.toLowerCase().includes(needle),
    );
    return needle
      ? [...monitorItems, ...incidentItems, ...actionItems]
      : [...monitorItems.slice(0, 2), ...incidentItems.slice(0, 1), ...actionItems.slice(0, 3)];
  }, [monitors.data, incidents.data, needle]);

  useEffect(() => setActive(0), [needle]);

  const go = (item: PaletteItem) => {
    onOpenChange(false);
    setQuery("");
    navigate(item.to);
  };

  let flatIndex = -1;
  const rows: ReactNode[] = [];
  for (const group of GROUPS) {
    const groupItems = items.filter((item) => item.group === group);
    if (groupItems.length === 0) continue;
    rows.push(
      <li
        key={`h-${group}`}
        className="px-4 pb-1 pt-3 text-xs font-medium text-quiet"
        role="presentation"
      >
        {group}
      </li>,
    );
    for (const item of groupItems) {
      flatIndex += 1;
      const index = flatIndex;
      rows.push(
        <li key={item.id} role="option" aria-selected={index === active}>
          <button
            type="button"
            tabIndex={-1}
            onMouseMove={() => setActive(index)}
            onClick={() => go(item)}
            className={cn(
              "flex w-full items-center gap-2.5 px-4 py-2 text-left text-[13.5px]",
              index === active && "bg-tray",
            )}
          >
            {item.dot && <Dot color={item.dot} />}
            <span className="min-w-0 flex-1 truncate">{item.label}</span>
            <span className="text-xs text-faint">
              {index === active && item.group !== "Actions" ? "Enter" : item.hint}
            </span>
          </button>
        </li>,
      );
    }
  }

  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/45 backdrop-blur-[2px]" />
        <DialogPrimitive.Popup
          aria-label="Search"
          className="fixed left-1/2 top-24 z-50 w-[580px] max-w-[calc(100%-2rem)] -translate-x-1/2 overflow-hidden rounded-[14px] bg-card shadow-overlay outline-none"
        >
          <div className="flex items-center gap-2.5 border-b border-hairline px-4 py-3.5">
            <Search className="size-4 text-quiet" aria-hidden />
            <input
              autoFocus
              aria-label="Search destinations"
              placeholder="Search monitors, incidents and actions"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "ArrowDown") {
                  event.preventDefault();
                  setActive((index) => Math.min(index + 1, items.length - 1));
                } else if (event.key === "ArrowUp") {
                  event.preventDefault();
                  setActive((index) => Math.max(index - 1, 0));
                } else if (event.key === "Enter" && items[active]) {
                  event.preventDefault();
                  go(items[active]);
                }
              }}
              className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-faint"
            />
            <span className="text-xs text-faint">Esc</span>
            <DialogPrimitive.Close
              aria-label="Close"
              className="rounded p-0.5 text-quiet hover:text-ink"
            >
              <X className="size-4" />
            </DialogPrimitive.Close>
          </div>
          <ul
            role="listbox"
            aria-label="Destinations"
            className="m-0 max-h-[360px] list-none overflow-y-auto p-0 pb-2"
          >
            {rows.length > 0 ? (
              rows
            ) : (
              <li className="px-4 py-4 text-[13px] text-quiet">No matches.</li>
            )}
          </ul>
        </DialogPrimitive.Popup>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
};
