import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Callout,
  CodeBlock,
  ConfirmDialog,
  FieldPanel,
  GridRow,
  GridTable,
  RaisedCard,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import { getApiBase } from "@/api/client";
import {
  getListIngestKeysQueryKey,
  useGetGeneralSettings,
  useListIngestKeys,
  useRevokeIngestKey,
  useRotateIngestKey,
  useSendAppTestLog,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { agoShort } from "../app-format";
import { KeyRevealDialog, NewKeyDialog } from "../dialogs/key-dialogs";

const COLUMNS = "minmax(0,1.2fr) 110px 90px 90px 150px";

const snippets = (origin: string, app: string) => [
  {
    label: "curl",
    code: `curl -X POST ${origin}/v1/ingest/logs \\
  -H "Authorization: Bearer $ORION_KEY" \\
  -d '{"event":"payment.failed","severity":"error","body":"upstream 502"}'`,
  },
  {
    label: "OpenTelemetry environment variables",
    code: `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=${origin}/v1/ingest/otlp/v1/logs
OTEL_EXPORTER_OTLP_LOGS_HEADERS=Authorization=Bearer $ORION_KEY
OTEL_SERVICE_NAME=${app}`,
  },
  {
    label: "TypeScript",
    code: `await fetch("${origin}/v1/ingest/logs", {
  method: "POST",
  headers: { Authorization: \`Bearer \${process.env.ORION_KEY}\` },
  body: JSON.stringify({ event: "payment.failed", severity: "error", body: "upstream 502" }),
});`,
  },
  {
    label: "Go",
    code: `req, _ := http.NewRequest("POST", "${origin}/v1/ingest/logs",
    strings.NewReader(\`{"event":"payment.failed","severity":"error"}\`))
req.Header.Set("Authorization", "Bearer "+os.Getenv("ORION_KEY"))
http.DefaultClient.Do(req)`,
  },
  {
    label: "Python",
    code: `import os, requests
requests.post("${origin}/v1/ingest/logs",
    headers={"Authorization": f"Bearer {os.environ['ORION_KEY']}"},
    json={"event": "payment.failed", "severity": "error"})`,
  },
];

export const IngestTab = ({ appId, appName }: { appId: string; appName: string }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const general = useGetGeneralSettings();
  const keys = useListIngestKeys(appId);
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<{ id: string; name: string }>();
  const [rotating, setRotating] = useState<{ id: string; name: string }>();
  const [rotated, setRotated] = useState<{ secret: string; until?: string }>();
  const [showSnippets, setShowSnippets] = useState(false);
  const [sentAt, setSentAt] = useState<number>();
  const origin = general.data?.public_url || getApiBase() || window.location.origin;
  const refresh = () =>
    void queryClient.invalidateQueries({ queryKey: getListIngestKeysQueryKey(appId) });
  const revoke = useRevokeIngestKey({
    mutation: {
      onSuccess: () => {
        refresh();
        toast.add({ title: "Key revoked" });
      },
      onError: () => toast.add({ title: "Unable to revoke the key" }),
    },
  });
  const rotate = useRotateIngestKey({
    mutation: {
      onSuccess: (result) => {
        refresh();
        setRotated({
          secret: result.key?.key ?? "",
          until: result.previous_key_valid_until,
        });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to rotate" }),
    },
  });
  const test = useSendAppTestLog({
    mutation: {
      onSuccess: () => setSentAt(Date.now()),
      onError: (failure) => toast.add({ title: (failure as Error).message || "Test log failed" }),
    },
  });
  const copy = (text: string, label: string) => {
    void navigator.clipboard.writeText(text);
    toast.add({ title: `${label} copied` });
  };
  const list = keys.data?.keys ?? [];
  const endpoints = [
    { label: "Native JSON", value: `POST ${origin}/v1/ingest/logs` },
    { label: "OpenTelemetry", value: `POST ${origin}/v1/ingest/otlp/v1/logs` },
  ];

  return (
    <div className="flex flex-col gap-3.5">
      <FieldPanel title="Ingest keys" aside="Write-only. A key cannot read logs or touch monitors">
        {keys.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : keys.isError ? (
          <Callout tone="down">Unable to load ingest keys.</Callout>
        ) : (
          list.length > 0 && (
            <RaisedCard>
              <GridTable>
                <GridRow columns={COLUMNS} header>
                  <span>Key</span>
                  <span>Prefix</span>
                  <span>Created</span>
                  <span>Last used</span>
                  <span />
                </GridRow>
                {list.map((key) => {
                  const revokedAt = key.revoked_at ? new Date(key.revoked_at) : undefined;
                  const graceActive = revokedAt && revokedAt.getTime() > Date.now();
                  const dead = revokedAt && !graceActive;
                  return (
                    <GridRow
                      key={key.id}
                      columns={COLUMNS}
                      className={dead ? "opacity-55" : undefined}
                    >
                      <b className="flex items-center gap-2 font-medium">
                        {key.name}
                        {graceActive && (
                          <StatusBadge status="degraded">
                            Valid until {formatDate(key.revoked_at, "HH:mm")}
                          </StatusBadge>
                        )}
                        {dead && <StatusBadge status="unknown">Revoked</StatusBadge>}
                      </b>
                      <span className="text-faint">{key.prefix}</span>
                      <span className="text-faint">{formatDate(key.created_at, "MMM d")}</span>
                      <span className="text-faint">
                        {key.last_used_at ? agoShort(key.last_used_at) : "Never"}
                      </span>
                      <span className="flex justify-end gap-1.5">
                        {!revokedAt && (
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => setRotating({ id: key.id ?? "", name: key.name ?? "" })}
                          >
                            Rotate
                          </Button>
                        )}
                        {(!revokedAt || graceActive) && (
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => setRevoking({ id: key.id ?? "", name: key.name ?? "" })}
                          >
                            Revoke
                          </Button>
                        )}
                      </span>
                    </GridRow>
                  );
                })}
              </GridTable>
            </RaisedCard>
          )
        )}
        <Button size="sm" className="mt-3" onClick={() => setCreating(true)}>
          <Plus className="size-3.5" />
          New key
        </Button>
      </FieldPanel>
      <FieldPanel title="Endpoints" aside="Use the one your app already speaks">
        <RaisedCard>
          {endpoints.map((endpoint) => (
            <div
              key={endpoint.label}
              className="grid min-h-[42px] grid-cols-[170px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0"
            >
              <b className="font-medium">{endpoint.label}</b>
              <span className="truncate font-mono text-xs text-quiet">{endpoint.value}</span>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => copy(endpoint.value, "Endpoint")}
              >
                Copy
              </Button>
            </div>
          ))}
        </RaisedCard>
      </FieldPanel>
      <FieldPanel
        title="Send a test log"
        aside={showSnippets ? "Pick what your app already uses" : "Check the app is wired up"}
      >
        {showSnippets && (
          <div className="mb-3 grid gap-2.5">
            {snippets(origin, appName).map((snippet) => (
              <div key={snippet.label} className="grid gap-1.5">
                <div className="flex items-center justify-between text-[13px]">
                  <span>{snippet.label}</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => copy(snippet.code, snippet.label)}
                  >
                    Copy
                  </Button>
                </div>
                <CodeBlock>{snippet.code}</CodeBlock>
              </div>
            ))}
          </div>
        )}
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" onClick={() => setShowSnippets((v) => !v)}>
            {showSnippets ? "Hide snippets" : "Open snippets"}
          </Button>
          <Button size="sm" disabled={test.isPending} onClick={() => test.mutate({ id: appId })}>
            Send a test log now
          </Button>
        </div>
        {sentAt && (
          <p className="mt-3 flex items-center gap-2 text-[13px]">
            <i className="size-2 rounded-full bg-[#2a9f69]" />
            <b className="font-medium">Received just now</b>
            <span className="text-quiet">test.log · "Hello from Orion"</span>
          </p>
        )}
      </FieldPanel>
      <NewKeyDialog appId={appId} open={creating} onOpenChange={setCreating} onCreated={refresh} />
      <ConfirmDialog
        open={Boolean(rotating)}
        onOpenChange={(open) => !open && setRotating(undefined)}
        title={`Rotate ${rotating?.name ?? "key"}`}
        description="You get a new key now. The old key keeps working for 10 minutes so you can switch over."
        confirmLabel="Rotate key"
        tone="default"
        onConfirm={() => rotating && rotate.mutate({ id: appId, keyId: rotating.id })}
      />
      <ConfirmDialog
        open={Boolean(revoking)}
        onOpenChange={(open) => !open && setRevoking(undefined)}
        title={`Revoke ${revoking?.name ?? "key"}`}
        description="Apps using this key stop being able to send logs immediately."
        confirmLabel="Revoke key"
        onConfirm={() => revoking && revoke.mutate({ id: appId, keyId: revoking.id })}
      />
      {rotated && (
        <KeyRevealDialog
          title="New ingest key"
          secret={rotated.secret}
          note={
            rotated.until
              ? `The old key keeps working until ${formatDate(rotated.until, "HH:mm")}.`
              : undefined
          }
          onDone={() => setRotated(undefined)}
        />
      )}
    </div>
  );
};
