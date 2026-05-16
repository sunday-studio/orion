export const DetailItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600 capitalize">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);
