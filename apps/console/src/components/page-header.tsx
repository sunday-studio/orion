type PageHeaderProps = {
  title: string;
  description?: string | React.ReactNode;
};

export const PageHeader = ({ title, description }: PageHeaderProps) => {
  return (
    <>
      <h1 className="text-2xl font-medium">{title}</h1>
      {description && <p className="text-sm text-neutral-600">{description}</p>}
    </>
  );
};
