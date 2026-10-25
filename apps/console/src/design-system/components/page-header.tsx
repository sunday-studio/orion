import type * as React from "react";
import { cn } from "@/utils/cn";

type PageHeaderProps = {
  title: string;
  description?: string | React.ReactNode;
  /** Right-aligned actions, for example a primary button. */
  actions?: React.ReactNode;
  className?: string;
};

/** Page title block: 28px title, muted subtitle, actions on the right (canvas `.phd`). */
export const PageHeader = ({ title, description, actions, className }: PageHeaderProps) => {
  return (
    <div className={cn("flex items-center gap-3 pb-2 pt-7", className)}>
      <div className="min-w-0 flex-1">
        <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{title}</h1>
        {typeof description === "string" ? (
          <p className="mt-0.5 text-[13.5px] leading-[1.4] text-quiet">{description}</p>
        ) : (
          description && (
            <div className="mt-0.5 text-[13.5px] leading-[1.4] text-quiet">{description}</div>
          )
        )}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-1.5">{actions}</div>}
    </div>
  );
};
