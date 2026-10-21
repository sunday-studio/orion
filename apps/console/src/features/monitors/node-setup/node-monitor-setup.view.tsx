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
import {
  type ApiNodeMonitorDefinitionRequestCheckType,
  useCreateNodeMonitorDefinition,
  useGetNodes,
  useListNodeMonitorDefinitions,
} from "@/orion-sdk";

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

export const NodeMonitorSetupPage = () => {
  const nodes = useGetNodes({ limit: 200 });
  const servers = (nodes.data?.nodes ?? []).filter((node) => node.status !== "down");
  const [server, setServer] = useState("");
  const [what, setWhat] = useState<string>("process");
  const [value, setValue] = useState("");
  const [every, setEvery] = useState("30");
  const [fail, setFail] = useState("3");
  const [error, setError] = useState<string>();
  const [created, setCreated] = useState<string>();
  const create = useCreateNodeMonitorDefinition();
  const chosen = servers.find((node) => node.id === server) ?? servers[0];
  const [label, placeholder] = field[what as keyof typeof field] ?? field.process;
  const definitions = useListNodeMonitorDefinitions(chosen?.id ?? "", {
    query: {
      enabled: Boolean(created && chosen?.id),
      refetchInterval: (query) =>
        query.state.data?.definitions?.some(
          (item) => item.id === created && item.state === "reporting",
        )
          ? false
          : 5000,
    },
  });
  const definition = (definitions.data?.definitions ?? []).find((item) => item.id === created);

  const submit = () => {
    if (!chosen?.id) return;
    const text = value.trim();
    const params: Record<string, unknown> =
      what === "disk"
        ? { path: text, threshold_percent: 90 }
        : what === "memory"
          ? { threshold_percent: Number(text) }
          : what === "port"
            ? { port: Number(text) }
            : { process_name: text };
    const name =
      what === "process"
        ? text
        : what === "disk"
          ? `disk ${text}`
          : what === "port"
            ? `port ${text}`
            : "memory";
    setError(undefined);
    create.mutate(
      {
        nodeId: chosen.id,
        data: {
          name,
          check_type: what as ApiNodeMonitorDefinitionRequestCheckType,
          params,
          interval_seconds: Number(every),
          fail_after: Number(fail),
        },
      },
      {
        onSuccess: (result) => setCreated(result.definition?.id),
        onError: (failure) =>
          setError(
            (failure as Error).message ||
              "Unable to create the monitor. Check the values and try again.",
          ),
      },
    );
  };

  return (
    <div className="w-full pb-8">
      <PageHeader
        title="New Node monitor"
        description="Runs on your server through the Node and reports to Core."
        actions={
          <Link to="/monitors">
            <Button size="sm" variant="secondary">
              Cancel
            </Button>
          </Link>
        }
      />
      <div className="mt-3 flex flex-col gap-3">
        <FieldPanel title="Server" aside="Only servers with a healthy Node">
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            Server
            <Select
              aria-label="Server"
              className="w-full font-normal"
              options={servers.map((node) => ({
                value: node.id ?? "",
                label: `${node.name} (${node.status_text ?? node.status ?? "unknown"}${
                  node.node_version ? `, node ${node.node_version}` : ""
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
              <Dot color={definition?.state === "reporting" ? "#2a9f69" : "#a1a1aa"} />
              <b className="font-medium">
                {definition?.state === "reporting"
                  ? `${chosen?.name} is reporting this check`
                  : `Waiting for ${chosen?.name ?? "the server"} to run the check`}
              </b>
              <span className="text-right text-xs text-faint">
                {created
                  ? "The Node picks it up when it next syncs its definitions."
                  : "Appears after you create the monitor."}
              </span>
            </div>
          </RaisedCard>
        </FieldPanel>
        {error && (
          <Callout tone="warn">
            Unable to create the monitor. Check the values and try again.
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
            disabled={!chosen || !value.trim() || create.isPending || Boolean(created)}
            onClick={submit}
          >
            Create monitor
          </Button>
        </div>
      </div>
    </div>
  );
};
