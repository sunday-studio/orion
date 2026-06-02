import { cn } from "@/utils/cn";
import type { ReactNode } from "react";

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

export const DetailGroup = ({ title, children }: { title: string; children: ReactNode }) => (
  <div className="space-y-3 bg-neutral-50 px-3 py-3">
    <h2 className="text-sm font-medium">{title}</h2>
    <div className="space-y-3">{children}</div>
  </div>
);

export const reportTimestamp = (report?: TimestampedReport) =>
  report?.created_at ?? report?.collected_at;
