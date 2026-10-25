import { parseAsString, useQueryState } from "nuqs";
import { useState } from "react";
import { Button, PageHeader, Tabs, TabsList, TabsTab } from "@/design-system";
import { AppsTab } from "./tabs/apps-tab";
import { AuditTab } from "./tabs/audit-tab";
import { GeneralTab } from "./tabs/general-tab";
import { LifecycleTab } from "./tabs/lifecycle-tab";
import { SecurityTab } from "./tabs/security-tab";
import { SystemTab } from "./tabs/system-tab";

const TABS = [
  { value: "lifecycle", label: "Data lifecycle" },
  { value: "general", label: "General" },
  { value: "apps", label: "Apps" },
  { value: "security", label: "Security" },
  { value: "system", label: "System" },
  { value: "audit", label: "Audit log" },
] as const;

export const SettingsPage = () => {
  const [tab, setTab] = useQueryState("tab", parseAsString.withDefault("lifecycle"));
  const [exporting, setExporting] = useState(false);
  const active = TABS.some((item) => item.value === tab) ? tab : "lifecycle";
  return (
    <div className="w-full pb-10">
      <PageHeader
        title="Settings"
        description={
          active === "audit"
            ? "Who changed what."
            : "How Core keeps data, who can sign in, and its health."
        }
        actions={
          active === "audit" && (
            <Button size="sm" variant="secondary" onClick={() => setExporting(true)}>
              Export
            </Button>
          )
        }
      />
      <div className="pb-4 pt-3">
        <Tabs value={active} onValueChange={(next) => void setTab(next)}>
          <TabsList>
            {TABS.map((item) => (
              <TabsTab key={item.value} value={item.value}>
                {item.label}
              </TabsTab>
            ))}
          </TabsList>
        </Tabs>
      </div>
      {active === "lifecycle" && <LifecycleTab />}
      {active === "general" && <GeneralTab />}
      {active === "apps" && <AppsTab />}
      {active === "security" && <SecurityTab />}
      {active === "system" && <SystemTab />}
      {active === "audit" && <AuditTab exportOpen={exporting} onExportOpen={setExporting} />}
    </div>
  );
};
