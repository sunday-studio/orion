import { cn } from "@/lib/utils";

type PageHeaderProps = {
  title: string;
  description?: string | React.ReactNode;
  className?: string;
};

export const PageHeader = ({ title, description, className }: PageHeaderProps) => {
  return (
    <div className={cn("space-y-1", className)}>
      <h1 className="text-2xl font-medium">{title}</h1>
      {typeof description === "string" ? (
        <p className="text-sm text-neutral-600">{description}</p>
      ) : (
        description && <div className="text-sm text-neutral-600">{description}</div>
      )}
    </div>
  );
};
