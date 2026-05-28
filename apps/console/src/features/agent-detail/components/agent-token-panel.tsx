import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiAgentTokenIssuedResponse,
  type ApiAgentTokenStatusResponse,
  getGetAgentQueryKey,
  getGetAgentTokenStatusQueryKey,
  getGetAgentsQueryKey,
  useGetAgentTokenStatus,
  useReissueAgentToken,
  useRevokeAgentToken,
  useRotateAgentToken,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Check, Clipboard, KeyRound, RotateCcw, ShieldOff } from "lucide-react";
import { useState } from "react";

type AgentTokenPanelProps = {
  agentId: string;
};

type IssuedTokenState = {
  action: "reissue" | "rotate";
  response: ApiAgentTokenIssuedResponse;
};

const isRevoked = (status?: ApiAgentTokenStatusResponse) =>
  status?.state === "revoked" || Boolean(status?.token_revoked_at);

const statusLabel = (status?: ApiAgentTokenStatusResponse) => {
  if (!status) return "unknown";
  if (status.state) return status.state;
  if (!status.token_exists) return "missing";
  return "active";
};

const formatBoolean = (value?: boolean) => {
  if (value === undefined) return "Unknown";
  return value ? "Yes" : "No";
};

const mutationMessage = (error: unknown) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  return "Action failed.";
};

const CopyButton = ({ value }: { value: string }) => {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  };

  return (
    <Button size="sm" variant="outline" onClick={() => void copy()}>
      {copied ? <Check /> : <Clipboard />}
      {copied ? "Copied" : "Copy token"}
    </Button>
  );
};

const MetadataItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="break-words text-sm font-medium">{value}</div>
  </div>
);

export const AgentTokenPanel = ({ agentId }: AgentTokenPanelProps) => {
  const queryClient = useQueryClient();
  const tokenStatus = useGetAgentTokenStatus(agentId);
  const rotateToken = useRotateAgentToken();
  const reissueToken = useReissueAgentToken();
  const revokeToken = useRevokeAgentToken();
  const [issuedToken, setIssuedToken] = useState<IssuedTokenState | null>(null);
  const [isRevokeOpen, setIsRevokeOpen] = useState(false);
  const [revokeReason, setRevokeReason] = useState("");

  const status = tokenStatus.data;
  const revoked = isRevoked(status);
  const hasToken = status?.token_exists ?? false;
  const actionError =
    mutationMessage(rotateToken.error) ||
    mutationMessage(reissueToken.error) ||
    mutationMessage(revokeToken.error);
  const isMutating = rotateToken.isPending || reissueToken.isPending || revokeToken.isPending;
  const trimmedReason = revokeReason.trim();

  const refreshStatus = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: getGetAgentTokenStatusQueryKey(agentId) }),
      queryClient.invalidateQueries({ queryKey: getGetAgentQueryKey(agentId) }),
      queryClient.invalidateQueries({ queryKey: getGetAgentsQueryKey() }),
    ]);
  };

  const clearIssuedToken = () => {
    setIssuedToken(null);
    rotateToken.reset();
    reissueToken.reset();
  };

  const handleRotate = async () => {
    const response = await rotateToken.mutateAsync({ agentId });
    setIssuedToken({ action: "rotate", response });
    await refreshStatus();
  };

  const handleReissue = async () => {
    const response = await reissueToken.mutateAsync({ agentId });
    setIssuedToken({ action: "reissue", response });
    await refreshStatus();
  };

  const handleRevoke = async () => {
    await revokeToken.mutateAsync({ agentId, data: { reason: trimmedReason } });
    setIsRevokeOpen(false);
    setRevokeReason("");
    await refreshStatus();
  };

  return (
    <section className="space-y-4 border border-neutral-200 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 text-sm font-medium">
            <KeyRound className="size-4" />
            Agent Token
          </h2>
          <p className="text-sm text-neutral-600">
            Non-secret lifecycle metadata and replacement-token actions.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {revoked || !hasToken ? (
            <Button size="sm" onClick={() => void handleReissue()} disabled={isMutating}>
              <RotateCcw />
              {reissueToken.isPending ? "Reissuing..." : "Reissue"}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleRotate()}
              disabled={isMutating}
            >
              <RotateCcw />
              {rotateToken.isPending ? "Rotating..." : "Rotate"}
            </Button>
          )}
          <Button
            size="sm"
            variant="destructive"
            onClick={() => setIsRevokeOpen(true)}
            disabled={isMutating || revoked || !hasToken}
          >
            <ShieldOff />
            Revoke
          </Button>
        </div>
      </div>

      {tokenStatus.isLoading ? (
        <div className="text-sm text-neutral-600">Loading token metadata...</div>
      ) : tokenStatus.error ? (
        <div className="text-sm text-red-700">Unable to load token metadata.</div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <MetadataItem label="status" value={statusLabel(status)} />
          <MetadataItem label="token exists" value={formatBoolean(status?.token_exists)} />
          <MetadataItem label="version" value={status?.token_version ?? "Unknown"} />
          <MetadataItem
            label="rotated"
            value={formatDate(status?.token_rotated_at, DATE_TIME_FORMAT)}
          />
          <MetadataItem
            label="revoked"
            value={formatDate(status?.token_revoked_at, DATE_TIME_FORMAT)}
          />
          <MetadataItem
            label="revocation reason"
            value={status?.token_revocation_reason || "None"}
          />
          <MetadataItem label="agent id" value={status?.agent_id || agentId} />
          <MetadataItem label="request id" value={status?.request_id || "None"} />
        </div>
      )}

      {actionError && <div className="text-sm text-red-700">{actionError}</div>}

      <Dialog open={Boolean(issuedToken)} onOpenChange={(open) => !open && clearIssuedToken()}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {issuedToken?.action === "reissue" ? "Replacement token issued" : "Token rotated"}
            </DialogTitle>
            <DialogDescription>
              This token is shown once. Copy it now and update the agent configuration before
              closing this dialog.
            </DialogDescription>
          </DialogHeader>
          {issuedToken?.response.token ? (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="text-sm font-medium">One-time token</div>
                <CopyButton value={issuedToken.response.token} />
              </div>
              <pre className="max-h-56 overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">
                {issuedToken.response.token}
              </pre>
            </div>
          ) : (
            <div className="text-sm text-red-700">The response did not include a token.</div>
          )}
          <DialogFooter>
            <Button variant="ghost" onClick={clearIssuedToken}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isRevokeOpen} onOpenChange={setIsRevokeOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Revoke agent token</DialogTitle>
            <DialogDescription>
              Revoking disables the active token for this agent. Enter a reason to confirm.
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={revokeReason}
            onChange={(event) => setRevokeReason(event.target.value)}
            placeholder="Reason for revocation"
            rows={4}
          />
          {revokeToken.error && (
            <div className="text-sm text-red-700">{mutationMessage(revokeToken.error)}</div>
          )}
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setIsRevokeOpen(false)}
              disabled={revokeToken.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleRevoke()}
              disabled={!trimmedReason || revokeToken.isPending}
            >
              {revokeToken.isPending ? "Revoking..." : "Revoke token"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
};
