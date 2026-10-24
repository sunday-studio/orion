import { Callout } from "@/design-system";

/** Non-blocking notice from Core when the public address only works inside a private network. */
export const DomainTargetWarning = ({ message }: { message?: string }) =>
  message ? (
    <Callout tone="warn" lead="Only reachable inside a private network.">
      {message}
    </Callout>
  ) : null;

/** Hint under the DNS record table about proxied records. */
export const DomainProxyHint = () => (
  <p className="text-[12.5px] text-faint">
    Using Cloudflare? Set the record to DNS only (grey cloud) while verifying; a proxied record
    hides the target.
  </p>
);
