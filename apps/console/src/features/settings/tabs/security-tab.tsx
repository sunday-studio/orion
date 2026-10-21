import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Callout,
  ConfirmDialog,
  FieldPanel,
  GridRow,
  GridTable,
  Input,
  RaisedCard,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  getGetSystemSettingsQueryKey,
  getListAPITokensQueryKey,
  useGetSystemSettings,
  useListAPITokens,
  useRevokeAPIToken,
  useRotateRegistrationToken,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { SettingRow } from "../components-ui";
import { TokenDialog } from "./token-dialog";

const COLUMNS = "minmax(0,1.2fr) 100px 110px 80px 90px";

const ago = (iso?: string) => {
  if (!iso) return "Never";
  const minutes = Math.round((Date.now() - new Date(iso).getTime()) / 60000);
  if (minutes < 60) return minutes <= 1 ? "Just now" : `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return "Today";
  const days = Math.round(hours / 24);
  return days < 7 ? `${days}d ago` : formatDate(iso, "MMM d");
};

export const SecurityTab = () => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const system = useGetSystemSettings();
  const tokens = useListAPITokens();
  const [creating, setCreating] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [revoking, setRevoking] = useState<{ id: string; name: string }>();
  const refreshTokens = () =>
    void queryClient.invalidateQueries({ queryKey: getListAPITokensQueryKey() });
  const rotate = useRotateRegistrationToken({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getGetSystemSettingsQueryKey() });
        toast.add({ title: "Registration token rotated" });
      },
      onError: () => toast.add({ title: "Unable to rotate the token" }),
    },
  });
  const revoke = useRevokeAPIToken({
    mutation: {
      onSuccess: () => {
        refreshTokens();
        toast.add({ title: "Token revoked" });
      },
      onError: () => toast.add({ title: "Unable to revoke the token" }),
    },
  });

  const registration = system.data?.registration;
  const signIn = system.data?.sign_in;
  const list = tokens.data?.tokens ?? [];
  return (
    <div className="flex flex-col gap-3.5">
      {system.isPending ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : system.isError ? (
        <Callout tone="down">Unable to load security settings.</Callout>
      ) : (
        <>
          <FieldPanel title="Console sign-in" aside="Set on the Core host">
            <RaisedCard>
              <SettingRow label="Username" detail="ORION_ADMIN_USERNAME">
                <b className="font-medium">{signIn?.username ?? "admin"}</b>
              </SettingRow>
              <SettingRow label="Session length" detail="Signed out after">
                <b className="font-medium">{signIn?.session_hours ?? 12} hours</b>
              </SettingRow>
            </RaisedCard>
          </FieldPanel>
          <FieldPanel title="Node registration" aside="Nodes register with this token">
            <div className="flex gap-2">
              <Input
                readOnly
                aria-label="Registration token"
                className="font-mono"
                value={registration?.token || registration?.token_mask || "Not configured"}
              />
              <Button
                variant="secondary"
                disabled={!registration?.token}
                onClick={() => {
                  void navigator.clipboard.writeText(registration?.token ?? "");
                  toast.add({ title: "Token copied" });
                }}
              >
                Copy
              </Button>
              <Button
                variant="secondary"
                disabled={rotate.isPending}
                onClick={() => setRotating(true)}
              >
                Rotate
              </Button>
            </div>
          </FieldPanel>
        </>
      )}
      <FieldPanel title="API tokens" aside="For scripts and integrations">
        {tokens.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : tokens.isError ? (
          <Callout tone="down">Unable to load API tokens.</Callout>
        ) : (
          list.length > 0 && (
            <RaisedCard>
              <GridTable>
                <GridRow columns={COLUMNS} header>
                  <span>Name</span>
                  <span>Created</span>
                  <span>Last used</span>
                  <span>Scope</span>
                  <span />
                </GridRow>
                {list.map((token) => (
                  <GridRow key={token.id} columns={COLUMNS}>
                    <b className="font-medium">{token.name}</b>
                    <span className="text-faint">{formatDate(token.created_at, "MMM d")}</span>
                    <span className="text-faint">{ago(token.last_used_at)}</span>
                    <span className="capitalize text-faint">{token.scope}</span>
                    <span className="text-right">
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => setRevoking({ id: token.id ?? "", name: token.name ?? "" })}
                      >
                        Revoke
                      </Button>
                    </span>
                  </GridRow>
                ))}
              </GridTable>
            </RaisedCard>
          )
        )}
        <Button size="sm" className="mt-3" onClick={() => setCreating(true)}>
          <Plus className="size-3.5" />
          New token
        </Button>
      </FieldPanel>
      <TokenDialog open={creating} onOpenChange={setCreating} onCreated={refreshTokens} />
      <ConfirmDialog
        open={rotating}
        onOpenChange={setRotating}
        title="Rotate the registration token"
        description="New Nodes need the new token. Servers that are already connected keep working."
        confirmLabel="Rotate token"
        onConfirm={() => rotate.mutate()}
      />
      <ConfirmDialog
        open={Boolean(revoking)}
        onOpenChange={(open) => !open && setRevoking(undefined)}
        title={`Revoke ${revoking?.name ?? "token"}`}
        description="Anything using this token stops working immediately."
        confirmLabel="Revoke token"
        onConfirm={() => revoking && revoke.mutate({ id: revoking.id })}
      />
    </div>
  );
};
