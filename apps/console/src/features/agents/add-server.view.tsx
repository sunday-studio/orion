import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getApiBase } from "@/api/client";
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  PageHeader,
} from "@/design-system";
import { useGetAgents } from "@/orion-sdk";

const installCommand = (coreURL: string) =>
  `curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \\
  --core-url ${coreURL || "https://core.your-domain.tld"}`;

/** Show the install command, then watch the Servers list for a Server that was not there before. */
export const AddServerPage = () => {
  const [coreURL, setCoreURL] = useState(() => getApiBase() || window.location.origin);
  const [copied, setCopied] = useState(false);
  const [baseline, setBaseline] = useState<Set<string> | null>(null);
  const agentsQuery = useGetAgents({ limit: 200 }, { query: { refetchInterval: 3000 } });
  const agents = useMemo(() => agentsQuery.data?.agents ?? [], [agentsQuery.data]);

  // The first successful load is the baseline; anything that appears afterwards is the new Server.
  if (baseline === null && agentsQuery.isSuccess) {
    setBaseline(new Set(agents.map((agent) => agent.id ?? "")));
  }
  const registered = baseline
    ? agents.find((agent) => agent.id && !baseline.has(agent.id))
    : undefined;
  const command = installCommand(coreURL);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="mx-auto max-w-[760px] space-y-6">
      <PageHeader
        title="Add server"
        description="Install the Orion agent on a Linux or macOS host and it registers itself."
      />
      <Card>
        <CardHeader>
          <CardTitle>1. Run the installer on the host</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className="grid gap-1 text-[13px]">
            Core URL the server can reach
            <Input value={coreURL} onChange={(event) => setCoreURL(event.target.value)} />
          </label>
          <pre
            aria-label="Install command"
            className="overflow-auto rounded-lg bg-ink p-3 font-mono text-xs text-page"
          >
            {command}
          </pre>
          <Button size="sm" variant="secondary" onClick={() => void copy()}>
            {copied ? "Copied" : "Copy command"}
          </Button>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>2. Wait for it to register</CardTitle>
        </CardHeader>
        <CardContent>
          {registered ? (
            <p role="status" data-testid="server-registered" className="text-[13px] text-brand">
              {registered.name ?? "A new server"} registered.{" "}
              <Link className="underline" to={`/servers/${registered.id}`}>
                Open server
              </Link>
            </p>
          ) : (
            <p role="status" className="text-[13px] text-quiet">
              Waiting for a new server to appear...
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
