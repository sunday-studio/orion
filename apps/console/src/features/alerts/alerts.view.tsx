import { parseAsStringLiteral, useQueryState } from "nuqs";
import { useState } from "react";
import { Button, PageHeader, Tabs, TabsList, TabsPanel, TabsTab } from "@/design-system";
import { Plus } from "lucide-react";
import type {
  ApiAlertChannelResponse,
  ApiAlertRuleResponse,
  ApiLabelRouteResponse,
} from "@/orion-sdk";
import { useGetAlertChannels } from "@/orion-sdk";
import { ChannelDialog } from "./channels/channel-dialog";
import { ChannelsTab } from "./tabs/channels-tab";
import { RoutesTab } from "./tabs/routes-tab";
import { RulesTab } from "./tabs/rules-tab";
import { NotificationLogTab } from "./tabs/notification-log-tab";

const TABS = ["logs", "channels", "rules", "routes"] as const;

export const AlertsPage = () => {
  const [tab, setTab] = useQueryState("tab", parseAsStringLiteral(TABS).withDefault("logs"));
  const [dialog, setDialog] = useState<{ open: boolean; channel?: ApiAlertChannelResponse }>({
    open: false,
  });
  const [ruleEditor, setRuleEditor] = useState<{ open: boolean; rule?: ApiAlertRuleResponse }>({
    open: false,
  });
  const [routeEditor, setRouteEditor] = useState<{ open: boolean; route?: ApiLabelRouteResponse }>({
    open: false,
  });
  const channels = useGetAlertChannels();

  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Alerts"
        description="Who gets told, and whether it worked."
        actions={
          tab === "channels" ? (
            <Button size="sm" onClick={() => setDialog({ open: true })}>
              <Plus /> Add channel
            </Button>
          ) : tab === "rules" ? (
            <Button size="sm" onClick={() => setRuleEditor({ open: true })}>
              <Plus /> New rule
            </Button>
          ) : tab === "routes" ? (
            <Button size="sm" onClick={() => setRouteEditor({ open: true })}>
              <Plus /> New route
            </Button>
          ) : undefined
        }
      />
      <Tabs
        value={tab}
        onValueChange={(next) => void setTab(next as (typeof TABS)[number])}
        className="mt-3 gap-3"
      >
        <TabsList>
          <TabsTab value="logs">Notification log</TabsTab>
          <TabsTab value="channels">Channels</TabsTab>
          <TabsTab value="rules">Rules</TabsTab>
          <TabsTab value="routes">Routes</TabsTab>
        </TabsList>
        <TabsPanel value="logs">
          <NotificationLogTab />
        </TabsPanel>
        <TabsPanel value="channels">
          <ChannelsTab
            onAdd={() => setDialog({ open: true })}
            onEdit={(channel) => setDialog({ open: true, channel })}
          />
        </TabsPanel>
        <TabsPanel value="rules">
          <RulesTab
            channels={channels.data?.channels ?? []}
            editor={ruleEditor}
            onEditorChange={setRuleEditor}
          />
        </TabsPanel>
        <TabsPanel value="routes">
          <RoutesTab
            channels={channels.data?.channels ?? []}
            editor={routeEditor}
            onEditorChange={setRouteEditor}
          />
        </TabsPanel>
      </Tabs>
      <ChannelDialog
        open={dialog.open}
        channel={dialog.channel}
        onOpenChange={(open) => setDialog((current) => ({ ...current, open }))}
      />
    </div>
  );
};
