import { cn } from "@/utils/cn";
import type { ReactNode } from "react";

export type DetailCardVariant = "neutral" | "indigo" | "emerald" | "amber" | "rose" | "violet";

const variantClassNames: Record<
  DetailCardVariant,
  { card: string; accent: string; title: string }
> = {
  neutral: {
    card: "border-neutral-200 bg-white",
    accent: "bg-neutral-300",
    title: "text-neutral-950",
  },
  indigo: {
    card: "border-indigo-200 bg-indigo-50/70",
    accent: "bg-indigo-500",
    title: "text-indigo-950",
  },
  emerald: {
    card: "border-emerald-200 bg-emerald-50/70",
    accent: "bg-emerald-500",
    title: "text-emerald-950",
  },
  amber: {
    card: "border-amber-200 bg-amber-50/70",
    accent: "bg-amber-400",
    title: "text-amber-950",
  },
  rose: {
    card: "border-rose-200 bg-rose-50/70",
    accent: "bg-rose-500",
    title: "text-rose-950",
  },
  violet: {
    card: "border-violet-200 bg-violet-50/70",
    accent: "bg-violet-500",
    title: "text-violet-950",
  },
};

type DetailCardProps = {
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  description?: ReactNode;
  title?: ReactNode;
  variant?: DetailCardVariant;
};

export const DetailCard = ({
  action,
  children,
  className,
  contentClassName,
  description,
  title,
  variant = "neutral",
}: DetailCardProps) => {
  const variantClasses = variantClassNames[variant];
  const hasHeader = title || description || action;

  return (
    <section
      className={cn(
        "relative overflow-hidden rounded-lg border px-3 py-3 shadow-xs",
        variantClasses.card,
        className,
      )}
    >
      <div className={cn("absolute inset-x-0 top-0 h-1", variantClasses.accent)} />
      {hasHeader && (
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3 pt-1">
          <div className="min-w-0 space-y-0.5">
            {title && <h2 className={cn("text-sm font-medium", variantClasses.title)}>{title}</h2>}
            {description && <p className="text-sm text-neutral-600">{description}</p>}
          </div>
          {action}
        </div>
      )}
      <div className={cn("space-y-3", contentClassName)}>{children}</div>
    </section>
  );
};
