import { useState } from "react";
import {
  type ThemePreference,
  ToastProvider,
  applyTheme,
  readThemePreference,
} from "@/design-system";
import { Select } from "@/design-system";
import { ControlsPreview } from "@/design-system/preview/preview-controls";
import { OverlaysPreview } from "@/design-system/preview/preview-overlays";
import { TablePreview } from "@/design-system/preview/preview-table";

const themeOptions = [
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
  { value: "system", label: "System" },
];

/** Living reference for the design system. Route: /design-system. */
export const DesignSystemPage = () => {
  const [theme, setTheme] = useState<ThemePreference>(readThemePreference);

  return (
    <ToastProvider>
      <div className="space-y-8 pb-16">
        <header className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <h1 className="text-[30px] leading-9 font-medium">Design system</h1>
            <p className="text-sm text-quiet">Components, tokens and states used across Orion.</p>
          </div>
          <Select
            aria-label="Theme"
            label="Theme"
            options={themeOptions}
            value={theme}
            onValueChange={(next) => {
              setTheme(next as ThemePreference);
              applyTheme(next as ThemePreference);
            }}
          />
        </header>
        <ControlsPreview />
        <OverlaysPreview />
        <TablePreview />
      </div>
    </ToastProvider>
  );
};
