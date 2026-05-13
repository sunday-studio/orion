export const IncidentSummary = ({ count }: { count: number }) => {
  const label = count === 1 ? "incident needs attention" : "incidents need attention";

  return (
    <div className="py-2">
      <p className="font-medium">
        {count} {label}
      </p>
      <p>Open monitor issues that may need a closer look.</p>
    </div>
  );
};
