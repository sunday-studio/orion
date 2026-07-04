import { cn } from "@/utils/cn";
import type { ReactNode } from "react";
import { DetailCard, type DetailCardVariant } from "./detail-card";

type TimestampedReport = {
  collected_at?: string;
  created_at?: string;
};

export const DetailItem = ({
  className,
  label,
  value,
}: {
  className?: string;
  label: string;
  value: ReactNode;
}) => (
  <div>
    <div className="text-xs font-medium text-neutral-500">{label}</div>
    <div className={cn("mt-1 break-words text-sm font-medium text-neutral-950", className)}>
      {value}
    </div>
  </div>
);

export const DetailGroup = ({
  children,
  contentClassName,
  description,
  title,
  variant = "neutral",
}: {
  children: ReactNode;
  contentClassName?: string;
  description?: ReactNode;
  title: string;
  variant?: DetailCardVariant;
}) => (
  <DetailCard
    title={title}
    description={description}
    variant={variant}
    contentClassName={contentClassName}
  >
    {children}
  </DetailCard>
);

export const reportTimestamp = (report?: TimestampedReport) =>
  report?.created_at ?? report?.collected_at;
