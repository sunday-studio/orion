import type * as React from "react";
import { cn } from "@/utils/cn";

/** Dark monospace block for commands and keys (canvas `pre.code` in deep green). */
export const CodeBlock = ({ className, children, ...props }: React.ComponentProps<"pre">) => (
  <pre
    className={cn(
      "m-0 overflow-x-auto whitespace-pre rounded-lg bg-[#142a23] px-3 py-[11px] font-mono text-[12.5px] leading-[1.65] text-[#e6f1ec]",
      className,
    )}
    {...props}
  >
    {children}
  </pre>
);
