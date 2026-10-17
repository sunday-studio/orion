import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  ChoiceTiles,
  CodeBlock,
  FieldPanel,
  PageHeader,
  RaisedCard,
} from "@/design-system";
import { type ApiAgentResponse, useGetAgents } from "@/orion-sdk";
import {
  NOT_CONNECTED_AFTER_MS,
  type ServerSystem,
  coreHost,
  installCommand,
  systemTiles,
  useEnrollmentToken,
} from "./enrollment";
import { AddServerConnected } from "./add-server-connected";
import { AddServerFailed } from "./add-server-failed";
import { AddServerProgress } from "./add-server-progress";

const Narrow = ({ children }: { children: React.ReactNode }) => (
  <div className="mx-auto w-full max-w-[712px]">{children}</div>
);

const CancelButton = () => (
  <Link to="/servers">
    <Button size="sm" variant="ghost">
      Cancel
    </Button>
  </Link>
);

/** Add server: pick a system, run one command, watch the server appear (F1, four states). */
export const AddServerPage = () => {
  const [system, setSystem] = useState<ServerSystem>("linux");
  const [copied, setCopied] = useState(false);
  const [startedAt, setStartedAt] = useState(() => Date.now());
  const [now, setNow] = useState(() => Date.now());
  const [baseline, setBaseline] = useState<Set<string> | null>(null);
  const { enrollment, renew } = useEnrollmentToken();

  const agentsQuery = useGetAgents(
    { limit: 200 },
    { query: { refetchInterval: 3000, refetchOnMount: "always" } },
  );
  const agents = useMemo(() => agentsQuery.data?.agents ?? [], [agentsQuery.data]);

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  // Only fresh data counts as the baseline: a cached list could predate recent registrations.
  if (baseline === null && agentsQuery.isSuccess && agentsQuery.isFetchedAfterMount) {
    setBaseline(new Set(agents.map((agent) => agent.id ?? "")));
  }
  const registered: ApiAgentResponse | undefined = baseline
    ? agents.find((agent) => agent.id && !baseline.has(agent.id))
    : undefined;

  const elapsed = now - startedAt;
  const expired = now >= enrollment.expiresAt;
  const command = installCommand(system, enrollment.token);
  const host = coreHost();

  const startOver = () => {
    renew();
    setStartedAt(Date.now());
    setBaseline(null);
  };

  const copyCommand = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  if (registered?.last_seen) {
    return (
      <Narrow>
        <AddServerConnected agent={registered} />
      </Narrow>
    );
  }
  if (registered) {
    return (
      <Narrow>
        <PageHeader
          title="Waiting for the new server"
          description="Run the command on the host. This page updates by itself."
          actions={<CancelButton />}
        />
        <div className="mt-3 flex flex-col gap-3">
          <AddServerProgress agent={registered} />
          <Callout tone="warn" className="text-[12.5px]">
            Nothing here after 5 minutes? Run <b>orion-agent doctor</b> on the host. It prints the
            exact fix for each problem.
          </Callout>
        </div>
      </Narrow>
    );
  }
  if (expired) {
    return (
      <Narrow>
        <AddServerFailed host={host} onStartOver={startOver} onCopy={copyCommand} />
      </Narrow>
    );
  }

  return (
    <Narrow>
      <PageHeader
        title="Add a server"
        description="Pick the system, run one command, and the server appears here."
        actions={<CancelButton />}
      />
      <div className="mt-3 flex flex-col gap-3">
        <ChoiceTiles
          aria-label="Operating system"
          tiles={systemTiles}
          value={system}
          onValueChange={(value) => setSystem(value as ServerSystem)}
        />
        <FieldPanel
          title="Run this on the server"
          aside={
            enrollment.placeholder
              ? "Replace the placeholder with your enrollment token"
              : "One-time token, expires in 15 minutes"
          }
        >
          <CodeBlock aria-label="Install command">{command}</CodeBlock>
          <div className="mt-2.5">
            <Button size="sm" onClick={() => void copyCommand()}>
              {copied ? "Copied" : "Copy command"}
            </Button>
          </div>
        </FieldPanel>
        <FieldPanel title="Waiting for the agent" aside="Usually under 30 seconds">
          <RaisedCard className="flex items-center gap-3 px-4 py-3">
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-faint" />
            <p className="text-[13px] text-mid">
              This page updates by itself when the server registers.
            </p>
          </RaisedCard>
        </FieldPanel>
        {elapsed >= NOT_CONNECTED_AFTER_MS && (
          <Callout tone="down" lead="Not connected after 5 minutes?">
            Check that the server can reach {host}, then run the command again.
          </Callout>
        )}
      </div>
    </Narrow>
  );
};
