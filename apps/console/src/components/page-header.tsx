type PageHeaderProps = {
  title: string;
  description?: string;
};

export const PageHeader = ({ title, description }: PageHeaderProps) => {
  return (
    <div>
      <h1 className="text-2xl font-medium">{title}</h1>
      {description && <p className="text-sm text-neutral-600">{description}</p>}
    </div>
  );
};
