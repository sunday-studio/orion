export const IncidentSummary = ({ count }: { count: number }) => {
  const label = count === 1 ? "incident found" : "incidents found";

  return (
    <div className="py-2">
      <p className="font-medium">
        {count} {label}
      </p>
      <p className="text-sm text-neutral-600">Operational issues recorded by Core.</p>
    </div>
  );
};
