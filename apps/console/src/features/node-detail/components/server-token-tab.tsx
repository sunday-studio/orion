import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  CodeBlock,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  FieldPanel,
  RaisedCard,
  StatusBadge,
  Textarea,
} from "@/design-system";
import {
  type ApiNodeTokenIssuedResponse,
  getGetNodeQueryKey,
  getGetNodeTokenStatusQueryKey,
  getGetNodesQueryKey,
  useGetNodeTokenStatus,
  useReissueNodeToken,
  useRevokeNodeToken,
  useRotateNodeToken,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";

type Issued = { action: "reissue" | "rotate"; response: ApiNodeTokenIssuedResponse };

const Row = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div className="grid grid-cols-[12px_140px_1fr] items-center gap-3 border-t border-line-soft px-3.5 py-3 text-[13px] first:border-t-0">
    <span />
    <b className="font-medium">{label}</b>
    <span className="min-w-0 truncate text-right text-faint">{children}</span>
  </div>
);

export const ServerTokenTab = ({ nodeId, name }: { nodeId: string; name: string }) => {
  const queryClient = useQueryClient();
  const status = useGetNodeTokenStatus(nodeId);
  const rotate = useRotateNodeToken();
  const reissue = useReissueNodeToken();
  const revoke = useRevokeNodeToken();
  const [issued, setIssued] = useState<Issued | null>(null);
  const [stored, setStored] = useState(false);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState(false);
  const [reason, setReason] = useState("");

  const data = status.data;
  const revoked = data?.state === "revoked" || Boolean(data?.token_revoked_at);
  const hasToken = data?.token_exists ?? false;
  const busy = rotate.isPending || reissue.isPending || revoke.isPending;
  const failed = rotate.isError || reissue.isError || revoke.isError;

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetNodeTokenStatusQueryKey(nodeId) }),
      queryClient.invalidateQueries({ queryKey: getGetNodeQueryKey(nodeId) }),
      queryClient.invalidateQueries({ queryKey: getGetNodesQueryKey() }),
    ]);
  };
  const issue = async (action: Issued["action"]) => {
    try {
      const response =
        action === "rotate"
          ? await rotate.mutateAsync({ nodeId })
          : await reissue.mutateAsync({ nodeId });
      setStored(false);
      setCopied(false);
      setIssued({ action, response });
      await refresh();
    } catch {
      // The failed state of the mutation drives the error callout.
    }
  };
  const copy = async () => {
    if (!issued?.response.token) return;
    await navigator.clipboard.writeText(issued.response.token);
    setCopied(true);
  };

  return (
    <div className="flex flex-col gap-3">
      <FieldPanel title="Server token" aside="Orion stores a hash, never the secret">
        <RaisedCard>
          <Row label="Status">
            <StatusBadge status={revoked ? "down" : hasToken ? "up" : "unknown"}>
              {revoked ? "Revoked" : hasToken ? "Active" : "Missing"}
            </StatusBadge>
          </Row>
          <Row label="Rotated">{formatDate(data?.token_rotated_at, DATE_TIME_FORMAT)}</Row>
          <Row label="Last used">{formatDate(data?.last_used_at, DATE_TIME_FORMAT)}</Row>
          <Row label="Server id">
            <span className="font-mono text-xs">{data?.node_id ?? nodeId}</span>
          </Row>
          {revoked && data?.token_revocation_reason && (
            <Row label="Revoked because">{data.token_revocation_reason}</Row>
          )}
        </RaisedCard>
      </FieldPanel>
      {failed && <Callout tone="down">That token action failed. Try again.</Callout>}
      <div className="flex justify-end gap-1.5">
        {revoked || !hasToken ? (
          <Button size="sm" disabled={busy} onClick={() => void issue("reissue")}>
            Reissue
          </Button>
        ) : (
          <Button
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={() => void issue("rotate")}
          >
            Rotate
          </Button>
        )}
        <Button
          size="sm"
          variant="destructive"
          disabled={busy || revoked || !hasToken}
          onClick={() => setRevoking(true)}
        >
          Revoke
        </Button>
      </div>
      <p className="text-xs text-quiet">Rotating keeps the old token valid for 10 minutes.</p>

      <Dialog open={revoking} onOpenChange={setRevoking}>
        <DialogContent className="max-w-[440px] gap-3 rounded-2xl p-5">
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Revoke server token</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] leading-[1.45] text-mid">
            Revoking disables the active token for {name}. Its monitors will go unknown. Enter a
            reason to confirm.
          </p>
          <Textarea
            aria-label="Reason for revocation"
            placeholder="Reason for revocation"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
          <DialogFooter>
            <Button variant="secondary" onClick={() => setRevoking(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={!reason.trim() || revoke.isPending}
              onClick={async () => {
                try {
                  await revoke.mutateAsync({ nodeId, data: { reason: reason.trim() } });
                  setRevoking(false);
                  setReason("");
                  await refresh();
                } catch {
                  // The failed state of the mutation drives the error callout.
                }
              }}
            >
              Revoke token
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={issued !== null}
        onOpenChange={(open) => {
          if (!open && stored) setIssued(null);
        }}
      >
        <DialogContent className="max-w-[480px] gap-3 rounded-2xl p-5" showCloseButton={false}>
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">New token for {name}</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] text-mid">Copy it now. Orion only shows it once.</p>
          {issued?.response.token ? (
            <>
              <CodeBlock className="break-all whitespace-pre-wrap">
                {issued.response.token}
              </CodeBlock>
              <div>
                <Button size="sm" onClick={() => void copy()}>
                  {copied ? "Copied" : "Copy token"}
                </Button>
              </div>
            </>
          ) : (
            <Callout tone="down">The response did not include a token.</Callout>
          )}
          {issued?.action === "rotate" && (
            <Callout tone="warn">
              The old token keeps working for 10 minutes, so you can update the node without a gap.
            </Callout>
          )}
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox checked={stored} onCheckedChange={(next) => setStored(Boolean(next))} />
            I have stored this token
          </label>
          <DialogFooter>
            <Button disabled={!stored} onClick={() => setIssued(null)}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};
