import type * as React from "react";
import { Input as DsInput } from "@/design-system";

/** Legacy import path; renders the design-system input. */
function Input({ ...props }: React.ComponentProps<"input">) {
  return <DsInput {...(props as React.ComponentProps<typeof DsInput>)} />;
}

export { Input };
