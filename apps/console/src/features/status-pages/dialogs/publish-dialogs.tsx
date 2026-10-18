import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Dot,
  RaisedCard,
} from "@/design-system";
import type { ApiStatusPageReadinessResponse } from "@/orion-sdk";

export const PublishDialog = ({
  open,
  title,
  readiness,
  pending,
  error,
  onOpenChange,
  onPublish,
}: {
  open: boolean;
  title: string;
  readiness?: ApiStatusPageReadinessResponse;
  pending: boolean;
  error?: string;
  onOpenChange: (open: boolean) => void;
  onPublish: () => void;
}) => {
  const blocked = readiness?.checks?.some((check) => check.blocking && !check.ok);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[440px] gap-3 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[15px] font-medium">Publish {title}</DialogTitle>
        </DialogHeader>
        <RaisedCard>
          {(readiness?.checks ?? []).map((check) => (
            <div
              key={check.key}
              className="grid grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
            >
              <Dot color={check.ok ? "#2a9f69" : check.blocking ? "#e5484d" : "#f59e0b"} />
              <b className="font-medium">{check.label}</b>
              {!check.ok && check.detail && (
                <span className="max-w-[150px] truncate text-xs text-faint">{check.detail}</span>
              )}
            </div>
          ))}
        </RaisedCard>
        <p className="text-[13px] leading-[1.5] text-quiet">
          Anyone with the link can see this page. Subscribers are not emailed on publish.
        </p>
        {error && <Callout tone="warn">{error}</Callout>}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={pending || blocked} onClick={onPublish}>
            Publish page
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export const UnpublishDialog = ({
  open,
  title,
  pending,
  onOpenChange,
  onUnpublish,
}: {
  open: boolean;
  title: string;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onUnpublish: () => void;
}) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-w-[420px] gap-3 rounded-2xl p-5">
      <DialogHeader>
        <DialogTitle className="text-[15px] font-medium">Unpublish {title}</DialogTitle>
      </DialogHeader>
      <p className="text-[13px] leading-[1.5] text-mid">
        The public address stops working. Subscribers keep their subscriptions and are not notified.
      </p>
      <DialogFooter>
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant="destructive" disabled={pending} onClick={onUnpublish}>
          Unpublish
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);
