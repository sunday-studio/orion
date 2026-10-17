import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { clearToken, getUsername } from "@/api/client";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
  Select,
  type ThemePreference,
  applyTheme,
  readThemePreference,
} from "@/design-system";
import { OPEN_SHORTCUTS_EVENT } from "@/components/shared/global-shortcut-events";
import { cn } from "@/utils/cn";
import { getTimezone, setTimezone } from "@/utils/date";

const themes: readonly { value: ThemePreference; label: string }[] = [
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
  { value: "system", label: "System" },
];

const timezoneOptions = [
  { value: "UTC", label: "UTC (default)" },
  { value: "local", label: "Browser local time" },
  { value: "Europe/London", label: "Europe/London" },
  { value: "Europe/Berlin", label: "Europe/Berlin" },
  { value: "America/New_York", label: "America/New_York" },
  { value: "America/Los_Angeles", label: "America/Los_Angeles" },
  { value: "Asia/Tokyo", label: "Asia/Tokyo" },
];

const itemClass =
  "flex w-full items-center justify-between rounded-md px-1.5 py-1.5 text-left text-[13px] text-ink hover:bg-tray focus-visible:bg-tray focus-visible:outline-none";

export const AccountMenu = () => {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [theme, setTheme] = useState<ThemePreference>(readThemePreference);
  const [timezone, setTimezoneState] = useState(getTimezone);

  const signOut = () => {
    clearToken();
    navigate("/login", { replace: true });
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        aria-label="Account menu"
        className="flex size-6 items-center justify-center rounded-full outline-none focus-visible:ring-[3px] focus-visible:ring-focus"
      >
        <span
          aria-hidden
          className="block size-6 rounded-full"
          style={{ background: "conic-gradient(from 40deg, #6366f1, #14b8a6, #f59e0b, #6366f1)" }}
        />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[240px] rounded-xl p-1.5 text-[13px]">
        <div className="px-1.5 pb-1.5 pt-1 text-xs text-quiet">Signed in as {getUsername()}</div>
        <button
          type="button"
          className={itemClass}
          disabled
          title="Profile and sessions arrive with Settings"
        >
          <span className="opacity-60">Profile and sessions</span>
        </button>
        <button
          type="button"
          className={itemClass}
          onClick={() => {
            setOpen(false);
            window.dispatchEvent(new Event(OPEN_SHORTCUTS_EVENT));
          }}
        >
          Keyboard shortcuts
          <span className="text-xs text-faint">?</span>
        </button>
        <div className="my-1.5 h-px bg-hairline" />
        <div className="px-1.5 pb-1 text-xs text-quiet">Theme</div>
        <div
          role="radiogroup"
          aria-label="Theme"
          className="mx-1.5 mb-2 flex w-fit gap-0.5 rounded-lg bg-tray p-[3px]"
        >
          {themes.map((option) => (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={theme === option.value}
              onClick={() => {
                setTheme(option.value);
                applyTheme(option.value);
              }}
              className={cn(
                "rounded-md px-3 py-1 text-xs outline-none focus-visible:ring-2 focus-visible:ring-focus",
                theme === option.value
                  ? "bg-chip font-medium shadow-[0_1px_2px_rgba(0,0,0,0.08)]"
                  : "text-mid",
              )}
            >
              {option.label}
            </button>
          ))}
        </div>
        <div className="px-1.5 pb-1 text-xs text-quiet">Timezone</div>
        <div className="mx-1.5 mb-1.5">
          <Select
            aria-label="Timezone"
            className="w-full"
            options={timezoneOptions}
            value={timezone}
            onValueChange={(value) => {
              setTimezoneState(value);
              setTimezone(value);
              // Timestamps render from the stored zone, so reload to apply it everywhere.
              window.location.reload();
            }}
          />
        </div>
        <div className="my-1.5 h-px bg-hairline" />
        <a
          className={itemClass}
          href="https://github.com/sunday-studio/orion/tree/main/docs"
          target="_blank"
          rel="noreferrer"
        >
          Documentation
        </a>
        <button
          type="button"
          className={cn(itemClass, "text-[var(--color-destructive)]")}
          onClick={signOut}
        >
          Sign out
        </button>
      </PopoverContent>
    </Popover>
  );
};
