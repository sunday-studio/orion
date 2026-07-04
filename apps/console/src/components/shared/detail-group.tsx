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
    <div className="text-sm text-neutral-600">{label}</div>
    <div className={cn("break-words text-sm font-medium", className)}>{value}</div>
  </div>
);

export const DetailGroup = ({
  children,
  title,
  variant = "neutral",
}: {
  children: ReactNode;
  title: string;
  variant?: DetailCardVariant;
}) => (
  <DetailCard title={title} variant={variant}>
    {children}
  </DetailCard>
);

export const reportTimestamp = (report?: TimestampedReport) =>
  report?.created_at ?? report?.collected_at;
