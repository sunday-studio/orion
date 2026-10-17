import { useState } from "react";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Checkbox,
  Combobox,
  EmptyState,
  Field,
  Input,
  Select,
  Skeleton,
  StatusBadge,
  Switch,
  Tabs,
  TabsList,
  TabsPanel,
  TabsTab,
} from "@/design-system";

const statusOptions = [
  { value: "all", label: "All statuses" },
  { value: "up", label: "Up" },
  { value: "down", label: "Down" },
];

const serverOptions = [
  { value: "edge-01", label: "edge-01" },
  { value: "edge-02", label: "edge-02" },
  { value: "db-01", label: "db-01" },
];

export const Section = ({ title, children }: { title: string; children: React.ReactNode }) => (
  <section aria-label={title} className="space-y-3">
    <h2 className="text-[15px] font-medium">{title}</h2>
    {children}
  </section>
);

export const ControlsPreview = () => {
  const [status, setStatus] = useState("all");
  const [server, setServer] = useState<string | null>(null);
  const [checked, setChecked] = useState(false);
  const [enabled, setEnabled] = useState(false);

  return (
    <>
      <Section title="Buttons">
        <div className="flex flex-wrap items-center gap-2">
          <Button>Primary</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="link">Link</Button>
          <Button size="sm">Small</Button>
          <Button disabled>Disabled</Button>
        </div>
      </Section>

      <Section title="Badges">
        <div className="flex flex-wrap gap-2">
          {["up", "down", "degraded", "unknown", "maintenance", "stale", "covered", "resolved"].map(
            (status) => (
              <StatusBadge key={status} status={status} />
            ),
          )}
          <Badge tone="critical">critical</Badge>
        </div>
      </Section>

      <Section title="Form controls">
        <div className="grid max-w-md gap-3">
          <Input aria-label="Name" placeholder="Name" />
          <Select label="Status" options={statusOptions} value={status} onValueChange={setStatus} />
          <Combobox
            aria-label="Server"
            options={serverOptions}
            value={server}
            onValueChange={setServer}
            placeholder="Search servers"
          />
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox
              aria-label="Notify me"
              checked={checked}
              onCheckedChange={(next) => setChecked(Boolean(next))}
            />
            Notify me
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <Switch
              aria-label="Enabled"
              checked={enabled}
              onCheckedChange={(next) => setEnabled(Boolean(next))}
            />
            Enabled: {enabled ? "on" : "off"}
          </label>
        </div>
      </Section>

      <Section title="Tabs">
        <Tabs defaultValue="one">
          <TabsList>
            <TabsTab value="one">First</TabsTab>
            <TabsTab value="two">Second</TabsTab>
          </TabsList>
          <TabsPanel value="one">First panel</TabsPanel>
          <TabsPanel value="two">Second panel</TabsPanel>
        </Tabs>
      </Section>

      <Section title="Cards and states">
        <Field>
          <Card>
            <CardHeader>
              <CardTitle>Card title</CardTitle>
              <CardDescription>Supporting text</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <Skeleton className="h-3 w-1/2" />
              <Skeleton className="h-3 w-2/3" />
            </CardContent>
          </Card>
        </Field>
        <EmptyState
          title="No servers yet"
          description="Install the agent to start collecting reports."
          action={<Button size="sm">Add server</Button>}
        />
      </Section>
    </>
  );
};
