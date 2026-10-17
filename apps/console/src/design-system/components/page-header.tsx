import { cn } from "@/utils/cn";

type PageHeaderProps = {
  title: string;
  description?: string | React.ReactNode;
  className?: string;
};

export const PageHeader = ({ title, description, className }: PageHeaderProps) => {
  return (
    <div className={cn("space-y-1", className)}>
      <h1 className="text-[30px] leading-9 font-medium">{title}</h1>
      {typeof description === "string" ? (
        <p className="text-sm text-quiet">{description}</p>
      ) : (
        description && <div className="text-sm text-quiet">{description}</div>
      )}
    </div>
  );
};
