import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { getApiBase } from "@/api/client";
import {
  Button,
  Callout,
  CodeBlock,
  CodeTabs,
  FieldPanel,
  Input,
  PageHeader,
  RaisedCard,
  Select,
  StatusBadge,
  StepIndicator,
  StepRows,
  useToast,
} from "@/design-system";
import {
  getListAppsQueryKey,
  useCreateApp,
  useCreateIngestKey,
  useGetApp,
  useSearchLogs,
  useSendAppTestLog,
} from "@/orion-sdk";
import { toZoned } from "@/utils/date";
import { logSnippets } from "./snippets";

type Step = "create" | "key" | "waiting" | "received";

const STEP_LABELS = ["Create app", "Add the key", "First log"] as const;
const stepNumber: Record<Step, number> = { create: 1, key: 2, waiting: 3, received: 3 };

const environmentOptions = [
  { value: "production", label: "production" },
  { value: "staging", label: "staging" },
  { value: "development", label: "development" },
];

const clock = (iso?: string) => {
  if (!iso) return "-";
  const date = toZoned(new Date(iso));
  const pad = (value: number, size = 2) => String(value).padStart(size, "0");
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`;
};

/** Connect an app: create it, copy the key once, send a log, see it arrive (F1, four steps). */
export const ConnectAppPage = () => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [step, setStep] = useState<Step>("create");
  const [name, setName] = useState("");
  const [environment, setEnvironment] = useState("production");
  const [appId, setAppId] = useState<string>();
  const [secret, setSecret] = useState<string>();
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  // The app survives a failed key request so a retry only creates the key.
  const createdApp = useRef<string | undefined>(undefined);
  const createApp = useCreateApp();
  const createKey = useCreateIngestKey();
  const testLog = useSendAppTestLog({
    mutation: { onError: () => toast.add({ title: "Test log failed" }) },
  });
  const origin = getApiBase() || window.location.origin;

  const appQuery = useGetApp(appId ?? "", {
    query: {
      enabled: Boolean(appId) && step === "waiting",
      refetchInterval: (query) => (query.state.data?.app?.last_ingest_at ? false : 2000),
    },
  });
  const received = Boolean(appQuery.data?.app?.last_ingest_at);
  if (step === "waiting" && received) setStep("received");

  const latestLog = useSearchLogs(
    { app_id: appId, limit: 1 },
    { query: { enabled: Boolean(appId) && step === "received" } },
  );
  const firstEntry = latestLog.data?.entries?.[0];

  const submit = async () => {
    setError("");
    try {
      let id = createdApp.current;
      if (!id) {
        const response = await createApp.mutateAsync({
          data: { name: name.trim(), environment },
        });
        id = response.app?.id;
        if (!id) throw new Error("The app was not created.");
        createdApp.current = id;
      }
      const key = await createKey.mutateAsync({ id, data: { name: "first key" } });
      setAppId(id);
      setSecret(key.key?.key);
      setStep("key");
      void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
    } catch (err) {
      const reason = err instanceof Error ? err.message : "Unable to create the app.";
      setError(
        createdApp.current
          ? `The app was created but its key was not: ${reason} Try again to create the key.`
          : reason,
      );
    }
  };

  const copyKey = async () => {
    if (!secret) return;
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  const appName = name.trim() || "your app";
  const cancel = (
    <Link to="/settings/apps">
      <Button size="sm" variant="ghost">
        Cancel
      </Button>
    </Link>
  );
  const heading = {
    create: {
      title: "Connect your first app",
      description: "Create an app, copy a key, and see its logs here.",
    },
    key: {
      title: `Add the key to ${appName}`,
      description: "Copy it now. Orion only shows it once.",
    },
    waiting: { title: "Waiting for the first log", description: "This page updates by itself." },
    received: { title: `${appName} is connected`, description: "Its first log just arrived." },
  }[step];

  return (
    <div className="mx-auto w-full max-w-[712px]">
      <PageHeader title={heading.title} description={heading.description} actions={cancel} />
      <StepIndicator steps={STEP_LABELS} current={stepNumber[step]} />
      <div className="flex flex-col gap-3">
        {step === "create" && (
          <form
            className="flex flex-col items-center gap-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (name.trim()) void submit();
            }}
          >
            <FieldPanel title="App" aside="Name it after the service" className="w-full">
              <div className="flex flex-col gap-3.5">
                <label className="flex flex-col gap-1.5 text-[13px] font-medium">
                  Name
                  <Input
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder="billing-api"
                    required
                  />
                </label>
                <div className="flex flex-col gap-1.5 text-[13px] font-medium">
                  <span id="environment-label">Environment</span>
                  <Select
                    aria-label="Environment"
                    className="w-full font-normal"
                    options={environmentOptions}
                    value={environment}
                    onValueChange={setEnvironment}
                  />
                </div>
              </div>
            </FieldPanel>
            {error && (
              <Callout tone="down" className="w-full">
                {error}
              </Callout>
            )}
            <Button
              type="submit"
              disabled={!name.trim() || createApp.isPending || createKey.isPending}
            >
              Create app
            </Button>
          </form>
        )}

        {step === "key" && secret && (
          <>
            <FieldPanel title="Ingest key" aside="Shown once">
              <CodeBlock aria-label="Ingest key">{secret}</CodeBlock>
              <div className="mt-2.5">
                <Button size="sm" onClick={() => void copyKey()}>
                  {copied ? "Copied" : "Copy key"}
                </Button>
              </div>
            </FieldPanel>
            <FieldPanel title="Send a log" aside="Use what your app already speaks">
              <CodeTabs aria-label="Language" tabs={logSnippets(origin)} />
            </FieldPanel>
            <div className="flex justify-center">
              <Button onClick={() => setStep("waiting")}>I sent one</Button>
            </div>
          </>
        )}

        {step === "waiting" && (
          <>
            <RaisedCard className="px-4 py-3.5">
              <div className="mb-2 flex items-center gap-2 text-[13px] font-medium">
                <span
                  aria-hidden
                  className="size-2 rounded-full bg-[#2a9f69] ring-4 ring-[#2a9f69]/20"
                />
                Listening for {appName}
              </div>
              <StepRows
                rows={[
                  { tone: "done", title: "Key is valid" },
                  {
                    tone: "idle",
                    title: "No log received yet",
                    detail: "Usually within a minute of your app writing one",
                  },
                ]}
              />
            </RaisedCard>
            <div className="flex justify-center gap-2">
              <Button
                disabled={!appId || testLog.isPending}
                onClick={() => appId && testLog.mutate({ id: appId })}
              >
                Send a test log
              </Button>
              <Button variant="secondary" onClick={() => setStep("key")}>
                Show snippets again
              </Button>
            </div>
          </>
        )}

        {step === "received" && (
          <>
            <Callout tone="up" lead="First log received">
              just now.
            </Callout>
            <RaisedCard className="flex items-center gap-4 px-4 py-3 text-[13px]">
              <span className="font-mono text-xs text-faint">{clock(firstEntry?.timestamp)}</span>
              <StatusBadge status={firstEntry?.severity}>
                {firstEntry?.severity ?? "info"}
              </StatusBadge>
              <span className="min-w-0 truncate font-medium">
                {firstEntry?.event || firstEntry?.body || "First log"}
              </span>
            </RaisedCard>
            <div className="flex justify-center gap-2">
              <Link to="/logs">
                <Button>Open in Logs</Button>
              </Link>
              <Button
                variant="secondary"
                onClick={() => {
                  createdApp.current = undefined;
                  setStep("create");
                  setName("");
                  setAppId(undefined);
                  setSecret(undefined);
                }}
              >
                Connect another app
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};
