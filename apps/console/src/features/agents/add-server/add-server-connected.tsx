import { Link } from "react-router-dom";
import { Button, FieldPanel, PageHeader, RaisedCard, StatusBadge } from "@/design-system";
import type { ApiAgentResponse } from "@/orion-sdk";

const pad = (value: number) => String(value).padStart(2, "0");
const formatClock = (iso: string) => {
  const date = new Date(iso);
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};

const Row = ({
  title,
  detail,
  action,
}: {
  title: string;
  detail: string;
  action?: React.ReactNode;
}) => (
  <div className="flex items-center gap-3 border-b border-line-soft px-3.5 py-[9px] text-[13px] last:border-b-0">
    <b className="w-60 shrink-0 font-medium">{title}</b>
    <span className="min-w-0 flex-1 truncate text-xs text-faint">{detail}</span>
    {action}
  </div>
);

export const AddServerConnected = ({ agent }: { agent: ApiAgentResponse }) => {
  const name = agent.name ?? "The server";
  const platform = [agent.os, agent.arch].filter(Boolean).join("/");
  return (
    <>
      <PageHeader
        title={`${name} is connected`}
        description="The agent registered and sent its first report."
      />
      <div className="mt-3 flex flex-col gap-3">
        <FieldPanel title="Server" aside="Just registered">
          <RaisedCard className="flex items-center gap-3 px-4 py-3 text-[13px]">
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-[#2a9f69]" />
            <b className="font-medium">{name}</b>
            <span className="ml-auto truncate text-xs text-faint">
              {[platform, agent.last_seen && `first report ${formatClock(agent.last_seen)}`]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <StatusBadge status="up">Reporting</StatusBadge>
          </RaisedCard>
        </FieldPanel>
        <FieldPanel title="Agent token" aside="Stored on the server">
          <RaisedCard>
            <Row title="Location" detail="/etc/orion/token, mode 0600, owner orion" />
            <Row
              title="Rotate"
              detail="From the Token tab; the old token works for 10 minutes"
              action={
                <Link to={`/servers/${agent.id}?tab=token`}>
                  <Button size="sm" variant="secondary">
                    Open Token tab
                  </Button>
                </Link>
              }
            />
          </RaisedCard>
        </FieldPanel>
        <FieldPanel title="Next" aside="Optional">
          <RaisedCard>
            <Row
              title="Add default monitors"
              detail="Disk, memory, CPU and the agent itself"
              action={
                <Button
                  size="sm"
                  disabled
                  title="Default monitors need a Core endpoint that does not exist yet"
                >
                  Add
                </Button>
              }
            />
            <Row
              title="Create a custom monitor"
              detail="HTTP, TCP or a process on this server"
              action={
                <Link to="/monitors?new=1">
                  <Button size="sm" variant="secondary">
                    Create
                  </Button>
                </Link>
              }
            />
          </RaisedCard>
        </FieldPanel>
        <div className="flex justify-center">
          <Link to={`/servers/${agent.id}`}>
            <Button variant="secondary">Go to server</Button>
          </Link>
        </div>
      </div>
    </>
  );
};
