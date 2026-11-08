import { useState } from "react";
import { Button, PageHeader } from "@/design-system";
import { AuditTab } from "./tabs/audit-tab";
import { GeneralTab } from "./tabs/general-tab";
import { LifecycleTab } from "./tabs/lifecycle-tab";
import { SecurityTab } from "./tabs/security-tab";
import { SystemTab } from "./tabs/system-tab";

export type SettingsSection = "general" | "lifecycle" | "security" | "system" | "audit";

const COPY: Record<SettingsSection, { title: string; description: string }> = {
  general: { title: "General", description: "Instance name, public URL and time." },
  lifecycle: {
    title: "Data lifecycle",
    description: "How long Core keeps reports and logs before archiving them.",
  },
  security: {
    title: "Security",
    description: "Sign-in, the Node registration token and API tokens.",
  },
  system: { title: "System", description: "Worker, database and backups." },
  audit: { title: "Audit log", description: "Who changed what." },
};

/** One Settings page. Each section has its own URL (/settings/general, /settings/audit, ...). */
export const SettingsPage = ({ section }: { section: SettingsSection }) => {
  const [exporting, setExporting] = useState(false);
  const { title, description } = COPY[section];
  return (
    <div className="w-full pb-10">
      <PageHeader
        title={title}
        description={description}
        actions={
          section === "audit" && (
            <Button size="sm" variant="secondary" onClick={() => setExporting(true)}>
              Export
            </Button>
          )
        }
      />
      <div className="pt-3">
        {section === "general" && <GeneralTab />}
        {section === "lifecycle" && <LifecycleTab />}
        {section === "security" && <SecurityTab />}
        {section === "system" && <SystemTab />}
        {section === "audit" && <AuditTab exportOpen={exporting} onExportOpen={setExporting} />}
      </div>
    </div>
  );
};
