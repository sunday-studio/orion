import { Callout } from "@/design-system";
import { type CoreWorkerDiagnostics, describeCoreWorkerWarning } from "./core-worker-model";

export {
  type CoreWorkerDiagnostics,
  coreWorkerDiagnosticsFromPayload,
  shouldWarnForCoreWorker,
} from "./core-worker-model";

type CoreWorkerWarningProps = {
  worker?: CoreWorkerDiagnostics;
  className?: string;
};

export const CoreWorkerWarning = ({ worker, className }: CoreWorkerWarningProps) => {
  const warning = describeCoreWorkerWarning(worker);
  if (!warning) return null;

  return (
    <Callout tone="warn" lead={warning.lead} className={className}>
      {warning.text}
    </Callout>
  );
};
