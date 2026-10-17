import { Dialog, DialogContent, DialogHeader, DialogTitle, Input } from "@/design-system";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { cn } from "@/utils/cn";

export const paletteDestinations = [
  { label: "Overview", to: "/" },
  { label: "Incidents", to: "/incidents" },
  { label: "Servers", to: "/servers" },
  { label: "Monitors", to: "/monitors" },
  { label: "Alerts", to: "/alerts" },
  { label: "Status pages", to: "/status-pages" },
  { label: "Logs", to: "/logs" },
  { label: "Apps", to: "/settings/apps" },
  { label: "Settings", to: "/settings" },
] as const;

type CommandPaletteProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export const CommandPalette = ({ open, onOpenChange }: CommandPaletteProps) => {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const results = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return paletteDestinations.filter((item) => item.label.toLowerCase().includes(needle));
  }, [query]);

  const go = (to: string) => {
    onOpenChange(false);
    setQuery("");
    setActive(0);
    navigate(to);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-3 sm:max-w-md" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>Go to</DialogTitle>
        </DialogHeader>
        <Input
          autoFocus
          aria-label="Search destinations"
          placeholder="Search pages"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(0);
          }}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              setActive((index) => Math.min(index + 1, results.length - 1));
            } else if (event.key === "ArrowUp") {
              event.preventDefault();
              setActive((index) => Math.max(index - 1, 0));
            } else if (event.key === "Enter" && results[active]) {
              event.preventDefault();
              go(results[active].to);
            }
          }}
        />
        <ul aria-label="Destinations" className="max-h-72 overflow-y-auto text-sm">
          {results.map((item, index) => (
            <li key={item.to}>
              <button
                type="button"
                className={cn(
                  "w-full px-3 py-2 text-left hover:bg-neutral-100",
                  index === active && "bg-neutral-100",
                )}
                onClick={() => go(item.to)}
              >
                {item.label}
              </button>
            </li>
          ))}
          {results.length === 0 && <li className="px-3 py-2 text-neutral-600">No matches.</li>}
        </ul>
      </DialogContent>
    </Dialog>
  );
};
