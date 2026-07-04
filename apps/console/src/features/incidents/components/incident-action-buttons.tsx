import { Button } from "@/components/ui/button";
import {
  CheckIcon,
  CircleCheckIcon,
  MegaphoneIcon,
  RotateCcwIcon,
  ShieldCheckIcon,
} from "lucide-react";

type IncidentActionButtonsProps = {
  actionPending: boolean;
  canAcknowledge: boolean;
  canCover: boolean;
  canReopen: boolean;
  canResolve: boolean;
  onAcknowledge: () => void;
  onCover: () => void;
  onOpenPublicDraft: () => void;
  onReopen: () => void;
  onResolve: () => void;
};

export const IncidentActionButtons = ({
  actionPending,
  canAcknowledge,
  canCover,
  canReopen,
  canResolve,
  onAcknowledge,
  onCover,
  onOpenPublicDraft,
  onReopen,
  onResolve,
}: IncidentActionButtonsProps) => (
  <div className="flex flex-wrap gap-2">
    {canAcknowledge && (
      <Button variant="outline" disabled={actionPending} onClick={onAcknowledge}>
        <CheckIcon />
        Acknowledge
      </Button>
    )}
    {canCover && (
      <Button variant="outline" disabled={actionPending} onClick={onCover}>
        <ShieldCheckIcon />
        Cover
      </Button>
    )}
    {canResolve && (
      <Button disabled={actionPending} onClick={onResolve}>
        <CircleCheckIcon />
        Resolve
      </Button>
    )}
    {canReopen && (
      <Button variant="outline" disabled={actionPending} onClick={onReopen}>
        <RotateCcwIcon />
        Reopen
      </Button>
    )}
    <Button variant="outline" disabled={actionPending} onClick={onOpenPublicDraft}>
      <MegaphoneIcon />
      Public draft
    </Button>
  </div>
);
