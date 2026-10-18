import {
  Button,
  Callout,
  CodeBlock,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/design-system";
import type { ApiAlertTestResult } from "@/orion-sdk";

const caption = "font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]";

type Props = {
  result?: ApiAlertTestResult;
  email?: boolean;
  /** Offered after a passing test of an unsaved webhook. */
  onSave?: () => void;
  onRunAgain: () => void;
  onResend?: () => void;
  onClose: () => void;
};

/** Test outcome: sent, failed (expected vs actual) or email recipients (canvas channel-test). */
export const ChannelTestResultDialog = ({
  result,
  email,
  onSave,
  onRunAgain,
  onResend,
  onClose,
}: Props) => {
  const passed = result?.passed;
  const title = email ? "Test email sent" : passed ? "Test sent" : "Test failed";
  const delivered = result?.recipients?.delivered ?? 0;
  const skipped = result?.recipients?.skipped ?? 0;
  return (
    <Dialog open={Boolean(result)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-[460px] gap-3 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[15px] font-medium">{title}</DialogTitle>
        </DialogHeader>
        {email ? (
          <>
            <Callout tone={delivered > 0 ? "up" : "warn"}>
              <b className="mr-1.5 font-semibold">
                {delivered} of {delivered + skipped}
              </b>
              recipients
            </Callout>
            {result?.hint && <p className="text-[13px] leading-[1.5] text-quiet">{result.hint}</p>}
          </>
        ) : passed ? (
          <>
            <Callout tone="up">
              <b className="mr-1.5 font-semibold">
                {result?.actual?.title ?? result?.status_code ?? "OK"}
              </b>
              {typeof result?.duration_ms === "number" ? `in ${result.duration_ms} ms` : ""}
            </Callout>
            {result?.body && <CodeBlock>{result.body}</CodeBlock>}
          </>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-2">
              <div className="rounded-[10px] bg-[#142a23] px-4 py-3.5 text-white">
                <span className={`${caption} text-[#80ffaa]`}>Expected</span>
                <b className="mt-1 block text-[22px] font-medium tracking-[-0.02em]">
                  {result?.expected?.title ?? "2xx"}
                </b>
                <small className="text-xs opacity-75">{result?.expected?.detail}</small>
              </div>
              <div className="rounded-[10px] border border-line-soft bg-card px-4 py-3.5">
                <span className={`${caption} text-faint`}>Actual</span>
                <b className="mt-1 block text-[22px] font-medium tracking-[-0.02em] text-[#be123c]">
                  {result?.actual?.title ?? "Failed"}
                </b>
                <small className="text-xs opacity-75">{result?.actual?.detail}</small>
              </div>
            </div>
            {result?.hint && <p className="text-[13px] leading-[1.5] text-quiet">{result.hint}</p>}
          </>
        )}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
          {email ? (
            skipped > 0 && onResend && <Button onClick={onResend}>Resend confirmation</Button>
          ) : passed ? (
            onSave && <Button onClick={onSave}>Save channel</Button>
          ) : (
            <Button onClick={onRunAgain}>Try again</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
