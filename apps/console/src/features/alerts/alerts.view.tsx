import { useState } from "react";
import { Button, PageHeader } from "@/design-system";
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

export type AlertsSection = "logs" | "channels" | "rules" | "routes";

const COPY: Record<AlertsSection, { title: string; description: string }> = {
  logs: { title: "Notifications", description: "Who got told, and whether it worked." },
  channels: { title: "Channels", description: "Where notifications are sent." },
  rules: { title: "Rules", description: "Which events send a notification." },
  routes: { title: "Routes", description: "Which channels each label or monitor reaches." },
};

/** One Notify page: the notification log, channels, rules or routes, each at its own URL. */
export const AlertsPage = ({ tab }: { tab: AlertsSection }) => {
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
        title={COPY[tab].title}
        description={COPY[tab].description}
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
      <div className="mt-3">
        {tab === "logs" && <NotificationLogTab />}
        {tab === "channels" && (
          <ChannelsTab
            onAdd={() => setDialog({ open: true })}
            onEdit={(channel) => setDialog({ open: true, channel })}
          />
        )}
        {tab === "rules" && (
          <RulesTab
            channels={channels.data?.channels ?? []}
            editor={ruleEditor}
            onEditorChange={setRuleEditor}
          />
        )}
        {tab === "routes" && (
          <RoutesTab
            channels={channels.data?.channels ?? []}
            editor={routeEditor}
            onEditorChange={setRouteEditor}
          />
        )}
      </div>
      <ChannelDialog
        open={dialog.open}
        channel={dialog.channel}
        onOpenChange={(open) => setDialog((current) => ({ ...current, open }))}
      />
    </div>
  );
};
