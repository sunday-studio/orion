import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  EmptyState,
  Input,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiApp,
  type ApiIngestKey,
  getListIngestKeysQueryKey,
  useCreateIngestKey,
  useListIngestKeys,
  useRevokeIngestKey,
  useSendAppTestLog,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { KeyReveal } from "./key-reveal";

type AppDetailDrawerProps = {
  app?: ApiApp;
  onOpenChange: (open: boolean) => void;
};

export const AppDetailDrawer = ({ app, onOpenChange }: AppDetailDrawerProps) => {
  const appId = app?.id ?? "";
  const queryClient = useQueryClient();
  const toast = useToast();
  const keysQuery = useListIngestKeys(appId, { query: { enabled: Boolean(appId) } });
  const [keyName, setKeyName] = useState("");
  const [secret, setSecret] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<ApiIngestKey | null>(null);

  const refreshKeys = () =>
    void queryClient.invalidateQueries({ queryKey: getListIngestKeysQueryKey(appId) });
  const createKey = useCreateIngestKey({
    mutation: {
      onSuccess: (response) => {
        setSecret(response.key?.key ?? null);
        setKeyName("");
        refreshKeys();
      },
    },
  });
  const revokeKey = useRevokeIngestKey({ mutation: { onSuccess: refreshKeys } });
  const testLog = useSendAppTestLog({
    mutation: {
      onSuccess: () =>
        toast.add({ title: "Test log sent", description: "It should appear in Logs shortly." }),
      onError: () => toast.add({ title: "Test log failed", description: "Check the app and key." }),
    },
  });

  const keys = keysQuery.data?.keys ?? [];

  return (
    <Drawer
      swipeDirection="right"
      open={Boolean(app)}
      onOpenChange={(open) => {
        if (!open) setSecret(null);
        onOpenChange(open);
      }}
    >
      <DrawerContent>
        <DrawerTitle>{app?.name ?? "App"}</DrawerTitle>
        <DrawerDescription>
          {app?.environment ? `${app.environment} · ` : ""}
          retention {app?.retention_days ?? 14} days
        </DrawerDescription>

        <section aria-label="Ingest keys" className="space-y-3">
          <h3 className="text-[15px] font-medium">Ingest keys</h3>
          {secret && <KeyReveal secret={secret} onDone={() => setSecret(null)} />}
          <form
            className="flex gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (!keyName.trim() || !appId) return;
              createKey.mutate({ id: appId, data: { name: keyName.trim() } });
            }}
          >
            <Input
              aria-label="Key name"
              placeholder="Key name, for example production"
              value={keyName}
              onChange={(event) => setKeyName(event.target.value)}
            />
            <Button type="submit" disabled={!keyName.trim() || createKey.isPending}>
              Create key
            </Button>
          </form>
          {createKey.isError && <p className="text-xs text-destructive">Unable to create key.</p>}

          {keysQuery.isLoading ? (
            <Skeleton className="h-8 w-full" />
          ) : keysQuery.isError ? (
            <EmptyState
              tone="error"
              title="Unable to load keys"
              action={
                <Button size="sm" variant="secondary" onClick={() => void keysQuery.refetch()}>
                  Retry
                </Button>
              }
            />
          ) : keys.length === 0 ? (
            <p className="text-[13px] text-quiet">No keys yet. Create one to start sending logs.</p>
          ) : (
            <ul className="divide-y divide-hairline text-[13px]">
              {keys.map((key) => (
                <li className="flex items-center justify-between gap-3 py-2" key={key.id}>
                  <div className="min-w-0">
                    <div className="truncate font-medium">{key.name}</div>
                    <div className="font-mono text-xs text-faint">
                      {key.prefix}… · last used{" "}
                      {key.last_used_at ? formatDate(key.last_used_at, DATE_TIME_FORMAT) : "never"}
                    </div>
                  </div>
                  {key.revoked_at ? (
                    <StatusBadge status="disabled">revoked</StatusBadge>
                  ) : (
                    <Button size="sm" variant="secondary" onClick={() => setRevoking(key)}>
                      Revoke
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>

        <section aria-label="Test" className="space-y-2">
          <h3 className="text-[15px] font-medium">Test</h3>
          <Button
            variant="secondary"
            disabled={!appId || testLog.isPending}
            onClick={() => testLog.mutate({ id: appId })}
          >
            Send test log
          </Button>
        </section>

        <ConfirmDialog
          open={revoking !== null}
          onOpenChange={(open) => {
            if (!open) setRevoking(null);
          }}
          title={`Revoke ${revoking?.name ?? "key"}?`}
          description="Apps using this key will immediately stop being able to send logs."
          confirmLabel="Revoke key"
          onConfirm={() => {
            if (revoking?.id) revokeKey.mutate({ id: appId, keyId: revoking.id });
          }}
        />
      </DrawerContent>
    </Drawer>
  );
};
