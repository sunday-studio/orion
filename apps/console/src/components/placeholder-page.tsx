type PlaceholderPageProps = {
  title: string;
  description: string;
  operations: string[];
};

export const PlaceholderPage = ({ title, description, operations }: PlaceholderPageProps) => {
  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-base font-medium">{title}</h1>
        <p className="text-sm text-neutral-600">{description}</p>
      </div>
      <div className="space-y-2 text-sm">
        <p className="font-medium">Needed operations</p>
        <ul className="space-y-1 text-neutral-600">
          {operations.map((operation) => (
            <li key={operation}>{operation}</li>
          ))}
        </ul>
      </div>
    </div>
  );
};
