import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  ChoiceTiles,
  Dot,
  FieldPanel,
  Input,
  PageHeader,
  RaisedCard,
  Select,
} from "@/design-system";
import { useGetAgents } from "@/orion-sdk";

/** Core's agent monitor definitions (F4 batch B) are not available yet; flip when they land. */
export const AGENT_MONITOR_API_AVAILABLE = false;

const WHAT = [
  { value: "disk", title: "Disk usage", caption: "Threshold" },
  { value: "process", title: "Process", caption: "Running" },
  { value: "memory", title: "Memory", caption: "Threshold" },
  { value: "port", title: "Local port", caption: "Listening" },
] as const;

const EVERY = [30, 60, 300].map((value) => ({
  value: String(value),
  label: value >= 60 ? `${value / 60} ${value === 60 ? "minute" : "minutes"}` : `${value} seconds`,
}));
const FAIL = [1, 2, 3, 5].map((value) => ({ value: String(value), label: `${value} reports` }));

const field = {
  disk: ["Mount path", "/"],
  process: ["Process name", "nginx"],
  memory: ["Threshold, percent", "90"],
  port: ["Port", "5432"],
} as const;

export const AgentMonitorSetupPage = () => {
  const agents = useGetAgents({ limit: 200 });
  const servers = (agents.data?.agents ?? []).filter((agent) => agent.status !== "down");
  const [server, setServer] = useState("");
  const [what, setWhat] = useState<string>("process");
  const [value, setValue] = useState("");
  const [every, setEvery] = useState("30");
  const [fail, setFail] = useState("3");
  const [notice, setNotice] = useState(false);
  const chosen = servers.find((agent) => agent.id === server) ?? servers[0];
  const [label, placeholder] = field[what as keyof typeof field] ?? field.process;

  return (
    <div className="mx-auto w-full max-w-[712px] pb-8">
      <PageHeader
        title="New agent monitor"
        description="Runs on your server through the agent and reports to Core."
        actions={
          <Link to="/monitors">
            <Button size="sm" variant="secondary">
              Cancel
            </Button>
          </Link>
        }
      />
      <div className="mt-3 flex flex-col gap-3">
        <FieldPanel title="Server" aside="Only servers with a healthy agent">
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            Server
            <Select
              aria-label="Server"
              className="w-full font-normal"
              options={servers.map((agent) => ({
                value: agent.id ?? "",
                label: `${agent.name} (${agent.status_text ?? agent.status ?? "unknown"}${
                  agent.agent_version ? `, agent ${agent.agent_version}` : ""
                })`,
              }))}
              value={chosen?.id ?? ""}
              onValueChange={setServer}
            />
          </div>
        </FieldPanel>
        <FieldPanel title="What to check">
          <div className="flex flex-col gap-3">
            <ChoiceTiles
              aria-label="What to check"
              tiles={WHAT}
              value={what}
              onValueChange={setWhat}
            />
            <label className="flex flex-col gap-1.5 text-[13px] font-medium">
              {label}
              <Input
                className="font-normal"
                placeholder={placeholder}
                value={value}
                onChange={(event) => setValue(event.target.value)}
              />
            </label>
            <div className="grid grid-cols-2 gap-4">
              <div className="flex flex-col gap-1.5 text-[13px] font-medium">
                Check every
                <Select
                  aria-label="Check every"
                  className="w-full font-normal"
                  options={EVERY}
                  value={every}
                  onValueChange={setEvery}
                />
              </div>
              <div className="flex flex-col gap-1.5 text-[13px] font-medium">
                Fail after
                <Select
                  aria-label="Fail after"
                  className="w-full font-normal"
                  options={FAIL}
                  value={fail}
                  onValueChange={setFail}
                />
              </div>
            </div>
          </div>
        </FieldPanel>
        <FieldPanel title="First report" aside="After you create it">
          <RaisedCard>
            <div className="grid grid-cols-[12px_auto_1fr] items-center gap-3 px-3.5 py-2.5 text-[13px]">
              <Dot color="#a1a1aa" />
              <b className="font-medium">
                Waiting for {chosen?.name ?? "the server"} to run the check
              </b>
              <span className="text-right text-xs text-faint">
                The agent picks it up on its next report, usually within 30 seconds.
              </span>
            </div>
          </RaisedCard>
        </FieldPanel>
        {notice && (
          <Callout tone="warn">
            Creating agent monitors needs a Core update that is not available yet.
          </Callout>
        )}
        <div className="flex justify-end gap-1.5">
          <Link to="/monitors">
            <Button size="sm" variant="secondary">
              Cancel
            </Button>
          </Link>
          <Button
            size="sm"
            disabled={!chosen || !value.trim()}
            onClick={() => !AGENT_MONITOR_API_AVAILABLE && setNotice(true)}
          >
            Create monitor
          </Button>
        </div>
      </div>
    </div>
  );
};
