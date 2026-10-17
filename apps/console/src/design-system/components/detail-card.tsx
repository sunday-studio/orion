import { cn } from "@/utils/cn";
import type { ReactNode } from "react";

export type DetailCardVariant = "neutral" | "indigo" | "emerald" | "amber" | "rose" | "violet";

const variantClassNames: Record<
  DetailCardVariant,
  { card: string; accent: string; title: string }
> = {
  neutral: {
    card: "border-hairline bg-card",
    accent: "bg-hairline",
    title: "text-ink",
  },
  indigo: {
    card: "border-indigo-200 bg-card",
    accent: "bg-indigo-500",
    title: "text-indigo-950",
  },
  emerald: {
    card: "border-emerald-200 bg-card",
    accent: "bg-emerald-500",
    title: "text-emerald-950",
  },
  amber: {
    card: "border-amber-200 bg-card",
    accent: "bg-amber-400",
    title: "text-amber-950",
  },
  rose: {
    card: "border-rose-200 bg-card",
    accent: "bg-rose-500",
    title: "text-rose-950",
  },
  violet: {
    card: "border-violet-200 bg-card",
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
        "relative overflow-hidden rounded-lg border px-4 py-4 shadow-xs",
        variantClasses.card,
        className,
      )}
    >
      <div className={cn("absolute inset-x-0 top-0 h-0.5 bg-card", variantClasses.accent)} />
      {hasHeader && (
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3 pt-1">
          <div className="min-w-0 space-y-0.5">
            {title && <h2 className={cn("text-sm font-medium", variantClasses.title)}>{title}</h2>}
            {description && <p className="text-sm text-quiet">{description}</p>}
          </div>
          {action}
        </div>
      )}
      <div className={cn("space-y-3", contentClassName)}>{children}</div>
    </section>
  );
};
