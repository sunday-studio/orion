import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Callout,
  ChoiceTiles,
  CodeBlock,
  EmptyState,
  FieldPanel,
  PageHeader,
  RaisedCard,
} from "@/design-system";
import {
  type ApiEnrollmentCreatedResponse,
  useCancelEnrollment,
  useCreateEnrollment,
  useGetEnrollment,
} from "@/orion-sdk";
import { AddServerConnected } from "./add-server-connected";
import { AddServerFailed } from "./add-server-failed";
import { AddServerProgress } from "./add-server-progress";
import { NOT_CONNECTED_AFTER_MS, type ServerPlatform, coreHost, platformTiles } from "./enrollment";

const Narrow = ({ children }: { children: React.ReactNode }) => (
  <div className="w-full">{children}</div>
);

const CancelButton = () => (
  <Link to="/servers">
    <Button size="sm" variant="ghost">
      Cancel
    </Button>
  </Link>
);

/** Add server: pick a platform, run one command, watch the server enroll (F1, four states). */
export const AddServerPage = () => {
  const [platform, setPlatform] = useState<ServerPlatform>("linux");
  const [enrollment, setEnrollment] = useState<ApiEnrollmentCreatedResponse>();
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  const createEnrollment = useCreateEnrollment();
  const cancelEnrollment = useCancelEnrollment();
  const activeId = useRef<string | undefined>(undefined);
  const generation = useRef(0);
  const statusRef = useRef<string>("waiting");

  const mint = (next: ServerPlatform) => {
    const ticket = (generation.current += 1);
    const previous = activeId.current;
    if (previous) cancelEnrollment.mutate({ id: previous });
    activeId.current = undefined;
    setEnrollment(undefined);
    createEnrollment.mutate(
      { data: { platform: next } },
      {
        onSuccess: (created) => {
          if (ticket !== generation.current) {
            if (created.id) cancelEnrollment.mutate({ id: created.id });
            return;
          }
          activeId.current = created.id;
          setEnrollment(created);
        },
      },
    );
  };

  // One enrollment per visit and platform; leaving the page cancels one that was never used.
  useEffect(() => {
    mint(platform);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [platform]);
  useEffect(
    () => () => {
      generation.current += 1;
      // Cancel only a token nobody used: a claimed or registered one is mid-registration.
      if (activeId.current && statusRef.current === "waiting") {
        cancelEnrollment.mutate({ id: activeId.current });
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const id = enrollment?.id ?? "";
  const statusQuery = useGetEnrollment(id, {
    query: {
      enabled: Boolean(id),
      refetchInterval: (query) => {
        const status = query.state.data?.status;
        return status === "reporting" || status === "expired" || status === "cancelled"
          ? false
          : 2000;
      },
    },
  });
  const live = statusQuery.data;
  const status = live?.status ?? enrollment?.status ?? "waiting";
  statusRef.current = status;
  const host = coreHost(enrollment?.core_url);
  const command = enrollment?.install_command ?? "";
  const createdMs = enrollment?.created_at ? new Date(enrollment.created_at).getTime() : now;

  const copyCommand = async () => {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  if (status === "reporting" && live) {
    return (
      <Narrow>
        <AddServerConnected enrollment={live} />
      </Narrow>
    );
  }
  if ((status === "claimed" || status === "registered") && live) {
    return (
      <Narrow>
        <PageHeader
          title="Waiting for the new server"
          description="Run the command on the host. This page updates by itself."
          actions={<CancelButton />}
        />
        <div className="mt-3 flex flex-col gap-3">
          <AddServerProgress enrollment={live} />
          <Callout tone="warn" className="text-[12.5px]">
            Nothing here after 5 minutes? Run <b>orion-agent doctor</b> on the host. It prints the
            exact fix for each problem.
          </Callout>
        </div>
      </Narrow>
    );
  }
  if (status === "expired" || status === "cancelled") {
    return (
      <Narrow>
        <AddServerFailed host={host} onStartOver={() => mint(platform)} onCopy={copyCommand} />
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
          tiles={platformTiles}
          value={platform}
          onValueChange={(value) => setPlatform(value as ServerPlatform)}
        />
        {createEnrollment.isError ? (
          <EmptyState
            tone="error"
            title="Unable to create an enrollment"
            action={
              <Button size="sm" variant="secondary" onClick={() => mint(platform)}>
                Retry
              </Button>
            }
          />
        ) : (
          <FieldPanel title="Run this on the server" aside="One-time token, expires in 15 minutes">
            <CodeBlock aria-label="Install command" className="whitespace-pre-wrap break-all">
              {command || "Preparing the command..."}
            </CodeBlock>
            <div className="mt-2.5">
              <Button size="sm" disabled={!command} onClick={() => void copyCommand()}>
                {copied ? "Copied" : "Copy command"}
              </Button>
            </div>
          </FieldPanel>
        )}
        <FieldPanel title="Waiting for the agent" aside="Usually under 30 seconds">
          <RaisedCard className="flex items-center gap-3 px-4 py-3">
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-faint" />
            <p className="text-[13px] text-mid">
              This page updates by itself when the server registers.
            </p>
          </RaisedCard>
        </FieldPanel>
        {now - createdMs >= NOT_CONNECTED_AFTER_MS && (
          <Callout tone="down" lead="Not connected after 5 minutes?">
            Check that the server can reach {host}, then run the command again.
          </Callout>
        )}
      </div>
    </Narrow>
  );
};
