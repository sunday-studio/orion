import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { getApiBase } from "@/api/client";
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
} from "@/design-system";
import { getListAppsQueryKey, useCreateApp, useCreateIngestKey, useGetApp } from "@/orion-sdk";
import { KeyReveal } from "./key-reveal";

type ConnectAppDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

type Step = "create" | "snippet";

const snippetFor = (secret: string) => {
  const base = getApiBase() || window.location.origin;
  return `curl -X POST ${base}/v1/ingest/logs \\
  -H "Authorization: Bearer ${secret}" \\
  -H "Content-Type: application/json" \\
  -d '{"severity":"info","event":"hello.orion","body":"My first log"}'`;
};

/** Create an app and key, show how to send a log, then wait for the first one to arrive. */
export const ConnectAppDialog = ({ open, onOpenChange }: ConnectAppDialogProps) => {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [step, setStep] = useState<Step>("create");
  const [appId, setAppId] = useState<string>();
  const [secret, setSecret] = useState<string>();
  const [error, setError] = useState("");
  const [stored, setStored] = useState(false);
  // Bumped on close so an in-flight create cannot update a dialog that was closed and reopened.
  const session = useRef(0);
  // The app survives a failed key request so a retry only creates the key.
  const createdAppId = useRef<string | undefined>(undefined);
  const createApp = useCreateApp();
  const createKey = useCreateIngestKey();

  const appQuery = useGetApp(appId ?? "", {
    query: {
      enabled: Boolean(appId) && step === "snippet",
      refetchInterval: (query) => (query.state.data?.app?.last_ingest_at ? false : 2000),
    },
  });
  const received = Boolean(appQuery.data?.app?.last_ingest_at);

  const submit = async () => {
    setError("");
    const token = session.current;
    try {
      let id = createdAppId.current;
      if (!id) {
        const appResponse = await createApp.mutateAsync({ data: { name: name.trim() } });
        id = appResponse.app?.id;
        if (!id) throw new Error("App was not created.");
        createdAppId.current = id;
      }
      const keyResponse = await createKey.mutateAsync({ id, data: { name: "first key" } });
      if (token !== session.current) return;
      setAppId(id);
      setSecret(keyResponse.key?.key);
      setStep("snippet");
      void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
    } catch (err) {
      if (token !== session.current) return;
      const reason = err instanceof Error ? err.message : "Unable to create the app.";
      setError(
        createdAppId.current
          ? `The app was created but its key was not: ${reason} Try again to create the key.`
          : reason,
      );
    }
  };

  const canClose = step === "create" || stored;

  const close = (next: boolean) => {
    if (!next) {
      // The key is shown once, so keep the dialog open until the user confirms they saved it.
      if (!canClose) return;
      session.current += 1;
      createdAppId.current = undefined;
      setStep("create");
      setName("");
      setSecret(undefined);
      setAppId(undefined);
      setError("");
      setStored(false);
    }
    onOpenChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Connect your first app</DialogTitle>
          <DialogDescription>
            {step === "create"
              ? "Name the app that will send logs. We create an ingest key for it."
              : "Send a log with this command, or use any OpenTelemetry SDK."}
          </DialogDescription>
        </DialogHeader>

        {step === "create" ? (
          <form
            className="grid gap-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (name.trim()) void submit();
            }}
          >
            <label className="grid gap-1 text-[13px]">
              App name
              <Input value={name} onChange={(event) => setName(event.target.value)} required />
            </label>
            {error && (
              <p role="alert" className="text-xs text-destructive">
                {error}
              </p>
            )}
            <DialogFooter>
              <Button
                type="submit"
                disabled={!name.trim() || createApp.isPending || createKey.isPending}
              >
                Create app
              </Button>
            </DialogFooter>
          </form>
        ) : (
          <div className="space-y-4">
            {secret && <KeyReveal secret={secret} onDone={() => undefined} hideDone />}
            <label className="flex items-center gap-2 text-[13px]">
              <Checkbox
                aria-label="I stored the key"
                checked={stored}
                onCheckedChange={(next) => setStored(Boolean(next))}
              />
              I stored the key somewhere safe
            </label>
            {secret && (
              <pre
                aria-label="Example request"
                className="overflow-auto rounded-lg bg-field p-3 font-mono text-xs"
              >
                {snippetFor(secret)}
              </pre>
            )}
            <div role="status" className="text-[13px]">
              {received ? (
                <span data-testid="first-log-received" className="text-brand">
                  First log received. Your app is connected.
                </span>
              ) : (
                <span className="text-quiet">Waiting for the first log...</span>
              )}
            </div>
            <DialogFooter>
              {received && (
                <Link className="self-center text-[13px] text-brand hover:underline" to="/logs">
                  View logs
                </Link>
              )}
              <Button
                variant={received ? "primary" : "secondary"}
                disabled={!canClose}
                onClick={() => close(false)}
              >
                {received ? "Done" : "Close"}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};
