import { CodeBlock } from "@/design-system/components/code-block";
import { cn } from "@/utils/cn";

/** Pretty-prints JSON (a string that parses, or any value) with two-space indent. */
const formatJson = (value: unknown) => {
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  try {
    return JSON.stringify(value, null, 2) ?? "";
  } catch {
    return String(value);
  }
};

/**
 * The one place JSON is shown: parsed, indented, wrapped, scrollable. Use it for payloads,
 * responses and raw reports instead of hand-formatting a CodeBlock.
 */
export const JsonBlock = ({
  value,
  className,
  maxHeight = 320,
}: {
  value: unknown;
  className?: string;
  maxHeight?: number;
}) => (
  <CodeBlock
    className={cn("overflow-auto whitespace-pre-wrap break-words", className)}
    style={{ maxHeight }}
  >
    {formatJson(value)}
  </CodeBlock>
);
