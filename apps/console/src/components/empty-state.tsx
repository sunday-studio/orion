import { cn } from "@/lib/utils";

type EmptyStateProps = {
  title: string;
  description?: string;
  className?: string;
};

export const EmptyState = ({ title, description, className }: EmptyStateProps) => {
  return (
    <div
      className={cn(
        "flex min-h-64 flex-col items-center justify-center px-4 py-10 text-center",
        className,
      )}
    >
      <div className="mb-4 h-10 w-10 bg-[radial-gradient(currentColor_1px,transparent_1px)] bg-[size:5px_5px] text-purple-400 [clip-path:polygon(50%_0,100%_50%,50%_100%,0_50%)]" />
      <div className="space-y-1">
        <p className="font-medium text-neutral-900 text-sm">{title}</p>
        {description && <p className="max-w-sm text-neutral-500 text-sm">{description}</p>}
      </div>
    </div>
  );
};
